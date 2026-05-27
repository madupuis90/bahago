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
	"github.com/jackc/pgx/v5/pgtype"
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

// ── Route registration ───────────────────────────────────────662943─────────────────

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
		campaigns, err := q.GetCampaignsForKingdom(ctx, kingdom.ID)
		if err != nil {
			return fmt.Errorf("get campaigns: %w", err)
		}
		for _, c := range campaigns {
			if c.LegionID == toID {
				return ErrLegionInUse
			}
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

func armyContent(kingdom *db.Kingdom, data armyData, targetName, action string) Node {
	otherIndex := make(map[int]string, len(data.others))
	for _, o := range data.others {
		otherIndex[o.ID] = o.Name
	}

	// Default send legion = first at-home legion.
	var defaultSendLegion int
	for _, l := range data.legions {
		if !l.CampaignStatus.Valid {
			defaultSendLegion = l.ID
			break
		}
	}

	return Div(
		H1(Class("page-title"), Text("Army")),
		ds.Signals(map[string]any{
			"xfer_from":   0,
			"xfer_to":     0,
			"xfer_unit":   firstKingdomUnit(data),
			"xfer_count":  1,
			"send_legion": defaultSendLegion,
			"send_action": action,
			"send_target": targetName,
			"send_ticks":  4,
		}, ds.ModifierIfMissing),
		Div(ds.Init(GetSSENoSignals(routes.KingdomArmyRefreshPath))),
		armyAlert(nil),
		transferForm(data),
		reservePanel(data.reserve),
		Group(Map(data.legions, func(l db.ListLegionsForKingdomRow) Node {
			return legionPanel(l, data.legionUnits[l.ID])
		})),
		sendForm(data),
		campaignsSection(data.campaigns, data.campaignUnits, otherIndex),
	)
}

func transferForm(data armyData) Node {
	var atHomeLegions []db.ListLegionsForKingdomRow
	for _, l := range data.legions {
		if !l.CampaignStatus.Valid {
			atHomeLegions = append(atHomeLegions, l)
		}
	}

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

	canCreateLegion := len(data.legions) < game.MaxLegionsPerKingdom
	allOrdered := append(append([]string{}, game.UnitOrder...), game.SummonOrder...)

	return Div(Class("army-section panel"),
		P(Class("panel-title"), Text("Transfer Units")),
		Div(Class("army-form"),
			Label(For("xfer-from"), Text("From")),
			Select(
				ID("xfer-from"),
				ds.Bind("xfer_from"),
				Option(Value("0"), Text("Reserve")),
				Group(Map(atHomeLegions, func(l db.ListLegionsForKingdomRow) Node {
					return Option(Value(strconv.Itoa(l.ID)), Text(l.Name))
				})),
			),

			Label(For("xfer-to"), Text("To")),
			Select(
				ID("xfer-to"),
				ds.Bind("xfer_to"),
				Option(Value("0"), Text("Reserve")),
				Group(Map(atHomeLegions, func(l db.ListLegionsForKingdomRow) Node {
					return Option(Value(strconv.Itoa(l.ID)), Text(l.Name))
				})),
				If(canCreateLegion, Option(Value("-1"), Text("New Legion"))),
			),

			Label(For("xfer-unit"), Text("Unit type")),
			Select(
				ID("xfer-unit"),
				ds.Bind("xfer_unit"),
				Group(Map(allOrdered, func(utype string) Node {
					return If(existing[utype], Option(Value(utype), Text(game.UnitDefs[utype].Name)))
				})),
			),

			Label(For("xfer-count"), Text("Count")),
			Input(
				ID("xfer-count"),
				Type("number"),
				Min("1"),
				Value("1"),
				ds.Bind("xfer_count"),
			),

			Button(
				Class("btn"),
				ds.On("click", datastar.PostSSE(routes.KingdomArmyTransferPath)),
				Text("Transfer"),
			),
		),
	)
}

func reservePanel(reserve []db.GetAvailableKingdomUnitsRow) Node {
	return Div(Class("army-section panel"),
		P(Class("panel-title"), Text("Reserve")),
		Iff(len(reserve) == 0, func() Node {
			return P(Class("army-empty"), Text("No units in reserve."))
		}),
		Iff(len(reserve) > 0, func() Node {
			return Table(Class("army-table"),
				THead(Tr(
					Th(Text("Unit")),
					Th(Text("Count")),
					Th(Text("Power")),
				)),
				TBody(Group(Map(reserve, func(u db.GetAvailableKingdomUnitsRow) Node {
					unit := game.UnitDefs[u.UnitType]
					return Tr(
						Td(Text(unit.Name)),
						Td(Text(strconv.Itoa(u.Count))),
						Td(Text(strconv.Itoa(unit.Power))),
					)
				}))),
			)
		}),
	)
}

func legionPanel(l db.ListLegionsForKingdomRow, units []db.KingdomLegionUnit) Node {
	isDeployed := l.CampaignStatus.Valid
	return Div(Class("army-section panel army-legion"),
		Div(Class("army-legion-header"),
			P(Class("panel-title"), Text(l.Name)),
			Span(Classes{
				"army-legion-badge":           true,
				"army-legion-badge--home":     !isDeployed,
				"army-legion-badge--deployed": isDeployed,
			}, Text(legionStatusLabel(l.CampaignStatus))),
		),
		Iff(len(units) == 0, func() Node {
			return P(Class("army-empty"), Text("No units assigned."))
		}),
		Iff(len(units) > 0, func() Node {
			return Table(Class("army-table"),
				THead(Tr(
					Th(Text("Unit")),
					Th(Text("Count")),
					Th(Text("Power")),
				)),
				TBody(Group(Map(units, func(u db.KingdomLegionUnit) Node {
					unit := game.UnitDefs[u.UnitType]
					return Tr(
						Td(Text(unit.Name)),
						Td(Text(strconv.Itoa(u.Count))),
						Td(Text(strconv.Itoa(unit.Power))),
					)
				}))),
			)
		}),
		Iff(!isDeployed, func() Node {
			return Button(
				Class("btn btn-text"),
				ds.On("click", datastar.PostSSE("%s", disbandURL(l.ID))),
				Text("Disband"),
			)
		}),
	)
}

func legionStatusLabel(status pgtype.Text) string {
	if !status.Valid {
		return "At Home"
	}
	switch status.String {
	case "en_route":
		return "En Route"
	case "active":
		return "Active"
	case "returning":
		return "Returning"
	}
	return status.String
}

func sendForm(data armyData) Node {
	var atHomeLegions []db.ListLegionsForKingdomRow
	for _, l := range data.legions {
		if !l.CampaignStatus.Valid {
			atHomeLegions = append(atHomeLegions, l)
		}
	}

	attackOptions := durationOptions(1, 5)
	defendOptions := durationOptions(1, 24)

	return Div(Class("army-section panel"),
		P(Class("panel-title"), Text("Send Campaign")),
		Div(Class("army-form"),
			Label(For("send-legion"), Text("Legion")),
			Select(
				ID("send-legion"),
				ds.Bind("send_legion"),
				Iff(len(atHomeLegions) == 0, func() Node {
					return Option(Value("0"), Text("No legions available"))
				}),
				Group(Map(atHomeLegions, func(l db.ListLegionsForKingdomRow) Node {
					return Option(Value(strconv.Itoa(l.ID)), Text(l.Name))
				})),
			),

			Label(For("send-action"), Text("Action")),
			Select(
				ID("send-action"),
				ds.Bind("send_action"),
				ds.On("change", "$send_ticks = 4"),
				Option(Value("attack"), Text("Attack")),
				Option(Value("defend"), Text("Defend")),
			),

			Label(For("send-duration"), Text("Duration")),
			Div(
				Select(
					ID("send-duration"),
					ds.Bind("send_ticks"),
					ds.Show(`$send_action === 'attack'`),
					Group(attackOptions),
				),
				Select(
					ds.Bind("send_ticks"),
					ds.Show(`$send_action === 'defend'`),
					Group(defendOptions),
				),
			),

			Label(For("send-target"), Text("Target kingdom")),
			Input(
				ID("send-target"),
				Type("text"),
				Placeholder("Kingdom name"),
				ds.Bind("send_target"),
			),

			Button(
				Class("btn"),
				ds.On("click", datastar.PostSSE(routes.KingdomArmySendPath)),
				Text("Send"),
			),
		),
	)
}

func campaignsSection(campaigns []db.GetCampaignsForKingdomRow, campaignUnits map[int][]db.KingdomCampaignUnit, otherIndex map[int]string) Node {
	return Div(Class("army-section panel"),
		P(Class("panel-title"), Text("Active Campaigns")),
		Iff(len(campaigns) == 0, func() Node {
			return P(Class("army-empty"), Text("No active campaigns."))
		}),
		Iff(len(campaigns) > 0, func() Node {
			return Table(Class("army-table"),
				THead(Tr(
					Th(Text("Legion")),
					Th(Text("Units")),
					Th(Text("Action")),
					Th(Text("Target")),
					Th(Text("Status")),
					Th(Text("Phase ends in")),
					Th(Text("Returns in")),
					Th(Text("")),
				)),
				TBody(Group(Map(campaigns, func(c db.GetCampaignsForKingdomRow) Node {
					return campaignRow(c, campaignUnits[c.ID], otherIndex)
				}))),
			)
		}),
	)
}

func campaignRow(c db.GetCampaignsForKingdomRow, units []db.KingdomCampaignUnit, otherIndex map[int]string) Node {
	statusLabel := campaignStatusLabel(c.Status, c.Action)
	eta := campaignETA(c)
	targetName := otherIndex[c.TargetKingdomID]
	if targetName == "" {
		targetName = strconv.Itoa(c.TargetKingdomID)
	}
	canCancel := c.Status != "returning"

	return Tr(
		Td(Text(c.LegionName)),
		Td(Text(unitCompositionStr(units))),
		Td(Text(c.Action)),
		Td(Text(targetName)),
		Td(Text(statusLabel)),
		Td(Text(fmt.Sprintf("%d ticks", c.TicksRemaining))),
		Td(Text(fmt.Sprintf("%d ticks", eta))),
		Td(
			Iff(canCancel, func() Node {
				return Button(
					Class("btn btn-text"),
					ds.On("click", datastar.PostSSE("%s", cancelURL(c.ID))),
					Text("Cancel"),
				)
			}),
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
