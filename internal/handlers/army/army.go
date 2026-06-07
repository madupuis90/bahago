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
		KingdomLayout(r, "Army", routes.KingdomArmyPath, kingdom, armyContent(kingdom, data, targetName, action)).Render(w)
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
			if isSendUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(err)))
				return
			}
			log.Printf("army send: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(errors.New("internal error"))))
			return
		}

		data, err := h.loadArmyData(r, kingdom.ID)
		if err != nil {
			log.Printf("army send: reload: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(errors.New("internal error"))))
			return
		}
		sse := datastar.NewSSE(w, r)
		sse.PatchElementGostar(MainContent(armyContent(kingdom, data, "", "attack")))
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
			if isCancelUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(err)))
				return
			}
			log.Printf("army cancel: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(errors.New("internal error"))))
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
			if isTransferUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(err)))
				return
			}
			log.Printf("army transfer: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(errors.New("internal error"))))
			return
		}

		data, err := h.loadArmyData(r, kingdom.ID)
		if err != nil {
			log.Printf("army transfer: reload: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(errors.New("internal error"))))
			return
		}
		sse := datastar.NewSSE(w, r)
		sse.PatchElementGostar(MainContent(armyContent(kingdom, data, "", "attack")))
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
			if isDisbandUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(err)))
				return
			}
			log.Printf("army disband: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(errors.New("internal error"))))
			return
		}

		data, err := h.loadArmyData(r, kingdom.ID)
		if err != nil {
			log.Printf("army disband: reload: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(armyAlert(AlertError(errors.New("internal error"))))
			return
		}
		sse := datastar.NewSSE(w, r)
		sse.PatchElementGostar(MainContent(armyContent(kingdom, data, "", "attack")))
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
	for _, utype := range append(append([]string{}, game.UnitOrder...), game.SummonOrder...) {
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

func unitIcon(utype string) string {
	switch utype {
	case game.UnitRecruit:
		return "spear"
	case game.UnitArcher:
		return "soldiers"
	case game.UnitRaider:
		return "flag"
	case game.UnitKnight:
		return "swords"
	case game.UnitCatapult:
		return "helmet"
	case game.UnitShade:
		return "flame"
	case game.UnitDreadKnight:
		return "flame"
	}
	return "spear"
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
				tone, label = "combat", "In combat"
			} else {
				tone, label = "guard", "On guard"
			}
		}
	}
	return Span(Classes{"status-pill": true, "status-pill--" + tone: true},
		Span(Class("status-dot")),
		Text(label),
	)
}

func actionTag(action string) Node {
	var tone, icon, label string
	if action == "attack" {
		tone, icon, label = "attack", "swords", "Attack"
	} else {
		tone, icon, label = "defend", "chevron", "Defend"
	}
	return Span(Classes{"action-tag": true, "action-tag--" + tone: true},
		Icon("shield-"+icon, 13, false),
		Text(label),
	)
}

func unitToken(utype string, count, size int) Node {
	unit := game.UnitDefs[utype]
	sz := strconv.Itoa(size)
	return Div(Class("unit-token"),
		Div(Class("unit-medallion"),
			Style(fmt.Sprintf("width:%spx;height:%spx", sz, sz)),
			Title(unit.Name),
			Icon("shield-"+unitIcon(utype), size/2, false),
		),
		Span(Class("unit-tally"), Text(strconv.Itoa(count))),
	)
}

func rosterStrip(units []db.KingdomLegionUnit, max, size int) Node {
	allOrdered := append(append([]string{}, game.UnitOrder...), game.SummonOrder...)
	byType := make(map[string]int, len(units))
	for _, u := range units {
		byType[u.UnitType] = u.Count
	}
	var tokens []Node
	extra := 0
	for _, utype := range allOrdered {
		count := byType[utype]
		if count <= 0 {
			continue
		}
		if len(tokens) < max {
			tokens = append(tokens, unitToken(utype, count, size))
		} else {
			extra++
		}
	}
	isEmpty := len(tokens) == 0 && extra == 0
	return Div(Class("roster-strip"),
		Group(tokens),
		If(extra > 0, Span(Class("roster-more"), Text(fmt.Sprintf("+%d", extra)))),
		If(isEmpty, Span(Class("roster-empty"), Text("Empty — transfer companies from the Reserve above."))),
	)
}

func rosterStripCampaign(units []db.KingdomCampaignUnit, max, size int) Node {
	allOrdered := append(append([]string{}, game.UnitOrder...), game.SummonOrder...)
	byType := make(map[string]int, len(units))
	for _, u := range units {
		byType[u.UnitType] = u.Count
	}
	var tokens []Node
	extra := 0
	for _, utype := range allOrdered {
		count := byType[utype]
		if count <= 0 {
			continue
		}
		if len(tokens) < max {
			tokens = append(tokens, unitToken(utype, count, size))
		} else {
			extra++
		}
	}
	isEmpty := len(tokens) == 0 && extra == 0
	return Div(Class("roster-strip"),
		Group(tokens),
		If(extra > 0, Span(Class("roster-more"), Text(fmt.Sprintf("+%d", extra)))),
		If(isEmpty, Span(Class("roster-empty"), Text("No companies committed."))),
	)
}

func renderStrengthLine(power, total int) Node {
	return Div(Class("strength-line"),
		Span(Class("sl-item"), Icon("shield-swords", 14, false), B(Text(strconv.Itoa(power))), Text(" power")),
		Span(Class("sl-sep"), Text("·")),
		Span(Class("sl-item"), B(Text(strconv.Itoa(total))), Text(" units")),
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
	midLabel := "At the gates"
	if c.Action == "defend" {
		midLabel = "On guard"
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
	var stops []Node
	for i, p := range phases {
		state := "todo"
		if i < at {
			state = "done"
		} else if i == at {
			state = "now"
		}
		stops = append(stops,
			Div(Classes{"tl-stop": true, "tl-stop--" + state: true},
				If(i > 0, Span(Class("tl-line"))),
				Span(Class("tl-dot")),
				Span(Class("tl-label"), Text(p.label)),
			),
		)
	}
	return Div(Class("timeline"), Group(stops))
}

func legionCrest(n int, afield bool, size int) Node {
	h := int(float64(size) * 1.18)
	return Div(
		Classes{"legion-crest": true, "is-afield": afield},
		Style(fmt.Sprintf("width:%dpx;height:%dpx", size, h)),
		Span(Class("crest-num"),
			Style(fmt.Sprintf("font-size:%dpx", int(float64(size)*0.46))),
			Text(romanNumeral(n)),
		),
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
	allOrdered := append(append([]string{}, game.UnitOrder...), game.SummonOrder...)

	var atHomeLegions []db.ListLegionsForKingdomRow
	for _, l := range data.legions {
		if !l.CampaignStatus.Valid {
			atHomeLegions = append(atHomeLegions, l)
		}
	}
	canCreateLegion := len(data.legions) < game.MaxLegionsPerKingdom

	var reserveTokens []Node
	for _, u := range data.reserve {
		if u.Count > 0 {
			reserveTokens = append(reserveTokens, unitToken(u.UnitType, u.Count, 46))
		}
	}

	return Div(Class("qm-card qm-stacked card is-lit"),
		Div(Class("card-header-row"),
			P(Class("section-title"), Text("The Reserve")),
			strengthLineReserve(data.reserve),
		),
		Div(Class("qm-strip"),
			Iff(len(reserveTokens) == 0, func() Node {
				return Span(Class("roster-empty"), Text("The Reserve is empty — every company is committed."))
			}),
			If(len(reserveTokens) > 0, Group(reserveTokens)),
		),
		Div(Class("qm-stacked-form"),
			Span(Class("eyebrow"), Text("Transfer companies")),
			Div(Class("xfer-grid"),
				Label(Class("field-group"),
					Span(Class("field-label"), Text("From")),
					Select(Class("field"),
						ds.Bind("xfer_from"),
						Option(Value("0"), Text("The Reserve")),
						Group(Map(atHomeLegions, func(l db.ListLegionsForKingdomRow) Node {
							return Option(Value(strconv.Itoa(l.ID)), Text(l.Name))
						})),
					),
				),
				Label(Class("field-group"),
					Span(Class("field-label"), Text("To")),
					Select(Class("field"),
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
					Select(Class("field"),
						ds.Bind("xfer_unit"),
						Group(Map(allOrdered, func(utype string) Node {
							return If(existing[utype], Option(Value(utype), Text(game.UnitDefs[utype].Name)))
						})),
					),
				),
				Label(Class("field-group xfer-count"),
					Span(Class("field-label"), Text("Count")),
					Input(Class("field"), Type("number"), Min("1"), Value("1"), ds.Bind("xfer_count")),
				),
				Button(Class("btn btn--primary xfer-go"),
					ds.On("click", datastar.PostSSE(routes.KingdomArmyTransferPath)),
					Text("Transfer"),
				),
			),
		),
	)
}

func legionDispatch(l db.ListLegionsForKingdomRow, hasUnits bool) Node {
	if !hasUnits {
		return Div(Class("legion-dispatch"),
			Span(Class("eyebrow"), Text("March orders")),
			P(Class("dispatch-empty"), Text("Transfer companies into this banner from the Reserve before it can march.")),
		)
	}
	attackOptions := durationOptions(1, 5)
	defendOptions := durationOptions(1, 24)
	marchExpr := fmt.Sprintf("$send_legion = %d; %s", l.ID, datastar.PostSSE(routes.KingdomArmySendPath))
	return Div(Class("legion-dispatch"),
		Span(Class("eyebrow"), Text("March orders")),
		Div(Class("field-group"),
			Span(Class("field-label"), Text("Order")),
			Div(Class("action-toggle"),
				Button(Type("button"),
					Classes{"seg": true, "seg--attack": true},
					ds.Class("'is-on'", "$send_action === 'attack'"),
					ds.On("click", "$send_action = 'attack'; $send_ticks = 4"),
					Icon("shield-swords", 13, false),
					Text("Attack"),
				),
				Button(Type("button"),
					Classes{"seg": true, "seg--defend": true},
					ds.Class("'is-on'", "$send_action === 'defend'"),
					ds.On("click", "$send_action = 'defend'; $send_ticks = 12"),
					Icon("shield-chevron", 13, false),
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
			Select(Class("field"),
				ds.Bind("send_ticks"),
				ds.Show("$send_action === 'attack'"),
				Group(attackOptions),
			),
			Select(Class("field"),
				ds.Bind("send_ticks"),
				ds.Show("$send_action === 'defend'"),
				Group(defendOptions),
			),
		),
		Button(Class("btn btn--primary dispatch-go"),
			ds.On("click", marchExpr),
			Icon("shield-flag", 14, false),
			Span(ds.Text("$send_action === 'attack' ? 'March to war' : 'Send to defend'")),
		),
	)
}

func legionCard(l db.ListLegionsForKingdomRow, units []db.KingdomLegionUnit, campaign *db.GetCampaignsForKingdomRow, campaignUnits []db.KingdomCampaignUnit, otherIndex map[int]string) Node {
	afield := campaign != nil
	returning := afield && campaign.Status == "returning"
	var totalUnits int
	for _, u := range units {
		totalUnits += u.Count
	}
	return Div(Classes{"legion-card": true, "card": true, "is-afield": afield, "is-returning": returning},
		Div(Class("legion-card-head"),
			legionCrest(l.Number, afield, 46),
			Div(Class("legion-card-id"),
				Span(Class("legion-name"), Text(l.Name)),
				statusPill(campaign),
			),
			Iff(afield, func() Node {
				return Div(Class("legion-head-order"),
					actionTag(campaign.Action),
					Span(Class("afield-arrow"), Text("→")),
					Span(Class("afield-target"), Text(otherIndex[campaign.TargetKingdomID])),
				)
			}),
		),
		Iff(afield, func() Node {
			eta := campaignETA(*campaign)
			etaUnit := "ticks"
			if eta == 1 {
				etaUnit = "tick"
			}
			phaseUnit := "ticks"
			if campaign.TicksRemaining == 1 {
				phaseUnit = "tick"
			}
			phaseLabel := "This phase ends in"
			if returning {
				phaseLabel = "Almost home"
			}
			return Div(Class("legion-campaign"),
				campaignTimeline(*campaign),
				Div(Class("cc-body"),
					Div(Class("cc-roster"),
						Span(Class("cc-label"), Text("Companies committed")),
						rosterStripCampaign(campaignUnits, 6, 40),
						strengthLineCampaign(campaignUnits),
					),
					Div(Class("cc-eta"),
						Div(Class("cc-eta-row"),
							Span(Class("cc-eta-lbl"), Text(phaseLabel)),
							Span(Class("cc-eta-val"),
								Text(strconv.Itoa(campaign.TicksRemaining)),
								Span(Class("cc-eta-unit"), Text(phaseUnit)),
							),
						),
						Div(Class("cc-eta-row cc-eta-row--total"),
							Span(Class("cc-eta-lbl"), Text("Returns home in")),
							Span(Class("cc-eta-val"),
								Text(strconv.Itoa(eta)),
								Span(Class("cc-eta-unit"), Text(etaUnit)),
							),
						),
					),
				),
				Div(Class("cc-foot"),
					If(returning,
						Span(Class("cc-foot-note"), Text(fmt.Sprintf("Survivors rejoin %s on arrival.", l.Name))),
					),
					If(!returning,
						Span(Class("cc-foot-note"), Text("Returning turns the legion around immediately.")),
					),
					If(!returning,
						Button(Class("btn btn--sm"),
							ds.On("click", datastar.PostSSE("%s", cancelURL(campaign.ID))),
							Text("Return home"),
						),
					),
				),
			)
		}),
		If(!afield, Div(Class("legion-home"),
			Div(Class("legion-muster"),
				rosterStrip(units, 6, 42),
				strengthLine(units),
				Button(Class("btn btn--sm btn--quiet legion-disband"),
					ds.On("click", datastar.PostSSE("%s", disbandURL(l.ID))),
					Text("Disband legion"),
				),
			),
			legionDispatch(l, totalUnits > 0),
		)),
	)
}

func legionSlotCard(slotNum int) Node {
	return Div(Class("legion-card legion-slot"),
		Div(Class("slot-crest"),
			Span(Text(romanNumeral(slotNum))),
		),
		Div(Class("slot-copy"),
			Div(Class("slot-h"), Text(fmt.Sprintf("Legion %s", romanNumeral(slotNum)))),
			Div(Class("slot-sub"), Text("Not formed yet")),
		),
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

	var legionCards []Node
	for n := range game.MaxLegionsPerKingdom {
		slotNum := n + 1
		l, ok := legionByNumber[slotNum]
		if !ok {
			legionCards = append(legionCards, legionSlotCard(slotNum))
			continue
		}
		campaign := campaignByLegion[l.ID]
		var cu []db.KingdomCampaignUnit
		if campaign != nil {
			cu = data.campaignUnits[campaign.ID]
		}
		legionCards = append(legionCards, legionCard(l, data.legionUnits[l.ID], campaign, cu, otherIndex))
	}

	return Div(
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
		Div(Class("page-header"),
			H1(Text("Command Tent")),
		),
		quartermasterCard(data),
		Div(Class("army-grid"),
			Div(Class("legion-stack"),
				Group(legionCards),
			),
		),
	)
}

func campaignStatusLabel(status, action string) string {
	switch status {
	case "en_route":
		return "En route"
	case "active":
		if action == "attack" {
			return "Attacking"
		}
		return "Defending"
	case "returning":
		return "Returning"
	}
	return status
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

func unitCompositionStr(units []db.KingdomCampaignUnit) string {
	unitMap := make(map[string]int, len(units))
	for _, u := range units {
		unitMap[u.UnitType] = u.Count
	}
	var parts []string
	for _, utype := range append(append([]string{}, game.UnitOrder...), game.SummonOrder...) {
		if count, ok := unitMap[utype]; ok {
			parts = append(parts, fmt.Sprintf("%d %s", count, game.UnitDefs[utype].Name))
		}
	}
	return strings.Join(parts, ", ")
}

func durationOptions(min, max int) []Node {
	var opts []Node
	for t := min; t <= max; t++ {
		opts = append(opts, Option(Value(strconv.Itoa(t)), Text(fmt.Sprintf("%d ticks", t))))
	}
	return opts
}

func armyAlert(inner Node) Node { return AlertContainer("army-alert", inner) }
