package units

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
	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/components"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/game"
	"bahago/internal/hub"
	. "bahago/internal/ui"
	"bahago/internal/router"
	"bahago/internal/routes"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ── Input struct (for ReadSignals) ────────────────────────────────────────────

type trainInput struct {
	UnitType string `json:"unit_type"`
	Count    int    `json:"train_count"`
}

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrUnknownUnitType       = errors.New("unknown unit type")
	ErrInvalidCount          = errors.New("count must be at least 1")
	ErrCountTooLarge         = errors.New("count is too large")
	ErrSummonsNotUnlocked    = errors.New("summons not unlocked")
	ErrUnitNotAvailable      = errors.New("unit not available")
	ErrTrainingInProgress    = errors.New("training already in progress")
	ErrInsufficientResources = errors.New("not enough resources")
)

// ── Route registration ────────────────────────────────────────────────────────

func RegisterRoutes(r router.Router, queries db.Querier, pool *pgxpool.Pool, tickHub *hub.Hub) {
	h := newHandler(queries, pool, tickHub)
	r.HandleFunc("GET "+routes.KingdomUnitsPath, h.handleUnitsPage())
	r.HandleFunc("GET "+routes.KingdomUnitsRefreshPath, h.handleUnitsRefresh())
	r.HandleFunc("POST "+routes.KingdomUnitsTrainPath, h.handleTrain())
}

type handler struct {
	queries db.Querier
	pool    *pgxpool.Pool
	hub     *hub.Hub
}

func newHandler(queries db.Querier, pool *pgxpool.Pool, tickHub *hub.Hub) *handler {
	return &handler{queries: queries, pool: pool, hub: tickHub}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handleUnitsPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		units, err := h.queries.GetKingdomUnits(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("units page: get units: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		buildings, err := h.queries.GetKingdomBuildings(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("units page: get buildings: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		training, err := loadTraining(r.Context(), h.queries, kingdom.ID)
		if err != nil {
			log.Printf("units page: get training: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		KingdomLayout(r, "Units", r.URL.Path, kingdom, unitsContent(kingdom, units, buildings, training)).Render(w)
	}
}

func (h *handler) handleUnitsRefresh() http.HandlerFunc {
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
				units, err := h.queries.GetKingdomUnits(r.Context(), k.ID)
				if err != nil {
					log.Printf("units refresh: get units: %v", err)
					return
				}
				buildings, err := h.queries.GetKingdomBuildings(r.Context(), k.ID)
				if err != nil {
					log.Printf("units refresh: get buildings: %v", err)
					return
				}
				training, err := loadTraining(r.Context(), h.queries, k.ID)
				if err != nil {
					log.Printf("units refresh: get training: %v", err)
					sse.PatchElementGostar(unitsAlert(AlertError(errors.New("internal error"))))
					return
				}
				page := unitsContent(&k, units, buildings, training)
				if err := sse.PatchElementGostar(MainContent(page)); err != nil {
					log.Printf("units refresh: patch: %v", err)
					return
				}
			}
		}
	}
}

func (h *handler) handleTrain() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		input := &trainInput{}
		if err := datastar.ReadSignals(r, input); err != nil {
			log.Printf("train: read signals: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsAlert(AlertError(errors.New("invalid request"))))
			return
		}

		if errs := validateTrainInput(input); len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(unitsAlert(AlertError(errs...)))
			return
		}

		if err := h.trainUnits(r.Context(), kingdom, input); err != nil {
			if isTrainUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(unitsAlert(AlertError(err)))
				return
			}
			log.Printf("train: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsAlert(AlertError(errors.New("internal error"))))
			return
		}

		// Reload everything from DB — DeductUnitCost changed the kingdom's resources.
		k, err := h.queries.GetKingdomByID(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("train: reload kingdom: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsAlert(AlertError(errors.New("internal error"))))
			return
		}
		allUnits, err := h.queries.GetKingdomUnits(r.Context(), k.ID)
		if err != nil {
			log.Printf("train: reload units: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsAlert(AlertError(errors.New("internal error"))))
			return
		}
		allBuildings, err := h.queries.GetKingdomBuildings(r.Context(), k.ID)
		if err != nil {
			log.Printf("train: reload buildings: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsAlert(AlertError(errors.New("internal error"))))
			return
		}
		training, err := loadTraining(r.Context(), h.queries, k.ID)
		if err != nil {
			log.Printf("train: reload training: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsAlert(AlertError(errors.New("internal error"))))
			return
		}

		page := unitsContent(&k, allUnits, allBuildings, training)
		sse := datastar.NewSSE(w, r)
		if err := sse.PatchElementGostar(MainContent(page)); err != nil {
			log.Printf("train: patch: %v", err)
		}
	}
}

// ── Validation ────────────────────────────────────────────────────────────────

// validateTrainInput runs every field-level rule that does not need DB access.
// Summons-unlock and building-prerequisite checks need kingdom/building data,
// so they live in the orchestrator.
func validateTrainInput(input *trainInput) []error {
	var errs []error
	if _, ok := game.UnitDefs[input.UnitType]; !ok {
		errs = append(errs, ErrUnknownUnitType)
	}
	if input.Count <= 0 {
		errs = append(errs, ErrInvalidCount)
	} else if input.Count > game.MaxUnitInput {
		errs = append(errs, ErrCountTooLarge)
	}
	return errs
}

// ── Orchestration ─────────────────────────────────────────────────────────────

// trainUnits enforces the summons-unlock and prerequisite checks, then opens a
// SERIALIZABLE transaction to enforce the one-active-training cap and deduct
// cost. PostgreSQL SSI detects concurrent double-starts; the
// SerializationFailure that fires at commit-time is translated back to
// ErrTrainingInProgress.
func (h *handler) trainUnits(ctx context.Context, kingdom *db.Kingdom, input *trainInput) error {
	unit := game.UnitDefs[input.UnitType]

	if unit.IsSummon && !game.CanTrainSummons(*kingdom) {
		return ErrSummonsNotUnlocked
	}

	// Prerequisite check (outside tx — no concurrency concern here).
	buildings, err := h.queries.GetKingdomBuildings(ctx, kingdom.ID)
	if err != nil {
		return fmt.Errorf("get buildings: %w", err)
	}
	if !game.CanTrain(input.UnitType, game.BuildingCountMap(buildings)) {
		return ErrUnitNotAvailable
	}

	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	txq := db.New(tx)

	// One active training order at a time — enforced inside the serializable tx
	// so concurrent requests cannot both pass the check simultaneously.
	existing, err := loadTraining(ctx, txq, kingdom.ID)
	if err != nil {
		return fmt.Errorf("check existing training: %w", err)
	}
	if existing != nil {
		return ErrTrainingInProgress
	}

	if _, err := txq.DeductUnitCost(ctx, db.DeductUnitCostParams{
		KingdomID: kingdom.ID,
		WoodCost:  unit.Cost.Wood * input.Count,
		StoneCost: unit.Cost.Stone * input.Count,
		ManaCost:  unit.Cost.Mana * input.Count,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInsufficientResources
		}
		return fmt.Errorf("deduct cost: %w", err)
	}

	if err := txq.StartTraining(ctx, db.StartTrainingParams{
		KingdomID:      kingdom.ID,
		UnitType:       input.UnitType,
		Count:          input.Count,
		TicksRemaining: unit.Ticks,
	}); err != nil {
		return fmt.Errorf("start training: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.SerializationFailure {
			return ErrTrainingInProgress
		}
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func isTrainUserError(err error) bool {
	return errors.Is(err, ErrSummonsNotUnlocked) ||
		errors.Is(err, ErrUnitNotAvailable) ||
		errors.Is(err, ErrTrainingInProgress) ||
		errors.Is(err, ErrInsufficientResources)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// loadTraining fetches the active training order for a kingdom.
// Returns (nil, nil) if no training is active, (nil, err) on a real DB error.
func loadTraining(ctx context.Context, queries db.Querier, kingdomID int) (*db.KingdomTraining, error) {
	t, err := queries.GetKingdomTraining(ctx, kingdomID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ── Components ────────────────────────────────────────────────────────────────

func unitsContent(kingdom *db.Kingdom, units []db.KingdomUnit, buildings []db.KingdomBuilding, training *db.KingdomTraining) Node {
	counts := game.UnitCountMap(units)
	buildingCounts := game.BuildingCountMap(buildings)
	canSummon := game.CanTrainSummons(*kingdom)

	return Div(
		H1(Class("page-title"), Text("Units")),
		Div(ds.Init(GetSSENoSignals(routes.KingdomUnitsRefreshPath))),
		unitsAlert(nil),
		Iff(training != nil, func() Node { return activeTrainingBanner(training) }),
		unitsTable(counts, buildingCounts),
		If(canSummon, summonTable(counts, buildingCounts)),
		trainForm(buildingCounts, canSummon, training != nil),
	)
}

func activeTrainingBanner(t *db.KingdomTraining) Node {
	def := game.UnitDefs[t.UnitType]
	progress := fmt.Sprintf("%d / %d ticks remaining", t.TicksRemaining, t.TicksTotal)
	return Div(Class("construction-banner panel"),
		Span(Text(fmt.Sprintf("Training: %d × %s", t.Count, def.Name))),
		Span(Text(progress)),
	)
}

func unitsTable(counts map[string]int, buildingCounts map[string]int) Node {
	return Div(Class("units-section panel"),
		P(Class("panel-title"), Text("Units")),
		Table(Class("units-table"),
			THead(
				Tr(
					Th(Text("Name")),
					Th(Text("Count")),
					Th(Text("Wood")),
					Th(Text("Stone")),
					Th(Text("Food Upkeep")),
					Th(Text("Power")),
					Th(Text("Attributes")),
				),
			),
			TBody(
				Group(Map(game.UnitOrder, func(utype string) Node {
					def := game.UnitDefs[utype]
					count := counts[utype]
					locked := !game.CanTrain(utype, buildingCounts)
					return unitRow(def, count, locked)
				})),
			),
		),
	)
}

func unitRow(def game.UnitDef, count int, locked bool) Node {
	return Tr(Classes{"unit-row--locked": locked},
		Td(Text(def.Name)),
		Td(Text(strconv.Itoa(count))),
		Td(Text(strconv.Itoa(def.Cost.Wood))),
		Td(Text(strconv.Itoa(def.Cost.Stone))),
		Td(Text(strconv.Itoa(def.FoodUpkeep))),
		Td(Text(strconv.Itoa(def.Power))),
		Td(Text(attributeList(def.Attributes))),
	)
}

func summonTable(counts map[string]int, buildingCounts map[string]int) Node {
	return Div(Class("units-section panel"),
		P(Class("panel-title"), Text("Summons")),
		Table(Class("units-table"),
			THead(
				Tr(
					Th(Text("Name")),
					Th(Text("Count")),
					Th(Text("Mana Cost")),
					Th(Text("Mana Upkeep")),
					Th(Text("Power")),
					Th(Text("Attributes")),
				),
			),
			TBody(
				Group(Map(game.SummonOrder, func(utype string) Node {
					def := game.UnitDefs[utype]
					count := counts[utype]
					locked := !game.CanTrain(utype, buildingCounts)
					return summonRow(def, count, locked)
				})),
			),
		),
	)
}

func summonRow(def game.UnitDef, count int, locked bool) Node {
	return Tr(Classes{"unit-row--locked": locked},
		Td(Text(def.Name)),
		Td(Text(strconv.Itoa(count))),
		Td(Text(strconv.Itoa(def.Cost.Mana))),
		Td(Text(strconv.Itoa(def.ManaUpkeep))),
		Td(Text(strconv.Itoa(def.Power))),
		Td(Text(attributeList(def.Attributes))),
	)
}

func trainForm(buildingCounts map[string]int, canSummon bool, busy bool) Node {
	unitNames := game.UnitOrder
	if canSummon {
		unitNames = make([]string, len(game.UnitOrder), len(game.UnitOrder)+len(game.SummonOrder))
		copy(unitNames, game.UnitOrder)
		unitNames = append(unitNames, game.SummonOrder...)
	}

	costs := make(map[string]game.ResourceValues, len(unitNames))
	for _, utype := range unitNames {
		costs[utype] = game.UnitDefs[utype].Cost
	}

	return Div(Class("units-train-form panel"),
		ds.Signals(map[string]any{
			"costs":     costs,
			"unit_type": unitNames[0],
		}, ds.ModifierIfMissing),
		P(Class("panel-title"), Text("Train Units")),
		Div(Class("units-train-fields"),
			Label(For("unit-type-select"), Text("Unit type")),
			Select(
				ID("unit-type-select"),
				ds.Bind("unit_type"),
				Group(Map(unitNames, func(utype string) Node {
					d := game.UnitDefs[utype]
					if !game.CanTrain(utype, buildingCounts) {
						return Option(Value(utype), Disabled(), Text("🔒 "+d.Name))
					}
					return Option(Value(utype), Text(d.Name))
				})),
			),
			Label(For("train-count-input"), Text("Count")),
			Input(
				ID("train-count-input"),
				Type("number"),
				Min("1"),
				Value("1"),
				ds.Bind("train_count"),
			),
			Button(
				Classes{
					"btn":      true,
					"btn-busy": busy,
				},
				If(busy, Disabled()),
				If(!busy, ds.On("click", datastar.PostSSE(routes.KingdomUnitsTrainPath))),
				Text("Train"),
			),
		),
		P(Class("units-train-cost"),
			ds.Computed(
				"wood_cost", "$costs[$unit_type].Wood*$train_count",
				"stone_cost", "$costs[$unit_type].Stone*$train_count",
				"mana_cost", "$costs[$unit_type].Mana*$train_count",
			),
			Text("Cost — Wood: "), Span(ds.Text("$wood_cost")),
			Text("  Stone: "), Span(ds.Text("$stone_cost")),
			Text("  Mana: "), Span(ds.Text("$mana_cost")),
		),
	)
}

func unitsAlert(inner Node) Node { return AlertContainer("units-alert", inner) }

func attributeList(attrs []game.Attribute) string {
	if len(attrs) == 0 {
		return "—"
	}
	strs := make([]string, len(attrs))
	for i, a := range attrs {
		strs[i] = string(a)
	}
	return strings.Join(strs, ", ")
}
