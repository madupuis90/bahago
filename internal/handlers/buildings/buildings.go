package buildings

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

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

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrUnknownBuildingType    = errors.New("unknown building type")
	ErrBuildingNotAvailable   = errors.New("building is not available")
	ErrConstructionInProgress = errors.New("construction already in progress")
	ErrInsufficientResources  = errors.New("not enough resources")
)

// ── Route registration ────────────────────────────────────────────────────────

func RegisterRoutes(r router.Router, queries db.Querier, pool *pgxpool.Pool, tickHub *hub.Hub) {
	h := newHandler(queries, pool, tickHub)
	r.HandleFunc("GET "+routes.KingdomBuildingsPath, h.handleBuildingsPage())
	r.HandleFunc("GET "+routes.KingdomBuildingsRefreshPath, h.handleBuildingsRefresh())
	r.HandleFunc("POST "+routes.KingdomConstructionStartPath, h.handleStartConstruction())
	r.HandleFunc("GET "+routes.KingdomBuildingsDetailPath, h.handleBuildingsDetail())
	r.HandleFunc("POST "+routes.KingdomBuildingsRaisePath, h.handleRaise())
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

func (h *handler) handleBuildingsPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		buildings, err := h.queries.GetKingdomBuildings(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("buildings page: get buildings: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		construction, err := loadConstruction(r.Context(), h.queries, kingdom.ID)
		if err != nil {
			log.Printf("buildings page: get construction: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		counts := game.BuildingCountMap(buildings)
		resources := resourceMap(kingdom)
		KingdomLayout(r, "Buildings", r.URL.Path, kingdom,
			buildingsContent(game.BuildingDefList, counts, construction, resources, "")).Render(w)
	}
}

func (h *handler) handleBuildingsRefresh() http.HandlerFunc {
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
				buildings, err := h.queries.GetKingdomBuildings(r.Context(), k.ID)
				if err != nil {
					log.Printf("buildings refresh: get buildings: %v", err)
					return
				}
				construction, err := loadConstruction(r.Context(), h.queries, k.ID)
				if err != nil {
					log.Printf("buildings refresh: get construction: %v", err)
					sse.PatchElementGostar(buildingsAlert(AlertError(errors.New("internal error"))))
					return
				}
				counts := game.BuildingCountMap(buildings)
				resources := resourceMap(&k)
				page := buildingsContent(game.BuildingDefList, counts, construction, resources, "")
				if err := sse.PatchElementGostar(MainContent(page)); err != nil {
					log.Printf("buildings refresh: patch: %v", err)
					return
				}
			}
		}
	}
}

func (h *handler) handleStartConstruction() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		btype := r.URL.Query().Get("type")
		if err := validateBuildingType(btype); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(buildingsAlert(AlertError(err)))
			return
		}

		if err := h.startConstruction(r.Context(), kingdom.ID, btype); err != nil {
			if isStartConstructionUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(buildingsAlert(AlertError(err)))
				return
			}
			log.Printf("start construction: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(buildingsAlert(AlertError(errors.New("internal error"))))
			return
		}

		k, err := h.queries.GetKingdomByID(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("start construction: reload kingdom: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(buildingsAlert(AlertError(errors.New("internal error"))))
			return
		}
		buildings, err := h.queries.GetKingdomBuildings(r.Context(), k.ID)
		if err != nil {
			log.Printf("start construction: reload buildings: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(buildingsAlert(AlertError(errors.New("internal error"))))
			return
		}
		construction, err := loadConstruction(r.Context(), h.queries, k.ID)
		if err != nil {
			log.Printf("start construction: get construction: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(buildingsAlert(AlertError(errors.New("internal error"))))
			return
		}
		counts := game.BuildingCountMap(buildings)
		resources := resourceMap(&k)
		page := buildingsContent(game.BuildingDefList, counts, construction, resources, btype)
		sse := datastar.NewSSE(w, r)
		if err := sse.PatchElementGostar(MainContent(page)); err != nil {
			log.Printf("start construction: patch: %v", err)
		}
	}
}

func (h *handler) handleBuildingsDetail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		buildingID := r.URL.Query().Get("building")
		if _, ok := game.BuildingDefs[buildingID]; !ok {
			http.Error(w, "unknown building", http.StatusBadRequest)
			return
		}

		buildings, err := h.queries.GetKingdomBuildings(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("buildings detail: get buildings: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		construction, err := loadConstruction(r.Context(), h.queries, kingdom.ID)
		if err != nil {
			log.Printf("buildings detail: get construction: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		counts := game.BuildingCountMap(buildings)
		resources := resourceMap(kingdom)

		sse := datastar.NewSSE(w, r)
		sse.PatchElementGostar(detailPanel(buildingID, game.BuildingDefList, counts, resources, construction))
		sse.MarshalAndPatchSignals(map[string]any{"selected_building": buildingID})
	}
}

func (h *handler) handleRaise() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		btype := r.URL.Query().Get("building")
		if err := validateBuildingType(btype); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(buildingsAlert(AlertError(err)))
			return
		}

		if err := h.startConstruction(r.Context(), kingdom.ID, btype); err != nil {
			if isStartConstructionUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(buildingsAlert(AlertError(err)))
				return
			}
			log.Printf("buildings raise: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(buildingsAlert(AlertError(errors.New("internal error"))))
			return
		}

		k, err := h.queries.GetKingdomByID(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("buildings raise: reload kingdom: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(buildingsAlert(AlertError(errors.New("internal error"))))
			return
		}
		buildings, err := h.queries.GetKingdomBuildings(r.Context(), k.ID)
		if err != nil {
			log.Printf("buildings raise: reload buildings: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(buildingsAlert(AlertError(errors.New("internal error"))))
			return
		}
		construction, err := loadConstruction(r.Context(), h.queries, k.ID)
		if err != nil {
			log.Printf("buildings raise: get construction: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(buildingsAlert(AlertError(errors.New("internal error"))))
			return
		}
		counts := game.BuildingCountMap(buildings)
		resources := resourceMap(&k)
		page := buildingsContent(game.BuildingDefList, counts, construction, resources, btype)
		sse := datastar.NewSSE(w, r)
		if err := sse.PatchElementGostar(MainContent(page)); err != nil {
			log.Printf("buildings raise: patch: %v", err)
		}
	}
}

// ── Validation ────────────────────────────────────────────────────────────────

func validateBuildingType(btype string) error {
	if _, ok := game.BuildingDefs[btype]; !ok {
		return ErrUnknownBuildingType
	}
	return nil
}

// ── Orchestration ─────────────────────────────────────────────────────────────

// startConstruction reads current buildings, checks the prerequisite, then
// opens a SERIALIZABLE transaction to enforce the one-active-construction cap
// and deduct cost. PostgreSQL SSI detects concurrent double-starts; the
// SerializationFailure that fires at commit-time is translated back to
// ErrConstructionInProgress.
func (h *handler) startConstruction(ctx context.Context, kingdomID int, btype string) error {
	def := game.BuildingDefs[btype]

	buildings, err := h.queries.GetKingdomBuildings(ctx, kingdomID)
	if err != nil {
		return fmt.Errorf("get buildings: %w", err)
	}
	if !game.CanBuild(btype, game.BuildingCountMap(buildings)) {
		return ErrBuildingNotAvailable
	}

	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	txq := db.New(tx)

	existing, err := loadConstruction(ctx, txq, kingdomID)
	if err != nil {
		return fmt.Errorf("check existing construction: %w", err)
	}
	if existing != nil {
		return ErrConstructionInProgress
	}

	if _, err := txq.DeductBuildingCost(ctx, db.DeductBuildingCostParams{
		KingdomID:     kingdomID,
		WoodCost:      def.Cost.Wood,
		StoneCost:     def.Cost.Stone,
		FoodCost:      def.Cost.Food,
		ManaCost:      def.Cost.Mana,
		DevotionCost:  def.Cost.Devotion,
		KnowledgeCost: def.Cost.Knowledge,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInsufficientResources
		}
		return fmt.Errorf("deduct cost: %w", err)
	}

	if err := txq.StartConstruction(ctx, db.StartConstructionParams{
		KingdomID:      kingdomID,
		BuildingType:   btype,
		TicksRemaining: def.Ticks,
	}); err != nil {
		return fmt.Errorf("insert construction: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.SerializationFailure {
			return ErrConstructionInProgress
		}
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func isStartConstructionUserError(err error) bool {
	return errors.Is(err, ErrBuildingNotAvailable) ||
		errors.Is(err, ErrConstructionInProgress) ||
		errors.Is(err, ErrInsufficientResources)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func loadConstruction(ctx context.Context, queries db.Querier, kingdomID int) (*db.KingdomConstruction, error) {
	c, err := queries.GetKingdomConstruction(ctx, kingdomID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func resourceMap(k *db.Kingdom) map[string]int {
	return map[string]int{
		"wood":      k.Wood,
		"stone":     k.Stone,
		"food":      k.Food,
		"mana":      k.Mana,
		"devotion":  k.Devotion,
		"knowledge": k.Knowledge,
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func buildingsAlert(inner Node) Node { return AlertContainer("buildings-alert", inner) }

// ── Page content ──────────────────────────────────────────────────────────────

func buildingsContent(defs []game.BuildingDef, counts map[string]int, construction *db.KingdomConstruction, resources map[string]int, selectedID string) Node {
	tree := game.PlaceNodes(defs)
	return Div(Class("builds"),
		ds.Signals(map[string]any{"selected_building": selectedID}),
		Div(Style("display:none"), ds.Init(GetSSENoSignals(routes.KingdomBuildingsRefreshPath))),
		buildingsAlert(nil),
		buildingsBanner(construction),
		PageHeader("Buildings"),
		Div(Class("builds-stage"),
			Div(Class("tree-scroll"),
				buildingsTree(tree, counts, construction),
			),
			detailPanel(selectedID, defs, counts, resources, construction),
		),
	)
}

func buildingsBanner(construction *db.KingdomConstruction) Node {
	if construction == nil {
		return Div(Classes{"progress-banner": true, "is-idle": true},
			Div(Class("progress-banner-body"),
				P(Class("progress-banner-idle-title"), Text("Active Construction")),
				P(Class("progress-banner-idle-text"), Text("No construction underway — select a building to begin.")),
			),
		)
	}
	def := game.BuildingDefs[construction.BuildingType]
	notches := make([]Node, 0, int(construction.TicksTotal))
	for range int(construction.TicksTotal) {
		notches = append(notches, Span(Class("meter-notch")))
	}
	fillPct := 0.0
	if construction.TicksTotal > 0 {
		fillPct = float64(construction.TicksTotal-construction.TicksRemaining) / float64(construction.TicksTotal) * 100
	}
	return Div(Class("progress-banner"),
		Div(Class("progress-banner-gem"), buildingGlyph(def, 34)),
		Div(Class("progress-banner-body"),
			Div(Class("meter"),
				Div(Class("meter-top"),
					Span(Class("meter-name"), Text(def.Name)),
					Span(Class("meter-eta"), Text(fmt.Sprintf("%d / %d ticks remaining", construction.TicksRemaining, construction.TicksTotal))),
				),
				Div(Class("meter-track"),
					Div(Class("meter-fill"), Style(fmt.Sprintf("width:%.1f%%", fillPct))),
					Div(Class("meter-notches"), Group(notches)),
				),
			),
		),
	)
}

// ── Tree ──────────────────────────────────────────────────────────────────────

func buildingsTree(tree game.PlacedTree, counts map[string]int, construction *db.KingdomConstruction) Node {
	style := fmt.Sprintf("width:%dpx;height:%dpx", tree.Width, tree.Height)
	return Div(Classes{"tree-wrap": true, "tree--lineage": true}, Style(style),
		buildingConnectors(tree, counts),
		Group(Map(tree.Nodes, func(pn game.PlacedNode) Node {
			return buildingNode(pn, counts, construction)
		})),
	)
}

func buildingConnectors(tree game.PlacedTree, counts map[string]int) Node {
	var paths []Node
	for _, pn := range tree.Nodes {
		for _, prereq := range pn.Prereqs {
			parent, ok := tree.NodeByID[prereq.Type]
			if !ok {
				continue
			}
			dim := counts[prereq.Type] < prereq.Min
			lit := !dim
			a := game.Point{X: parent.CX, Y: parent.Bottom}
			b := game.Point{X: pn.CX, Y: pn.Top}
			d := game.ElbowPath(a, b)
			mid := (a.Y + b.Y) / 2
			paths = append(paths,
				El("g", Classes{"tree-link": true, "is-lit": lit},
					El("path", Class("tree-link-ink"), Attr("d", d)),
					El("path", Class("tree-link-core"), Attr("d", d)),
				),
				El("circle", Classes{"tree-joint": true, "is-lit": lit},
					Attr("cx", itoa(b.X)), Attr("cy", itoa(mid)), Attr("r", "3")),
			)
		}
	}
	return El("svg", Class("tree-links"), Attr("aria-hidden", "true"),
		Group(paths),
	)
}

func buildingNode(pn game.PlacedNode, counts map[string]int, construction *db.KingdomConstruction) Node {
	count := counts[pn.ID]
	maxed := count >= pn.Max
	prereqMet := game.PrereqMet(pn.BuildingDef, counts)
	locked := !prereqMet && !maxed
	isBuilding := construction != nil && construction.BuildingType == pn.ID
	return Div(
		Classes{"node": true, "is-locked": locked, "is-unlocked": !locked && !maxed, "is-maxed": maxed, "is-building": isBuilding},
		ds.Class("'is-selected'", fmt.Sprintf("$selected_building === '%s'", pn.ID)),
		Style(fmt.Sprintf("left:%dpx;top:%dpx", pn.Left, pn.Top)),
		ds.On("click", datastar.GetSSE(routes.KingdomBuildingsDetailPath+"?building=%s", pn.ID)),
		Div(Class("node-top"),
			buildingGlyph(pn.BuildingDef, 40),
			Iff(locked, func() Node { return Span(Class("node-lock"), lockIcon()) }),
		),
		P(Class("node-name"), Text(pn.Name)),
		nodeCount(count, pn.Max),
		nodePips(count, pn.Max),
		Iff(isBuilding, func() Node { return Div(Class("node-building-ring")) }),
	)
}

func nodePips(count, max int) Node {
	nodes := make([]Node, max+1)
	nodes[0] = Class("pips")
	for i := range max {
		nodes[i+1] = Span(Classes{"pip": true, "on": i < count})
	}
	return El("div", nodes...)
}

func nodeCount(count, max int) Node {
	if count == 0 {
		return P(Class("node-count"), Text(fmt.Sprintf("0 / %d", max)))
	}
	if count >= max {
		return P(Class("node-count"), Span(Class("at-max"), Text(itoa(count))), Text(" / "+itoa(max)))
	}
	return P(Class("node-count"), Span(Class("full"), Text(itoa(count))), Text(" / "+itoa(max)))
}

// ── Detail panel ──────────────────────────────────────────────────────────────

func detailPanel(selectedID string, defs []game.BuildingDef, counts map[string]int, resources map[string]int, construction *db.KingdomConstruction) Node {
	return Div(ID("buildings-detail"), Classes{"card": true, "is-lit": true, "detail": true},
		Div(Class("card-inner"),
			If(selectedID == "", detailEmpty()),
			Iff(selectedID != "", func() Node {
				b := game.BuildingByID(defs, selectedID)
				return detailFull(b, counts, resources, construction)
			}),
		),
	)
}

func detailEmpty() Node {
	return Div(Class("detail-empty"),
		P(Class("detail-empty-title"), Text("Select a Building")),
		P(Class("detail-empty-text"), Text("Click any node in the tree to view its details and construction options.")),
	)
}

func detailFull(b game.BuildingDef, counts map[string]int, resources map[string]int, construction *db.KingdomConstruction) Node {
	count := counts[b.ID]
	return Group([]Node{
		Div(Class("detail-head"),
			buildingGlyph(b, 50),
			Div(
				P(Class("detail-title"), Text(b.Name)),
				P(Class("detail-tally"), Text(fmt.Sprintf("%d / %d raised", count, b.Max))),
			),
		),
		nodePips(count, b.Max),
		P(Class("detail-flavour"), Text(b.Flavour)),
		Div(Class("detail-rule")),
		Div(Class("detail-spec"),
			specRow("Yields", yieldsVal(b, count)),
			specRow("Cost", costVal(b, resources)),
			specRow("Time", Div(Class("spec-val"), Text(fmt.Sprintf("%d ticks", b.Ticks)))),
			If(len(b.Prereqs) > 0, specRow("Requires", reqVal(b, counts))),
		),
		Div(Class("detail-foot"),
			raiseButton(b, counts, resources, construction),
			raiseNote(b, counts, construction),
		),
	})
}

func specRow(label string, val Node) Node {
	return Div(Class("spec-row"),
		Span(Class("spec-label"), Text(label)),
		val,
	)
}

func yieldsVal(b game.BuildingDef, count int) Node {
	if !b.BonusPctPer.HasAny() {
		return Div(Class("spec-val"), Span(Class("bonus-none"), Text("—")))
	}
	bp := b.BonusPctPer
	var lines []Node
	appendBonus := func(pct int, label string) {
		if pct == 0 {
			return
		}
		line := Span(Class("bonus-val"), Text(fmt.Sprintf("+%d%% %s / instance", pct, label)))
		if count > 0 {
			lines = append(lines, Div(line, P(Class("bonus-total"), Text(fmt.Sprintf("+%d%% total", pct*count)))))
		} else {
			lines = append(lines, Div(line))
		}
	}
	appendBonus(bp.Wood, "wood")
	appendBonus(bp.Stone, "stone")
	appendBonus(bp.Food, "food")
	appendBonus(bp.Mana, "mana")
	appendBonus(bp.Devotion, "devotion")
	appendBonus(bp.Knowledge, "knowledge")
	return Div(Class("spec-val"), Group(lines))
}

// costVal renders the build cost as a shared StaticCostPill. The pill is whole-
// red when any resource is short (Fork A: whole-pill affordability).
func costVal(b game.BuildingDef, resources map[string]int) Node {
	return Div(Class("spec-val"),
		StaticCostPill(b.Cost, WithGemSize(22), WithStaticAvailability(resources)),
	)
}

func reqVal(b game.BuildingDef, counts map[string]int) Node {
	items := Map(b.Prereqs, func(p game.Prerequisite) Node {
		met := counts[p.Type] >= p.Min
		name := game.BuildingName(p.Type)
		label := fmt.Sprintf("%s (×%d)", name, p.Min)
		return Div(Classes{"req-item": true, "unmet": !met},
			Span(Classes{"req-mark": true, "met": met, "unmet": !met},
				If(met, checkMarkIcon()),
				If(!met, xMarkIcon()),
			),
			Text(label),
		)
	})
	return Div(Classes{"spec-val": true, "req-list": true}, Group(items))
}

func raiseButton(b game.BuildingDef, counts map[string]int, resources map[string]int, construction *db.KingdomConstruction) Node {
	count := counts[b.ID]
	maxed := count >= b.Max
	prereqMet := game.PrereqMet(b, counts)
	canAfford := game.CanAfford(b, resources)
	buildingThis := construction != nil && construction.BuildingType == b.ID
	anotherInProgress := construction != nil && !buildingThis
	switch {
	case maxed:
		return Button(Classes{"btn": true, "is-locked": true}, Disabled(), Text("Fully Raised"))
	case !prereqMet:
		return Button(Classes{"btn": true, "is-locked": true}, Disabled(), Text("Locked"))
	case anotherInProgress:
		return Button(Class("btn"), Disabled(), Text("Another work underway"))
	case buildingThis:
		return Button(Class("btn"), Disabled(), Text("Raising…"))
	case !canAfford:
		return Button(Classes{"btn": true, "is-insufficient": true}, Disabled(), Text("Not enough resources"))
	default:
		return Button(Classes{"btn": true, "btn--primary": true},
			ds.On("click", datastar.PostSSE(routes.KingdomBuildingsRaisePath+"?building=%s", b.ID)),
			Text("Raise the "+b.Name),
		)
	}
}

func raiseNote(b game.BuildingDef, counts map[string]int, construction *db.KingdomConstruction) Node {
	count := counts[b.ID]
	if count >= b.Max {
		return P(Class("detail-note"), Text("This building is fully raised."))
	}
	if construction != nil && construction.BuildingType == b.ID {
		return P(Class("detail-note"), Text(fmt.Sprintf("%d / %d ticks remaining.", construction.TicksRemaining, construction.TicksTotal)))
	}
	if construction != nil {
		return P(Classes{"detail-note": true, "is-warn": true}, Text("Another construction is already underway."))
	}
	return nil
}

// ── Glyphs & icons ────────────────────────────────────────────────────────────

func buildingGlyph(b game.BuildingDef, size int) Node {
	if b.Resource != "" {
		return ResourceGem(b.Resource, size)
	}
	initial := ""
	if len(b.Name) > 0 {
		initial = string(b.Name[0])
	}
	return El("span", Class("node-medallion"),
		Style(fmt.Sprintf("width:%dpx;height:%dpx", size, size)),
		Span(Class("node-medallion__initial"), Text(initial)),
	)
}

func lockIcon() Node {
	return El("svg", Attr("viewBox", "0 0 10 12"), Attr("aria-hidden", "true"),
		El("path", Attr("fill", "currentColor"),
			Attr("d", "M8 5V4a3 3 0 0 0-6 0v1H1v7h8V5H8zm-5-1a2 2 0 0 1 4 0v1H3V4z")),
	)
}

func checkMarkIcon() Node {
	return El("svg", Attr("viewBox", "0 0 9 9"), Attr("aria-hidden", "true"),
		El("polyline", Attr("points", "1.5,4.5 3.5,6.5 7.5,2.5"),
			Attr("fill", "none"), Attr("stroke", "currentColor"), Attr("stroke-width", "1.5"),
			Attr("stroke-linecap", "round"), Attr("stroke-linejoin", "round")),
	)
}

func xMarkIcon() Node {
	return El("svg", Attr("viewBox", "0 0 9 9"), Attr("aria-hidden", "true"),
		El("line", Attr("x1", "2"), Attr("y1", "2"), Attr("x2", "7"), Attr("y2", "7"),
			Attr("stroke", "currentColor"), Attr("stroke-width", "1.5"), Attr("stroke-linecap", "round")),
		El("line", Attr("x1", "7"), Attr("y1", "2"), Attr("x2", "2"), Attr("y2", "7"),
			Attr("stroke", "currentColor"), Attr("stroke-width", "1.5"), Attr("stroke-linecap", "round")),
	)
}
