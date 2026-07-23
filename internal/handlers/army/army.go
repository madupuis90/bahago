package army

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/components"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/game"
	"bahago/internal/hub"
	"bahago/internal/router"
	"bahago/internal/routes"
	. "bahago/internal/ui"
)

// ── Input structs ─────────────────────────────────────────────────────────────

type transferInput struct {
	FromID   int    `json:"xfer_from"`
	ToID     int    `json:"xfer_to"`
	UnitType string `json:"xfer_unit"`
	Count    int    `json:"xfer_count"`
}

type sendInput struct {
	LegionID      int    `json:"send_legion"`
	Action        string `json:"send_action"`
	TargetName    string `json:"send_target"`
	DurationTicks int    `json:"send_ticks"`
}

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrUnknownUnitType           = errors.New("unknown unit type")
	ErrInvalidCount              = errors.New("count must be at least 1")
	ErrCountTooLarge             = errors.New("count is too large")
	ErrInvalidAction             = errors.New("invalid action")
	ErrInvalidDuration           = errors.New("invalid duration")
	ErrTargetRequired            = errors.New("target kingdom name is required")
	ErrTargetNotFound            = errors.New("target kingdom not found")
	ErrSelfTarget                = errors.New("cannot target your own kingdom")
	ErrInvalidCampaignID         = errors.New("invalid campaign id")
	ErrCampaignNotFound          = errors.New("campaign not found or already returning")
	ErrLegionNotFound            = errors.New("legion not found")
	ErrLegionInUse               = errors.New("legion is currently deployed")
	ErrLegionEmpty               = errors.New("legion has no units")
	ErrLegionCapReached          = errors.New("maximum number of legions reached")
	ErrInsufficientUnitsInSource = errors.New("not enough units in source")
	ErrSameSourceAndDestination  = errors.New("source and destination must be different")
	ErrInvalidLegionID           = errors.New("invalid legion id")
	ErrTransferConflict          = errors.New("transfer conflict, please try again")
)

// ── Route registration ───────────────────────────────────────────────────────────────

func RegisterRoutes(r router.Router, queries db.Querier, pool *pgxpool.Pool, tickHub *hub.Hub) {
	h := &handler{queries: queries, pool: pool, hub: tickHub}
	r.HandleFunc("GET "+routes.KingdomArmyPath, h.handleArmyPage())
	r.HandleFunc("GET "+routes.KingdomArmyRefreshPath, h.handleArmyRefresh())
	r.HandleFunc("POST "+routes.KingdomArmySendPath, h.handleSend())
	r.HandleFunc("POST "+routes.KingdomArmyCancelPath, h.handleCancel())
	r.HandleFunc("POST "+routes.KingdomArmyTransferPath, h.handleTransfer())
	r.HandleFunc("POST "+routes.KingdomArmyDisbandPath, h.handleDisband())
}

type handler struct {
	queries db.Querier
	pool    *pgxpool.Pool
	hub     *hub.Hub
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func cancelURL(id int) string {
	return strings.ReplaceAll(routes.KingdomArmyCancelPath, "{id}", strconv.Itoa(id))
}

func disbandURL(id int) string {
	return strings.ReplaceAll(routes.KingdomArmyDisbandPath, "{id}", strconv.Itoa(id))
}

func (h *handler) handleArmyPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		data, err := h.loadArmyData(r, kingdom.ID)
		if err != nil {
			log.Printf("army page: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		targetName := r.URL.Query().Get("target")
		action := r.URL.Query().Get("action")
		if action != "attack" && action != "defend" {
			action = "attack"
		}
		KingdomLayout(r, "Campaign", routes.KingdomArmyPath, kingdom, armyContent(kingdom, data, targetName, action)).Render(w)
	}
}

func (h *handler) handleArmyRefresh() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		ch, cleanup := h.hub.Subscribe(kingdom.ID)
		defer cleanup()

		sse := datastar.NewSSE(w, r)
		for {
			select {
			case <-r.Context().Done():
				return
			case k := <-ch:
				data, err := h.loadArmyData(r, k.ID)
				if err != nil {
					log.Printf("army refresh: %v", err)
					return
				}
				if err := sse.PatchElementGostar(MainContent(armyContent(&k, data, "", "attack"))); err != nil {
					log.Printf("army refresh: patch: %v", err)
					return
				}
			}
		}
	}
}

func (h *handler) handleSend() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		input := &sendInput{}
		if err := datastar.ReadSignals(r, input); err != nil {
			log.Printf("army send: read signals: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(errors.New("invalid request"))))
			return
		}

		if errs := validateSendInput(input); len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(errs...)))
			return
		}

		if err := h.sendCampaign(r.Context(), kingdom, input); err != nil {
			respondArmyError(w, r, err, isSendUserError, "army send")
			return
		}

		h.renderArmyPage(w, r, kingdom, "army send")
	}
}

func (h *handler) handleCancel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		id, err := validateCancelID(r.PathValue("id"))
		if err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(err)))
			return
		}

		if err := h.cancelCampaign(r.Context(), kingdom.ID, id); err != nil {
			respondArmyError(w, r, err, isCancelUserError, "army cancel")
			return
		}

		if err := datastar.NewSSE(w, r).Redirect(routes.KingdomArmyPath); err != nil {
			log.Printf("army cancel: redirect: %v", err)
		}
	}
}

func (h *handler) handleTransfer() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		input := &transferInput{}
		if err := datastar.ReadSignals(r, input); err != nil {
			log.Printf("army transfer: read signals: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(errors.New("invalid request"))))
			return
		}

		if errs := validateTransferInput(input); len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(errs...)))
			return
		}

		if err := h.transferUnits(r.Context(), kingdom, input); err != nil {
			respondArmyError(w, r, err, isTransferUserError, "army transfer")
			return
		}

		h.renderArmyPage(w, r, kingdom, "army transfer")
	}
}

func (h *handler) handleDisband() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		id, err := validateLegionPathID(r.PathValue("id"))
		if err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(err)))
			return
		}

		if err := h.disbandLegion(r.Context(), kingdom, id); err != nil {
			respondArmyError(w, r, err, isDisbandUserError, "army disband")
			return
		}

		h.renderArmyPage(w, r, kingdom, "army disband")
	}
}

// respondArmyError patches a user-facing alert for expected user errors, or
// logs and patches a generic alert for unexpected ones. logPrefix identifies
// the operation in logs.
func respondArmyError(w http.ResponseWriter, r *http.Request, err error, isUserErr func(error) bool, logPrefix string) {
	if isUserErr(err) {
		datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(err)))
		return
	}
	log.Printf("%s: %v", logPrefix, err)
	datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(errors.New("internal error"))))
}

// renderArmyPage reloads all army data and patches the main content area.
// On reload failure it patches a generic alert instead. logPrefix identifies
// the calling operation in logs.
func (h *handler) renderArmyPage(w http.ResponseWriter, r *http.Request, kingdom *db.Kingdom, logPrefix string) {
	data, err := h.loadArmyData(r, kingdom.ID)
	if err != nil {
		log.Printf("%s: reload: %v", logPrefix, err)
		datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(errors.New("internal error"))))
		return
	}
	if err := datastar.NewSSE(w, r).PatchElementGostar(MainContent(armyContent(kingdom, data, "", "attack"))); err != nil {
		log.Printf("%s: patch: %v", logPrefix, err)
	}
}

// ── Validation ────────────────────────────────────────────────────────────────

func validateTransferInput(input *transferInput) []error {
	var errs []error
	if input.Count <= 0 {
		errs = append(errs, ErrInvalidCount)
	} else if input.Count > game.MaxUnitInput {
		errs = append(errs, ErrCountTooLarge)
	}
	if input.FromID < 0 {
		errs = append(errs, ErrInvalidLegionID)
	}
	if input.FromID == input.ToID {
		errs = append(errs, ErrSameSourceAndDestination)
	}
	if _, ok := game.UnitDefs[input.UnitType]; !ok {
		errs = append(errs, ErrUnknownUnitType)
	}
	return errs
}

func validateSendInput(input *sendInput) []error {
	var errs []error
	if input.LegionID <= 0 {
		errs = append(errs, ErrInvalidLegionID)
	}

	actionValid := input.Action == "attack" || input.Action == "defend"
	if !actionValid {
		errs = append(errs, ErrInvalidAction)
	}

	maxDuration := 5
	if input.Action == "defend" {
		maxDuration = 24
	}
	if actionValid && (input.DurationTicks < 1 || input.DurationTicks > maxDuration) {
		errs = append(errs, ErrInvalidDuration)
	}

	if input.TargetName == "" {
		errs = append(errs, ErrTargetRequired)
	}
	return errs
}

func validateCancelID(idStr string) (int, error) {
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		return 0, ErrInvalidCampaignID
	}
	return id, nil
}

func validateLegionPathID(idStr string) (int, error) {
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		return 0, ErrInvalidLegionID
	}
	return id, nil
}

// ── Orchestration ─────────────────────────────────────────────────────────────

func (h *handler) sendCampaign(ctx context.Context, kingdom *db.Kingdom, input *sendInput) error {
	target, err := h.queries.GetKingdomByName(ctx, input.TargetName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: %q", ErrTargetNotFound, input.TargetName)
		}
		return fmt.Errorf("get kingdom by name: %w", err)
	}
	if target.ID == kingdom.ID {
		return ErrSelfTarget
	}

	travelTicks := game.TravelTicks(kingdom.X, kingdom.Y, target.X, target.Y)

	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	q := db.New(tx)

	if _, err := q.GetLegionForUpdate(ctx, db.GetLegionForUpdateParams{ID: input.LegionID, KingdomID: kingdom.ID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrLegionNotFound
		}
		return fmt.Errorf("get legion for update: %w", err)
	}

	units, err := q.ListLegionUnits(ctx, input.LegionID)
	if err != nil {
		return fmt.Errorf("list legion units: %w", err)
	}
	if len(units) == 0 {
		return ErrLegionEmpty
	}

	campaignParams := db.CreateCampaignParams{
		KingdomID:       kingdom.ID,
		TargetKingdomID: target.ID,
		LegionID:        input.LegionID,
		Action:          input.Action,
		TicksRemaining:  travelTicks,
		ActionTicks:     input.DurationTicks,
		TravelTicks:     travelTicks,
	}
	campaign, err := q.CreateCampaign(ctx, campaignParams)
	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
			return ErrLegionInUse
		}
		return fmt.Errorf("create campaign: %w", err)
	}

	snapshotParams := db.SnapshotLegionUnitsIntoCampaignParams{CampaignID: campaign.ID, LegionID: input.LegionID}
	if err := q.SnapshotLegionUnitsIntoCampaign(ctx, snapshotParams); err != nil {
		return fmt.Errorf("snapshot legion units: %w", err)
	}

	if err := q.ClearLegionUnits(ctx, input.LegionID); err != nil {
		return fmt.Errorf("clear legion units: %w", err)
	}

	return tx.Commit(ctx)
}

func (h *handler) cancelCampaign(ctx context.Context, kingdomID, id int) error {
	if _, err := h.queries.CancelCampaign(ctx, db.CancelCampaignParams{ID: id, KingdomID: kingdomID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrCampaignNotFound
		}
		return fmt.Errorf("cancel campaign: %w", err)
	}
	return nil
}

func (h *handler) transferUnits(ctx context.Context, kingdom *db.Kingdom, input *transferInput) error {
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	q := db.New(tx)
	toID := input.ToID

	// Verify destination legion ownership and at-home status.
	if toID > 0 {
		if _, err := q.GetLegionForUpdate(ctx, db.GetLegionForUpdateParams{ID: toID, KingdomID: kingdom.ID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrLegionNotFound
			}
			return fmt.Errorf("get to-legion for update: %w", err)
		}
		deployed, err := q.IsLegionDeployed(ctx, toID)
		if err != nil {
			return fmt.Errorf("check legion deployed: %w", err)
		}
		if deployed {
			return ErrLegionInUse
		}
	}

	// Create a new legion when destination is the sentinel -1.
	// CreateLegion returns ErrNoRows when all slots are taken (generate_series finds no gap).
	if toID == -1 {
		newLegion, err := q.CreateLegion(ctx, db.CreateLegionParams{KingdomID: kingdom.ID, Cap: game.MaxLegionsPerKingdom})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrLegionCapReached
			}
			return fmt.Errorf("create legion: %w", err)
		}
		toID = newLegion.ID
	}

	if input.FromID == 0 {
		// From Reserve: check available count then upsert into destination legion.
		// Serializable isolation prevents a concurrent transfer from racing past this check.
		available, err := q.GetAvailableKingdomUnits(ctx, kingdom.ID)
		if err != nil {
			return fmt.Errorf("get available units: %w", err)
		}
		var reserveCount int
		for _, u := range available {
			if u.UnitType == input.UnitType {
				reserveCount = u.Count
				break
			}
		}
		if reserveCount < input.Count {
			return ErrInsufficientUnitsInSource
		}
		upsertParams := db.UpsertLegionUnitParams{LegionID: toID, UnitType: input.UnitType, Count: input.Count}
		if err := q.UpsertLegionUnit(ctx, upsertParams); err != nil {
			return fmt.Errorf("upsert legion unit: %w", err)
		}
	} else {
		// From Legion: lock the row, check count, decrement, and upsert to destination.
		if _, err := q.GetLegionForUpdate(ctx, db.GetLegionForUpdateParams{ID: input.FromID, KingdomID: kingdom.ID}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrLegionNotFound
			}
			return fmt.Errorf("get from-legion for update: %w", err)
		}
		units, err := q.ListLegionUnits(ctx, input.FromID)
		if err != nil {
			return fmt.Errorf("list legion units: %w", err)
		}
		var fromCount int
		for _, u := range units {
			if u.UnitType == input.UnitType {
				fromCount = u.Count
				break
			}
		}
		if fromCount < input.Count {
			return ErrInsufficientUnitsInSource
		}
		decrementParams := db.DecrementLegionUnitParams{Amount: input.Count, LegionID: input.FromID, UnitType: input.UnitType}
		if err := q.DecrementLegionUnit(ctx, decrementParams); err != nil {
			return fmt.Errorf("decrement legion unit: %w", err)
		}
		if toID != 0 {
			upsertParams := db.UpsertLegionUnitParams{LegionID: toID, UnitType: input.UnitType, Count: input.Count}
			if err := q.UpsertLegionUnit(ctx, upsertParams); err != nil {
				return fmt.Errorf("upsert legion unit: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.SerializationFailure {
			return ErrTransferConflict
		}
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (h *handler) disbandLegion(ctx context.Context, kingdom *db.Kingdom, legionID int) error {
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	q := db.New(tx)

	if _, err := q.GetLegionForUpdate(ctx, db.GetLegionForUpdateParams{ID: legionID, KingdomID: kingdom.ID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrLegionNotFound
		}
		return fmt.Errorf("get legion for update: %w", err)
	}

	if err := q.DeleteLegion(ctx, db.DeleteLegionParams{ID: legionID, KingdomID: kingdom.ID}); err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.ForeignKeyViolation {
			return ErrLegionInUse
		}
		return fmt.Errorf("delete legion: %w", err)
	}

	return tx.Commit(ctx)
}

func isSendUserError(err error) bool {
	return errors.Is(err, ErrTargetNotFound) ||
		errors.Is(err, ErrSelfTarget) ||
		errors.Is(err, ErrLegionNotFound) ||
		errors.Is(err, ErrLegionInUse) ||
		errors.Is(err, ErrLegionEmpty)
}

func isTransferUserError(err error) bool {
	return errors.Is(err, ErrLegionNotFound) ||
		errors.Is(err, ErrLegionInUse) ||
		errors.Is(err, ErrLegionCapReached) ||
		errors.Is(err, ErrInsufficientUnitsInSource) ||
		errors.Is(err, ErrTransferConflict)
}

func isDisbandUserError(err error) bool {
	return errors.Is(err, ErrLegionNotFound) || errors.Is(err, ErrLegionInUse)
}

func isCancelUserError(err error) bool {
	return errors.Is(err, ErrCampaignNotFound)
}

// ── Data loading ──────────────────────────────────────────────────────────────

type armyData struct {
	legions       []db.ListLegionsForKingdomRow
	legionUnits   map[int][]db.KingdomLegionUnit
	campaigns     []db.GetCampaignsForKingdomRow
	campaignUnits map[int][]db.KingdomCampaignUnit
	reserve       []db.GetAvailableKingdomUnitsRow
	others        []db.ListOtherKingdomsRow
}

func (h *handler) loadArmyData(r *http.Request, kingdomID int) (armyData, error) {
	ctx := r.Context()

	legions, err := h.queries.ListLegionsForKingdom(ctx, kingdomID)
	if err != nil {
		return armyData{}, fmt.Errorf("list legions: %w", err)
	}

	allLegionUnits, err := h.queries.ListAllLegionUnitsForKingdom(ctx, kingdomID)
	if err != nil {
		return armyData{}, fmt.Errorf("list legion units: %w", err)
	}
	legionUnits := make(map[int][]db.KingdomLegionUnit)
	for _, u := range allLegionUnits {
		legionUnits[u.LegionID] = append(legionUnits[u.LegionID], u)
	}

	campaigns, err := h.queries.GetCampaignsForKingdom(ctx, kingdomID)
	if err != nil {
		return armyData{}, fmt.Errorf("get campaigns: %w", err)
	}

	allCampaignUnits, err := h.queries.ListCampaignUnitsForKingdom(ctx, kingdomID)
	if err != nil {
		return armyData{}, fmt.Errorf("list campaign units: %w", err)
	}
	campaignUnits := make(map[int][]db.KingdomCampaignUnit)
	for _, u := range allCampaignUnits {
		campaignUnits[u.CampaignID] = append(campaignUnits[u.CampaignID], u)
	}

	reserve, err := h.queries.GetAvailableKingdomUnits(ctx, kingdomID)
	if err != nil {
		return armyData{}, fmt.Errorf("get available units: %w", err)
	}

	others, err := h.queries.ListOtherKingdoms(ctx, kingdomID)
	if err != nil {
		return armyData{}, fmt.Errorf("list kingdoms: %w", err)
	}

	return armyData{
		legions:       legions,
		legionUnits:   legionUnits,
		campaigns:     campaigns,
		campaignUnits: campaignUnits,
		reserve:       reserve,
		others:        others,
	}, nil
}

// ── Components ────────────────────────────────────────────────────────────────

// firstKingdomUnit returns the first unit type (in canonical order) that exists
// anywhere in the kingdom — reserve or any legion.
func firstKingdomUnit(data armyData) string {
	existing := make(map[string]bool)
	for _, u := range data.reserve {
		if u.Count > 0 {
			existing[u.UnitType] = true
		}
	}
	for _, units := range data.legionUnits {
		for _, u := range units {
			if u.Count > 0 {
				existing[u.UnitType] = true
			}
		}
	}
	for _, utype := range game.AllUnitOrder() {
		if existing[utype] {
			return utype
		}
	}
	return ""
}

func romanNumeral(n int) string {
	switch n {
	case 1:
		return "I"
	case 2:
		return "II"
	case 3:
		return "III"
	}
	return strconv.Itoa(n)
}

func statusPill(campaign *db.GetCampaignsForKingdomRow) Node {
	var tone, label string
	if campaign == nil {
		tone, label = "home", "At home"
	} else {
		switch campaign.Status {
		case "en_route":
			tone, label = "march", "Marching out"
		case "returning":
			tone, label = "return", "Returning home"
		default:
			if campaign.Action == "attack" {
				tone, label = "combat", "Attacking"
			} else {
				tone, label = "guard", "Defending"
			}
		}
	}
	return Span(Classes{"status-pill": true, "status-pill--" + tone: true},
		Span(Class("status-dot")),
		Text(label),
	)
}

// actionTag renders the attack/defend order chip. sm renders the compact
// variant used inside the afield order readout.
func actionTag(action string, sm bool) Node {
	var tone, label string
	if action == "attack" {
		tone, label = "attack", "Attack"
	} else {
		tone, label = "defend", "Defend"
	}
	return Span(Classes{"action-tag": true, "action-tag--" + tone: true, "action-tag--sm": sm},
		Text(label),
	)
}

func unitToken(utype string, count, size int) Node {
	unit := game.UnitDefs[utype]
	sz := strconv.Itoa(size)
	initial := ""
	if len(unit.Name) > 0 {
		initial = string(unit.Name[0])
	}
	return Span(Class("unit-token"),
		Span(Classes{"portrait": true, "is-summon": unit.Category == game.CategorySummon},
			Style(fmt.Sprintf("width:%spx;height:%spx", sz, sz)),
			Title(unit.Name),
			Span(Class("portrait__initial"), Text(initial)),
		),
		Span(Class("unit-tally"), Text(strconv.Itoa(count))),
	)
}

func rosterStrip(units []db.KingdomLegionUnit, max, size int) Node {
	allOrdered := game.AllUnitOrder()
	byType := make(map[string]int, len(units))
	for _, u := range units {
		byType[u.UnitType] = u.Count
	}
	var items []Node
	extra := 0
	for _, utype := range allOrdered {
		count := byType[utype]
		if count <= 0 {
			continue
		}
		if len(items) < max {
			items = append(items, Div(Class("roster-item"),
				unitToken(utype, count, size),
				Span(Class("roster-name"), Text(game.UnitDefs[utype].Name)),
			))
		} else {
			extra++
		}
	}
	isEmpty := len(items) == 0 && extra == 0
	return Div(Class("roster-strip"),
		Group(items),
		If(extra > 0, Span(Class("roster-more"), Text(fmt.Sprintf("+%d", extra)))),
		If(isEmpty, Span(Class("roster-empty"), Text("Empty — transfer companies from the Reserve above."))),
	)
}

func rosterStripCampaign(units []db.KingdomCampaignUnit, max, size int) Node {
	allOrdered := game.AllUnitOrder()
	byType := make(map[string]int, len(units))
	for _, u := range units {
		byType[u.UnitType] = u.Count
	}
	var items []Node
	extra := 0
	for _, utype := range allOrdered {
		count := byType[utype]
		if count <= 0 {
			continue
		}
		if len(items) < max {
			items = append(items, Div(Class("roster-item"),
				unitToken(utype, count, size),
				Span(Class("roster-name"), Text(game.UnitDefs[utype].Name)),
			))
		} else {
			extra++
		}
	}
	isEmpty := len(items) == 0 && extra == 0
	return Div(Class("roster-strip"),
		Group(items),
		If(extra > 0, Span(Class("roster-more"), Text(fmt.Sprintf("+%d", extra)))),
		If(isEmpty, Span(Class("roster-empty"), Text("No companies committed."))),
	)
}

func renderStrengthLine(power, total int) Node {
	return Div(Class("strength-line"),
		Span(Class("sl-item"), B(Text(strconv.Itoa(total))), Text(" units")),
		Span(Class("sl-sep"), Text("·")),
		Span(Class("sl-item"), B(Text(strconv.Itoa(power))), Text(" power")),
	)
}

func strengthLine(units []db.KingdomLegionUnit) Node {
	var power, total int
	for _, u := range units {
		def := game.UnitDefs[u.UnitType]
		power += def.Power * u.Count
		total += u.Count
	}
	return renderStrengthLine(power, total)
}

func strengthLineCampaign(units []db.KingdomCampaignUnit) Node {
	var power, total int
	for _, u := range units {
		def := game.UnitDefs[u.UnitType]
		power += def.Power * u.Count
		total += u.Count
	}
	return renderStrengthLine(power, total)
}

func strengthLineReserve(units []db.GetAvailableKingdomUnitsRow) Node {
	var power, total int
	for _, u := range units {
		def := game.UnitDefs[u.UnitType]
		power += def.Power * u.Count
		total += u.Count
	}
	return renderStrengthLine(power, total)
}

func campaignTimeline(c db.GetCampaignsForKingdomRow) Node {
	type phase struct{ key, label string }
	midLabel := "Attacking"
	if c.Action == "defend" {
		midLabel = "Defending"
	}
	phases := []phase{
		{"en_route", "March out"},
		{"active", midLabel},
		{"returning", "March home"},
	}
	at := 0
	for i, p := range phases {
		if p.key == c.Status {
			at = i
			break
		}
	}
	var dotCells, labelCells []Node
	for i, p := range phases {
		state := "todo"
		if i < at {
			state = "done"
		} else if i == at {
			state = "now"
		}
		dotCells = append(dotCells,
			Div(Classes{"tl-dot-cell": true, "tl-dot-cell--" + state: true},
				If(i > 0, Span(Class("tl-line"))),
				Span(Class("tl-dot")),
			),
		)
		labelCells = append(labelCells,
			Span(Classes{"tl-label": true, "tl-label--" + state: true}, Text(p.label)),
		)
	}
	return Div(Class("timeline"),
		Div(Class("tl-dots"), Group(dotCells)),
		Div(Class("tl-labels"), Group(labelCells)),
	)
}

func legionCrest(n int, afield bool) Node {
	return Div(Classes{"legion-crest": true, "is-afield": afield},
		Span(Class("crest-num"), Text(romanNumeral(n))),
	)
}

func quartermasterCard(data armyData) Node {
	existing := make(map[string]bool)
	for _, u := range data.reserve {
		if u.Count > 0 {
			existing[u.UnitType] = true
		}
	}
	for _, units := range data.legionUnits {
		for _, u := range units {
			if u.Count > 0 {
				existing[u.UnitType] = true
			}
		}
	}
	allOrdered := game.AllUnitOrder()

	var atHomeLegions []db.ListLegionsForKingdomRow
	for _, l := range data.legions {
		if !l.CampaignStatus.Valid {
			atHomeLegions = append(atHomeLegions, l)
		}
	}
	canCreateLegion := len(data.legions) < game.MaxLegionsPerKingdom

	var reserveItems []Node
	for _, u := range data.reserve {
		if u.Count > 0 {
			reserveItems = append(reserveItems, Div(Class("roster-item"),
				unitToken(u.UnitType, u.Count, 56),
				Span(Class("roster-name"), Text(game.UnitDefs[u.UnitType].Name)),
			))
		}
	}

	return Div(Class("card"),
		Div(Class("card-inner"),
			SectionHeader("The Reserve", ""),
			Div(Class("qm-strip"),
				Iff(len(reserveItems) == 0, func() Node {
					return Span(Class("roster-empty"), Text("The Reserve is empty — every company is committed."))
				}),
				If(len(reserveItems) > 0, Group(reserveItems)),
			),
			Div(Class("qm-strength"), strengthLineReserve(data.reserve)),
			Div(Class("xfer-grid"),
				Label(Class("field-group"),
					Span(Class("field-label"), Text("From")),
					Select(Class("select"),
						ds.Bind("xfer_from"),
						Option(Value("0"), Text("The Reserve")),
						Group(Map(atHomeLegions, func(l db.ListLegionsForKingdomRow) Node {
							return Option(Value(strconv.Itoa(l.ID)), Text(l.Name))
						})),
					),
				),
				Label(Class("field-group"),
					Span(Class("field-label"), Text("To")),
					Select(Class("select"),
						ds.Bind("xfer_to"),
						Option(Value("0"), Text("The Reserve")),
						Group(Map(atHomeLegions, func(l db.ListLegionsForKingdomRow) Node {
							return Option(Value(strconv.Itoa(l.ID)), Text(l.Name))
						})),
						If(canCreateLegion, Option(Value("-1"), Text("＋ New Legion"))),
					),
				),
				Label(Class("field-group"),
					Span(Class("field-label"), Text("Company")),
					Select(Class("select"),
						ds.Bind("xfer_unit"),
						Group(Map(allOrdered, func(utype string) Node {
							return If(existing[utype], Option(Value(utype), Text(game.UnitDefs[utype].Name)))
						})),
					),
				),
				Label(Class("field-group xfer-count"),
					Span(Class("field-label"), Text("Count")),
					Input(Class("field field--num"), Type("number"), Min("1"), Value("1"), ds.Bind("xfer_count")),
				),
				Button(Class("btn btn--primary xfer-go"),
					ds.On("click", datastar.PostSSE(routes.KingdomArmyTransferPath)),
					Text("Transfer"),
				),
			),
		),
	)
}

// marchOrdersCard is the single place a legion is sent to war. Centralising
// it (mirroring the Quartermaster transfer card) avoids the previous per-legion
// dispatch forms, which all shared one send_* signal set and so looked
// independent but were not. The selected legion, order, target, and duration
// all live in shared datastar signals bound here; the March button just posts.
func marchOrdersCard(data armyData) Node {
	var atHome []db.ListLegionsForKingdomRow
	for _, l := range data.legions {
		if !l.CampaignStatus.Valid {
			atHome = append(atHome, l)
		}
	}
	attackOptions := durationOptions(1, 5)
	defendOptions := durationOptions(1, 24)
	return Div(Class("card"),
		Div(Class("card-inner"),
			SectionHeader("Orders", ""),
			Iff(len(atHome) == 0, func() Node {
				return P(Class("dispatch-empty"),
					Text("No available legion to order."))
			}),
			If(len(atHome) > 0, Div(Class("march-form"),
				Label(Class("field-group"),
					Span(Class("field-label"), Text("Legion")),
					Select(Class("select"), ds.Bind("send_legion"),
						Group(Map(atHome, func(l db.ListLegionsForKingdomRow) Node {
							return Option(Value(strconv.Itoa(l.ID)), Text(l.Name))
						})),
					),
				),
				Div(Class("field-group"),
					Span(Class("field-label"), Text("Order")),
					Div(Class("action-toggle"),
						Button(Type("button"),
							Classes{"seg": true, "seg--attack": true},
							ds.Class("'is-on'", "$send_action === 'attack'"),
							ds.On("click", "$send_action = 'attack'; $send_ticks = 4"),
							Text("Attack"),
						),
						Button(Type("button"),
							Classes{"seg": true, "seg--defend": true},
							ds.Class("'is-on'", "$send_action === 'defend'"),
							ds.On("click", "$send_action = 'defend'; $send_ticks = 12"),
							Text("Defend"),
						),
					),
				),
				Label(Class("field-group"),
					Span(Class("field-label"), Text("Target Kingdom")),
					Input(Class("field"), Type("text"), Placeholder("Name a Kingdom…"), ds.Bind("send_target")),
				),
				Label(Class("field-group"),
					Span(Class("field-label"), Text("Time at target")),
					Select(Class("select"),
						ds.Bind("send_ticks"), ds.Show("$send_action === 'attack'"),
						Group(attackOptions),
					),
					Select(Class("select"),
						ds.Bind("send_ticks"), ds.Show("$send_action === 'defend'"),
						Group(defendOptions),
					),
				),
				Button(Classes{"btn": true, "dispatch-go": true},
					ds.Class("'btn--danger'", "$send_action === 'attack'", "'btn--accent'", "$send_action === 'defend'"),
					ds.On("click", datastar.PostSSE(routes.KingdomArmySendPath)),
					Text("Send orders"),
				),
				Div(Class("march-preview"),
					Group(Map(atHome, func(l db.ListLegionsForKingdomRow) Node {
						return Div(Classes{"march-preview-legion": true},
							ds.Show(fmt.Sprintf("$send_legion == %d", l.ID)),
							Span(Class("march-preview-name"), Text(l.Name)),
							strengthLine(data.legionUnits[l.ID]),
							rosterStrip(data.legionUnits[l.ID], 6, 52),
						)
					})),
				),
			)),
		),
	)
}

func legionCard(l db.ListLegionsForKingdomRow, units []db.KingdomLegionUnit, campaign *db.GetCampaignsForKingdomRow, campaignUnits []db.KingdomCampaignUnit, otherIndex map[int]string) Node {
	afield := campaign != nil
	returning := afield && campaign.Status == "returning"
	return Div(Classes{"legion-card": true, "is-afield": afield, "is-returning": returning},
		// At-home head: crest + "At home" pill.
		Iff(!afield, func() Node {
			return Div(Class("legion-card-head"),
				legionCrest(l.Number, afield),
				statusPill(campaign),
			)
		}),
		// Afield: one strip with identity · status · clock, roster as the body.
		Iff(afield, func() Node {
			eta := campaignETA(*campaign)
			etaUnit := "ticks"
			if eta == 1 {
				etaUnit = "tick"
			}
			return Div(Class("legion-campaign"),
				Div(Class("cc-head"),
					Div(Class("cc-head-left"),
						legionCrest(l.Number, afield),
						Span(Class("afield-order"),
							actionTag(campaign.Action, true),
							Span(Class("afield-arrow"), Text("→")),
							Span(Class("afield-target"), Text(otherIndex[campaign.TargetKingdomID])),
						),
					),
					campaignTimeline(*campaign),
					Div(Class("cc-head-right"),
						Div(Class("cc-eta"),
							Span(Class("cc-eta-lbl"), Text("Returns home in")),
							Span(Class("cc-eta-val"),
								Text(strconv.Itoa(eta)),
								Span(Class("cc-eta-unit"), Text(etaUnit)),
							),
						),
					),
				),
				Div(Class("cc-body"),
					Div(Class("cc-roster"),
						rosterStripCampaign(campaignUnits, 6, 50),
					),
				),
				Div(Class("legion-foot"),
					strengthLineCampaign(campaignUnits),
					Button(Class("btn btn--sm"),
						ds.On("click", datastar.PostSSE("%s", cancelURL(campaign.ID))),
						Text("Recall"),
					),
				),
			)
		}),
		If(!afield,
			Div(Class("legion-muster"),
				rosterStrip(units, 6, 52),
				Div(Class("legion-foot"),
					strengthLine(units),
					Button(Class("btn btn--quiet"),
						ds.On("click", fmt.Sprintf("if(!confirm(%q))return; %s",
							"Disband this legion? Its companies return to the Reserve.",
							datastar.PostSSE("%s", disbandURL(l.ID)))),
						Text("Disband legion"),
					),
				),
			),
		),
	)
}

func legionSlotCard(slotNum int) Node {
	return Div(Classes{"open-slot": true, "legion-slot": true},
		Span(Class("open-slot__crest"), Text(romanNumeral(slotNum))),
		Span(Class("open-slot__copy"),
			Span(Class("open-slot__h"), Text(fmt.Sprintf("Vacant · Legion %s", romanNumeral(slotNum)))),
			Span(Class("open-slot__sub"), Text("Transfer troops from the reserve to use this legion.")),
		),
	)
}

// summaryStrip renders the thin command-summary stat bar above the body.
func summaryStrip(data armyData) Node {
	raised := len(data.legions)
	vacantLegions := game.MaxLegionsPerKingdom - raised
	freeLegions := raised - len(data.campaigns)

	var totalUnits, totalPower int
	for _, u := range data.reserve {
		totalUnits += u.Count
		totalPower += game.UnitDefs[u.UnitType].Power * u.Count
	}
	for _, units := range data.legionUnits {
		for _, u := range units {
			totalUnits += u.Count
			totalPower += game.UnitDefs[u.UnitType].Power * u.Count
		}
	}
	for _, units := range data.campaignUnits {
		for _, u := range units {
			totalUnits += u.Count
			totalPower += game.UnitDefs[u.UnitType].Power * u.Count
		}
	}

	powerTone := StatDefault
	if totalPower == 0 {
		powerTone = StatMuted
	}
	return SummaryStrip(
		SummaryStat{Label: "Vacant legions", Sub: fmt.Sprintf("/ %d", game.MaxLegionsPerKingdom), Num: vacantLegions},
		SummaryStat{Label: "Free legions", Num: freeLegions},
		SummaryStat{Label: "Total units", Num: totalUnits},
		SummaryStat{Label: "Total power", Num: totalPower, Tone: powerTone},
	)
}

func armyContent(kingdom *db.Kingdom, data armyData, targetName, action string) Node {
	otherIndex := make(map[int]string, len(data.others))
	for _, o := range data.others {
		otherIndex[o.ID] = o.Name
	}

	campaignByLegion := make(map[int]*db.GetCampaignsForKingdomRow, len(data.campaigns))
	for i := range data.campaigns {
		c := &data.campaigns[i]
		campaignByLegion[c.LegionID] = c
	}

	legionByNumber := make(map[int]db.ListLegionsForKingdomRow, len(data.legions))
	for _, l := range data.legions {
		legionByNumber[l.Number] = l
	}

	var defaultSendLegion int
	for _, l := range data.legions {
		if !l.CampaignStatus.Valid {
			defaultSendLegion = l.ID
			break
		}
	}
	defaultTicks := 4
	if action == "defend" {
		defaultTicks = 12
	}

	// Group cards by state so the time-sensitive afield legions lead, then
	// at-home legions, then vacant legion slots. Within each group legions stay
	// in number order (ListLegionsForKingdom already sorts by number).
	var afieldCards, homeCards, slotCards []Node
	for _, l := range data.legions {
		campaign := campaignByLegion[l.ID]
		var cu []db.KingdomCampaignUnit
		if campaign != nil {
			cu = data.campaignUnits[campaign.ID]
		}
		card := legionCard(l, data.legionUnits[l.ID], campaign, cu, otherIndex)
		if campaign != nil {
			afieldCards = append(afieldCards, card)
		} else {
			homeCards = append(homeCards, card)
		}
	}
	for n := 1; n <= game.MaxLegionsPerKingdom; n++ {
		if _, ok := legionByNumber[n]; !ok {
			slotCards = append(slotCards, legionSlotCard(n))
		}
	}
	legionCards := append(append(afieldCards, homeCards...), slotCards...)

	return Div(Class("army-content"),
		ds.Signals(map[string]any{
			"xfer_from":   0,
			"xfer_to":     0,
			"xfer_unit":   firstKingdomUnit(data),
			"xfer_count":  1,
			"send_legion": defaultSendLegion,
			"send_action": action,
			"send_target": targetName,
			"send_ticks":  defaultTicks,
		}, ds.ModifierIfMissing),
		Div(ds.Init(GetSSENoSignals(routes.KingdomArmyRefreshPath))),
		armyAlert(nil),
		PageHeader("Campaign"),
		summaryStrip(data),
		quartermasterCard(data),
		marchOrdersCard(data),
		Div(Class("legion-field"),
			SectionHeader("Your Legions", ""),
			Div(Class("legion-stack"),
				Group(legionCards),
			),
		),
	)
}

func campaignETA(c db.GetCampaignsForKingdomRow) int {
	switch c.Status {
	case "en_route":
		return c.TicksRemaining + c.ActionTicks + c.TravelTicks
	case "active":
		return c.TicksRemaining + c.TravelTicks
	case "returning":
		return c.TicksRemaining
	}
	return 0
}

func durationOptions(min, max int) []Node {
	var opts []Node
	for t := min; t <= max; t++ {
		opts = append(opts, Option(Value(strconv.Itoa(t)), Text(fmt.Sprintf("%d ticks", t))))
	}
	return opts
}

func armyAlert(inner Node) Node { return AlertContainer("army-alert", inner) }
