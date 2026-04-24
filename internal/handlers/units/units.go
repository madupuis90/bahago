package units

import (
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
	. "bahago/internal/layout"
	"bahago/internal/router"
	"bahago/internal/routes"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ── Input struct (for ReadSignals) ────────────────────────────────────────────

type trainInput struct {
	UnitType string `json:"unit_type"`
	Count    int    `json:"train_count"`
}

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

		training, err := loadTraining(r, h.queries, kingdom.ID)
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
				training, err := loadTraining(r, h.queries, k.ID)
				if err != nil {
					log.Printf("units refresh: get training: %v", err)
					sse.PatchElementGostar(unitsErrorComponent(errors.New("internal error")))
					return
				}
				page := KingdomLayout(r, "Units", routes.KingdomUnitsPath, &k, unitsContent(&k, units, buildings, training))
				if err := sse.PatchElementGostar(page, datastar.WithSelector("html")); err != nil {
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
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("invalid request")))
			return
		}

		utype := input.UnitType
		unit, ok := game.UnitDefs[utype]
		if !ok {
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("unknown unit type")))
			return
		}

		count := input.Count
		if count <= 0 {
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("count must be at least 1")))
			return
		}
		if count > game.MaxUnitInput {
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("count is too large")))
			return
		}

		if unit.IsSummon && !game.CanTrainSummons(*kingdom) {
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("summons not unlocked")))
			return
		}

		// Prerequisite check (outside tx — no concurrency concern here).
		buildings, err := h.queries.GetKingdomBuildings(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("train: get buildings: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("internal error")))
			return
		}
		buildingCounts := game.BuildingCountMap(buildings)
		if !game.CanTrain(utype, buildingCounts) {
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("unit not available")))
			return
		}

		// SERIALIZABLE transaction: the training count read and the insert must
		// share the same transaction so PostgreSQL SSI detects concurrent double-starts.
		tx, err := h.pool.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			log.Printf("train: begin tx: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("internal error")))
			return
		}
		defer tx.Rollback(r.Context())

		txq := db.New(tx)

		// One active training order at a time — enforced inside the serializable tx so
		// concurrent requests cannot both pass the check simultaneously.
		// A future perk can raise this limit without a schema change.
		existing, err := loadTraining(r, txq, kingdom.ID)
		if err != nil {
			log.Printf("train: check existing training: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("internal error")))
			return
		}
		if existing != nil {
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("training already in progress")))
			return
		}
		costParams := db.DeductUnitCostParams{
			KingdomID: kingdom.ID,
			WoodCost:  unit.Cost.Wood * count,
			StoneCost: unit.Cost.Stone * count,
			ManaCost:  unit.Cost.Mana * count,
		}
		_, err = txq.DeductUnitCost(r.Context(), costParams)
		if errors.Is(err, pgx.ErrNoRows) {
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("not enough resources")))
			return
		}
		if err != nil {
			log.Printf("train: deduct cost: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("internal error")))
			return
		}

		if err := txq.StartTraining(r.Context(), db.StartTrainingParams{
			KingdomID:      kingdom.ID,
			UnitType:       utype,
			Count:          count,
			TicksRemaining: unit.Ticks,
		}); err != nil {
			log.Printf("train: start training: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("internal error")))
			return
		}

		if err := tx.Commit(r.Context()); err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.SerializationFailure {
				datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("training already in progress")))
				return
			}
			log.Printf("train: commit: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("internal error")))
			return
		}

		// Reload everything from DB — DeductUnitCost changed the kingdom's resources.
		k, err := h.queries.GetKingdomByID(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("train: reload kingdom: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("internal error")))
			return
		}
		allUnits, err := h.queries.GetKingdomUnits(r.Context(), k.ID)
		if err != nil {
			log.Printf("train: reload units: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("internal error")))
			return
		}
		allBuildings, err := h.queries.GetKingdomBuildings(r.Context(), k.ID)
		if err != nil {
			log.Printf("train: reload buildings: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("internal error")))
			return
		}
		training, err := loadTraining(r, h.queries, k.ID)
		if err != nil {
			log.Printf("train: reload training: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsErrorComponent(errors.New("internal error")))
			return
		}

		page := KingdomLayout(r, "Units", routes.KingdomUnitsPath, &k, unitsContent(&k, allUnits, allBuildings, training))
		sse := datastar.NewSSE(w, r)
		if err := sse.PatchElementGostar(page, datastar.WithSelector("html")); err != nil {
			log.Printf("train: patch: %v", err)
		}
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// loadTraining fetches the active training order for a kingdom.
// Returns (nil, nil) if no training is active, (nil, err) on a real DB error.
func loadTraining(r *http.Request, queries db.Querier, kingdomID int) (*db.KingdomTraining, error) {
	t, err := queries.GetKingdomTraining(r.Context(), kingdomID)
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
		Div(ds.Init(datastar.GetSSE(routes.KingdomUnitsRefreshPath))),
		unitsErrorComponent(nil),
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

	costs := make(map[string]game.ResourceCost, len(unitNames))
	for _, utype := range unitNames {
		costs[utype] = game.UnitDefs[utype].Cost
	}

	return Div(Class("units-train-form panel"),
		ds.Signals(map[string]any{
			"costs":     costs,
			"unit_type": unitNames[0],
		}),
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

func unitsErrorComponent(err error) Node {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return Div(ID("units-alert"), Text(msg))
}

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
