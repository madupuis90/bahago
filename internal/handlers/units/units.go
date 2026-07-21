package units

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strconv"

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
	r.HandleFunc("POST "+routes.KingdomUnitsCancelPath, h.handleCancel())
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

		buildings, err := h.queries.GetKingdomBuildings(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("train: get buildings: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsAlert(AlertError(errors.New("internal error"))))
			return
		}

		if err := h.trainUnits(r.Context(), kingdom, input, buildings); err != nil {
			if isTrainUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(unitsAlert(AlertError(err)))
				return
			}
			log.Printf("train: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsAlert(AlertError(errors.New("internal error"))))
			return
		}

		// Reload kingdom (DeductUnitCost changed resources) and units.
		// Buildings are unchanged; reuse the pre-fetched slice.
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

		// Construct training state from known input — avoids a read-after-write
		// that would race the training tick.
		unit := game.UnitDefs[input.UnitType]
		training := &db.KingdomTraining{
			UnitType:       input.UnitType,
			Count:          input.Count,
			TicksRemaining: unit.Ticks,
			TicksTotal:     unit.Ticks,
		}

		page := unitsContent(&k, allUnits, buildings, training)
		h.hub.Publish(k)
		sse := datastar.NewSSE(w, r)
		if err := sse.PatchElementGostar(MainContent(page)); err != nil {
			log.Printf("train: patch: %v", err)
		}
	}
}

func (h *handler) handleCancel() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		training, err := loadTraining(r.Context(), h.queries, kingdom.ID)
		if err != nil {
			log.Printf("cancel: load training: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsAlert(AlertError(errors.New("internal error"))))
			return
		}

		if training == nil {
			datastar.NewSSE(w, r)
			return
		}

		unit, ok := game.UnitDefs[training.UnitType]
		if !ok {
			log.Printf("cancel: unit type %q removed from defs while training was in progress; cancelling without refund", training.UnitType)
		}
		params := db.CancelTrainingWithRefundParams{
			KingdomID:   kingdom.ID,
			TrainingID:  training.ID,
			WoodRefund:  unit.Cost.Wood * training.Count,
			StoneRefund: unit.Cost.Stone * training.Count,
			ManaRefund:  unit.Cost.Mana * training.Count,
		}
		if err := h.queries.CancelTrainingWithRefund(r.Context(), params); err != nil {
			log.Printf("cancel training: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsAlert(AlertError(errors.New("internal error"))))
			return
		}

		k, err := h.queries.GetKingdomByID(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("cancel: reload kingdom: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsAlert(AlertError(errors.New("internal error"))))
			return
		}
		allUnits, err := h.queries.GetKingdomUnits(r.Context(), k.ID)
		if err != nil {
			log.Printf("cancel: reload units: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsAlert(AlertError(errors.New("internal error"))))
			return
		}
		allBuildings, err := h.queries.GetKingdomBuildings(r.Context(), k.ID)
		if err != nil {
			log.Printf("cancel: reload buildings: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(unitsAlert(AlertError(errors.New("internal error"))))
			return
		}

		h.hub.Publish(k)

		sse := datastar.NewSSE(w, r)
		if err := sse.PatchElementGostar(MainContent(unitsContent(&k, allUnits, allBuildings, nil))); err != nil {
			log.Printf("cancel: patch: %v", err)
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
// SERIALIZABLE transaction to enforce the concurrent-training cap and deduct
// cost. PostgreSQL SSI detects concurrent double-starts; the
// SerializationFailure that fires at commit-time is translated back to
// ErrTrainingInProgress.
//
// There is deliberately NO unique constraint on kingdom_training.kingdom_id:
// skills will raise the concurrent-training cap beyond 1, so the cap must stay
// application-enforced. Do not "fix" this with a schema constraint. When the
// cap rises above 1, this existence check becomes a count check — and the
// single-row assumptions in the tick (DeleteTraining by kingdom_id) and
// GetKingdomTraining (:one) must be revisited at the same time.
func (h *handler) trainUnits(ctx context.Context, kingdom *db.Kingdom, input *trainInput, buildings []db.KingdomBuilding) error {
	unit := game.UnitDefs[input.UnitType]

	if unit.IsSummon && !game.CanTrainSummons(*kingdom) {
		return ErrSummonsNotUnlocked
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

	allTypes := slices.Concat(game.UnitOrder, game.SummonOrder)
	signals := make(map[string]any, len(allTypes))
	for _, utype := range allTypes {
		signals["count_"+utype] = 1
	}

	return Div(
		ds.Signals(signals, ds.ModifierIfMissing),
		Div(ds.Init(GetSSENoSignals(routes.KingdomUnitsRefreshPath))),
		unitsAlert(nil),
		trainingGrounds(training),
		PageHeader("Units"),
		Div(Class("host-cards"),
			musterRoll("", "", game.UnitOrder, counts, buildingCounts, training, false),
			musterRoll("Summons", "card-tab--summon", game.SummonOrder, counts, buildingCounts, training, !canSummon),
		),
		hostSummary(counts, allTypes),
	)
}

func hostSummary(counts map[string]int, allTypes []string) Node {
	totalUnits, totalFood, totalMana := 0, 0, 0
	for _, utype := range allTypes {
		def := game.UnitDefs[utype]
		n := counts[utype]
		totalUnits += n
		totalFood += n * def.FoodUpkeep
		totalMana += n * def.ManaUpkeep
	}
	return Div(Class("units-summary is-foot"),
		hostStat("Standing", "units", totalUnits, false),
		hostStat("Food Drain", "per tick", totalFood, totalFood > 0),
		hostStat("Mana Drain", "per tick", totalMana, totalMana > 0),
	)
}

func hostStat(label, sub string, num int, drain bool) Node {
	return Div(Class("units-summary-stat"),
		Span(Class("units-summary-label"), Text(label)),
		Div(Class("units-summary-val"),
			Span(Classes{"units-summary-num": true, "neg": drain}, Text(strconv.Itoa(num))),
			Span(Class("units-summary-sub"), Text(sub)),
		),
	)
}

func trainingGrounds(training *db.KingdomTraining) Node {
	if training == nil {
		return Div(Classes{"progress-banner": true, "is-idle": true},
			Div(Class("progress-banner-body"),
				P(Class("progress-banner-idle-title"), Text("Active Training")),
				P(Class("progress-banner-idle-text"), Text("No training in progress")),
			),
		)
	}
	def := game.UnitDefs[training.UnitType]
	fillPct := 0.0
	if training.TicksTotal > 0 {
		fillPct = float64(training.TicksTotal-training.TicksRemaining) / float64(training.TicksTotal) * 100
	}
	notches := make([]Node, 0, int(training.TicksTotal))
	for range int(training.TicksTotal) {
		notches = append(notches, Span(Class("meter-notch")))
	}
	return Div(Class("progress-banner"),
		Div(Class("progress-banner-gem "+portraitClass(def)),
			Div(Class("unit-portrait unit-portrait--sm "+portraitMod(def))),
		),
		Div(Class("progress-banner-body"),
			Div(Class("meter"),
				Div(Class("meter-top"),
					Span(Class("meter-name"),
						Text(def.Name+" "),
						Span(Class("meter-qty"), Text(fmt.Sprintf("× %d", training.Count))),
					),
					Span(Class("meter-eta"), Text(fmt.Sprintf("%d ticks", training.TicksRemaining))),
				),
				Div(Class("meter-track"),
					Div(Class("meter-fill"), Style(fmt.Sprintf("width:%.1f%%", fillPct))),
					Div(Class("meter-notches"), Group(notches)),
				),
			),
		),
		Button(Class("btn"), ds.On("click", datastar.PostSSE(routes.KingdomUnitsCancelPath)), Text("Cancel")),
	)
}

// portraitClass returns a modifier class for the gem wrapper so the summon vs
// unit portrait tint still applies inside the progress banner.
func portraitClass(def game.UnitDef) string {
	if def.IsSummon {
		return "progress-banner-gem--summon"
	}
	return "progress-banner-gem--unit"
}

func portraitMod(def game.UnitDef) string {
	if def.IsSummon {
		return "unit-portrait--summon"
	}
	return "unit-portrait--unit"
}

func musterRoll(title, tabClass string, order []string, counts, buildingCounts map[string]int, training *db.KingdomTraining, sectionLocked bool) Node {
	return Div(Class("card"),
		If(title != "", Div(Class("card-tab "+tabClass), Text(title))),
		Div(Class("card-inner"),
			If(sectionLocked, lockBanner()),
			Div(Class("muster-roll-head"),
				Div(),
				Div(Class("muster-roll-col"), Text("Name")),
				Div(Class("muster-roll-col muster-roll-col--r"), Text("Power")),
				Div(Class("muster-roll-col muster-roll-col--r"), Text("Upkeep")),
				Div(Class("muster-roll-col muster-roll-col--r"), Text("Cost")),
				Div(Class("muster-roll-col muster-roll-col--r"), Text("Train")),
			),
			Group(Map(order, func(utype string) Node {
				def := game.UnitDefs[utype]
				count := counts[utype]
				locked := !game.CanTrain(utype, buildingCounts)
				return unitRow(utype, def, count, locked, sectionLocked, training)
			})),
		),
	)
}

func unitRow(utype string, def game.UnitDef, count int, locked, manaLocked bool, training *db.KingdomTraining) Node {
	return Div(Classes{"unit-row": true, "is-locked": locked || manaLocked},
		ds.Computed(
			"cost_wood_"+utype, fmt.Sprintf("$count_%s * %d", utype, def.Cost.Wood),
			"cost_stone_"+utype, fmt.Sprintf("$count_%s * %d", utype, def.Cost.Stone),
			"cost_mana_"+utype, fmt.Sprintf("$count_%s * %d", utype, def.Cost.Mana),
		),
		Div(Class("unit-token"),
			Div(Class("unit-portrait unit-portrait--sm "+portraitMod(def))),
			Span(Class("unit-tally"), Text(strconv.Itoa(count))),
		),
		Div(
			Span(Class("unit-name"), Text(def.Name)),
			If(len(def.Attributes) > 0,
				Div(Class("attribute-row"),
					Group(Map(def.Attributes, func(a game.Attribute) Node {
						return Span(Class("attribute "+attrClass(a)), Text(string(a)))
					})),
				),
			),
		),
		Span(Class("stat-pill"),
			Text(strconv.Itoa(def.Power)),
		),
		upkeepPill(def),
		trainCostCell(utype, def),
		trainControl(utype, locked, manaLocked, training),
	)
}

func trainCostCell(utype string, def game.UnitDef) Node {
	entries := make([]DynamicCostEntry, 0, 3)
	if def.Cost.Wood > 0 {
		entries = append(entries, DynamicCostEntry{Resource: "wood", Expr: "$cost_wood_" + utype})
	}
	if def.Cost.Stone > 0 {
		entries = append(entries, DynamicCostEntry{Resource: "stone", Expr: "$cost_stone_" + utype})
	}
	if def.Cost.Mana > 0 {
		entries = append(entries, DynamicCostEntry{Resource: "mana", Expr: "$cost_mana_" + utype})
	}
	if len(entries) == 0 {
		return Div()
	}
	// WithSignalAvailability compares each $cost_* expression against the
	// layout-scoped $<resource> signals, so the pill goes whole-red when the
	// user can't afford the current training count.
	return Div(Class("unit-cost"),
		DynamicCostPill(entries, WithGemSize(17), WithSignalAvailability()),
	)
}

func trainControl(utype string, locked, manaLocked bool, training *db.KingdomTraining) Node {
	if manaLocked {
		return Div(Class("unit-train"),
			Span(Class("unit-lock-note"),
				ResourceGem("flame", 16),
				Text("Needs mana"),
			),
		)
	}
	if locked {
		return Div(Class("unit-train"),
			Span(Class("unit-lock-note"), Text("Building required")),
		)
	}
	if training != nil {
		return Div(Class("unit-train"),
			Div(Class("train-ctl"),
				Button(Class("btn"), Disabled(), Text("Train")),
			),
		)
	}
	trainExpr := fmt.Sprintf("$unit_type='%s';$train_count=$count_%s;%s",
		utype, utype, datastar.PostSSE(routes.KingdomUnitsTrainPath))
	countKey := "count_" + utype
	decExpr := fmt.Sprintf("$%s = Math.max(1, ($%s || 1) - 1)", countKey, countKey)
	incExpr := fmt.Sprintf("$%s = ($%s || 1) + 1", countKey, countKey)
	return Div(Class("unit-train"),
		Div(Class("train-ctl"),
			Div(Class("stepper"),
				Span(Class("step-btn step-btn--l"), Role("button"), ds.On("click", decExpr), Text("−")),
				Input(Class("count-box"), Attr("inputmode", "numeric"), Type("text"), ds.Bind(countKey)),
				Span(Class("step-btn step-btn--r"), Role("button"), ds.On("click", incExpr), Text("+")),
			),
			Button(Class("btn btn--sm btn--primary"), ds.On("click", trainExpr), Text("Train")),
		),
	)
}

func lockBanner() Node {
	return Div(Class("lock-banner"),
		ResourceGem("flame", 34),
		P(Text("Summoning is sealed — establish "),
			El("b", Text("mana production")),
			Text(" to channel the aether."),
		),
	)
}



func upkeepPill(def game.UnitDef) Node {
	if def.FoodUpkeep > 0 {
		return Span(Classes{"stat-pill": true, "stat-pill--drain": true},
			ResourceGem("wheat", 18),
			Span(Class("pill-neg"), Text(strconv.Itoa(def.FoodUpkeep))),
		)
	}
	if def.ManaUpkeep > 0 {
		return Span(Classes{"stat-pill": true, "stat-pill--drain": true},
			ResourceGem("flame", 18),
			Span(Class("pill-neg"), Text(strconv.Itoa(def.ManaUpkeep))),
		)
	}
	return Span(Class("unit-upkeep--none"), Text("none"))
}

func unitsAlert(inner Node) Node { return AlertContainer("units-alert", inner) }

func attrClass(a game.Attribute) string {
	switch a {
	case game.AttributeMelee, game.AttributeArcher, game.AttributeRaiders,
		game.AttributeFlying, game.AttributeSiegeEngine, game.AttributeEnrage:
		return "attribute--offense"
	case game.AttributeShields:
		return "attribute--defense"
	case game.AttributeUndead, game.AttributeDeathtouch, game.AttributeSummon:
		return "attribute--arcane"
	case game.AttributeWorshipper:
		return "attribute--faith"
	case game.AttributeGluttony, game.AttributePacifism:
		return "attribute--drawback"
	default:
		log.Printf("attrClass: unrecognised attribute %q", a)
		return ""
	}
}
