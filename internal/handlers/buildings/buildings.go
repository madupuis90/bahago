package buildings

import (
	"errors"
	"fmt"
	"log"
	"net/http"
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

// ── Route registration ────────────────────────────────────────────────────────

func RegisterRoutes(r router.Router, queries db.Querier, pool *pgxpool.Pool, tickHub *hub.Hub) {
	h := newHandler(queries, pool, tickHub)
	r.HandleFunc("GET "+routes.KingdomBuildingsPath, h.handleBuildingsPage())
	r.HandleFunc("GET "+routes.KingdomBuildingsRefreshPath, h.handleBuildingsRefresh())
	r.HandleFunc("POST "+routes.KingdomConstructionStartPath, h.handleStartConstruction())
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

		construction, err := loadConstruction(r, h.queries, kingdom.ID)
		if err != nil {
			log.Printf("buildings page: get construction: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		KingdomLayout(r, "Buildings", r.URL.Path, kingdom, buildingsContent(kingdom, buildings, construction)).Render(w)
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
				construction, err := loadConstruction(r, h.queries, k.ID)
				if err != nil {
					log.Printf("buildings refresh: get construction: %v", err)
					sse.PatchElementGostar(buildingsErrorComponent(errors.New("internal error")))
					return
				}
				page := buildingsContent(&k, buildings, construction)
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
		def, ok := game.BuildingDefs[btype]
		if !ok {
			datastar.NewSSE(w, r).PatchElementGostar(buildingsErrorComponent(errors.New("unknown building type")))
			return
		}

		// Prerequisite check (outside tx — no concurrency concern here).
		buildings, err := h.queries.GetKingdomBuildings(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("start construction: get buildings: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(buildingsErrorComponent(errors.New("internal error")))
			return
		}
		counts := game.BuildingCountMap(buildings)
		if !game.CanBuild(btype, counts) {
			datastar.NewSSE(w, r).PatchElementGostar(buildingsErrorComponent(errors.New("building is not available")))
			return
		}

		// SERIALIZABLE transaction: the construction count read and the insert must
		// share the same transaction so PostgreSQL SSI detects concurrent double-starts.
		tx, err := h.pool.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			log.Printf("start construction: begin tx: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(buildingsErrorComponent(errors.New("internal error")))
			return
		}
		defer tx.Rollback(r.Context()) //nolint:errcheck

		txq := db.New(tx)

		// One active construction at a time — enforced inside the serializable tx so
		// concurrent requests cannot both pass the check simultaneously.
		// A future perk can raise this limit without a schema change.
		existing, err := loadConstruction(r, txq, kingdom.ID)
		if err != nil {
			log.Printf("start construction: get construction: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(buildingsErrorComponent(errors.New("internal error")))
			return
		}
		if existing != nil {
			datastar.NewSSE(w, r).PatchElementGostar(buildingsErrorComponent(errors.New("construction already in progress")))
			return
		}
		_, err = txq.DeductBuildingCost(r.Context(), db.DeductBuildingCostParams{
			KingdomID:     kingdom.ID,
			WoodCost:      def.Cost.Wood,
			StoneCost:     def.Cost.Stone,
			FoodCost:      def.Cost.Food,
			ManaCost:      def.Cost.Mana,
			DevotionCost:  def.Cost.Devotion,
			KnowledgeCost: def.Cost.Knowledge,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			datastar.NewSSE(w, r).PatchElementGostar(buildingsErrorComponent(errors.New("not enough resources")))
			return
		}
		if err != nil {
			log.Printf("start construction: deduct cost: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(buildingsErrorComponent(errors.New("internal error")))
			return
		}

		if err := txq.StartConstruction(r.Context(), db.StartConstructionParams{
			KingdomID:      kingdom.ID,
			BuildingType:   btype,
			TicksRemaining: def.Ticks,
		}); err != nil {
			log.Printf("start construction: insert: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(buildingsErrorComponent(errors.New("internal error")))
			return
		}

		if err := tx.Commit(r.Context()); err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.SerializationFailure {
				datastar.NewSSE(w, r).PatchElementGostar(buildingsErrorComponent(errors.New("construction already in progress")))
				return
			}
			log.Printf("start construction: commit: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(buildingsErrorComponent(errors.New("internal error")))
			return
		}

		// Reload everything from DB — DeductBuildingCost changed the kingdom's resources.
		k, err := h.queries.GetKingdomByID(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("start construction: reload kingdom: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(buildingsErrorComponent(errors.New("internal error")))
			return
		}
		buildings, err = h.queries.GetKingdomBuildings(r.Context(), k.ID)
		if err != nil {
			log.Printf("start construction: reload buildings: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(buildingsErrorComponent(errors.New("internal error")))
			return
		}
		construction, err := loadConstruction(r, h.queries, k.ID)
		if err != nil {
			log.Printf("start construction: get construction: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(buildingsErrorComponent(errors.New("internal error")))
			return
		}
		page := buildingsContent(&k, buildings, construction)
		sse := datastar.NewSSE(w, r)
		if err := sse.PatchElementGostar(MainContent(page)); err != nil {
			log.Printf("start construction: patch: %v", err)
		}
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// startBuild returns the datastar @post action expression for starting construction
// of a specific building type.
func startBuild(btype string) string {
	return datastar.PostSSE(routes.KingdomConstructionStartPath+"?type=%s", btype)
}

// loadConstruction fetches the active construction for a kingdom.
// Returns (nil, nil) if no construction is active, (nil, err) on a real DB error.
func loadConstruction(r *http.Request, queries db.Querier, kingdomID int) (*db.KingdomConstruction, error) {
	c, err := queries.GetKingdomConstruction(r.Context(), kingdomID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ── Components ────────────────────────────────────────────────────────────────

func buildingsContent(kingdom *db.Kingdom, buildings []db.KingdomBuilding, construction *db.KingdomConstruction) Node {
	counts := game.BuildingCountMap(buildings)
	return Div(
		H1(Class("page-title"), Text("Buildings")),
		Div(ds.Init(GetSSENoSignals(routes.KingdomBuildingsRefreshPath))),
		buildingsErrorComponent(nil),
		Iff(construction != nil, func() Node { return activeConstructionBanner(construction) }),
		Div(Class("buildings-grid"),
			buildingCard(kingdom, counts, construction, game.BuildingMill),
			buildingCard(kingdom, counts, construction, game.BuildingQuarry),
			buildingCard(kingdom, counts, construction, game.BuildingFarm),
			buildingCard(kingdom, counts, construction, game.BuildingFactory),
			buildingCard(kingdom, counts, construction, game.BuildingBlacksmith),
			buildingCard(kingdom, counts, construction, game.BuildingGrainerie),
			buildingCard(kingdom, counts, construction, game.BuildingArmory),
		),
	)
}

func activeConstructionBanner(c *db.KingdomConstruction) Node {
	def := game.BuildingDefs[c.BuildingType]
	progress := fmt.Sprintf("%d / %d ticks remaining", c.TicksRemaining, c.TicksTotal)
	return Div(Class("construction-banner panel"),
		Span(Text("Building: "+def.Name)),
		Span(Text(progress)),
	)
}

func buildingCard(kingdom *db.Kingdom, counts map[string]int, construction *db.KingdomConstruction, btype string) Node {
	def := game.BuildingDefs[btype]
	count := counts[btype]
	atMax := count >= def.MaxCount
	canBuild := game.CanBuild(btype, counts)
	unlocked := canBuild || atMax
	locked := !unlocked
	canAfford := kingdom.Wood >= def.Cost.Wood &&
		kingdom.Stone >= def.Cost.Stone &&
		kingdom.Food >= def.Cost.Food &&
		kingdom.Mana >= def.Cost.Mana &&
		kingdom.Devotion >= def.Cost.Devotion &&
		kingdom.Knowledge >= def.Cost.Knowledge
	busy := construction != nil
	buildDisabled := busy || !canBuild || !canAfford

	var btnText string
	switch {
	case locked:
		btnText = "🔒 Locked"
	case !canAfford:
		btnText = "✗ Build"
	default:
		btnText = "Build"
	}

	return Div(Classes{"building-card": true, "panel": true, "building-card--locked": locked},
		P(Class("panel-title"), Text(def.Name)),
		P(Text(fmt.Sprintf("%d / %d", count, def.MaxCount))),
		If(def.BonusPctPer.HasAny(), P(Text(bonusText(def)))),
		If(locked, P(Text(prereqText(def)))),
		If(!atMax, Group([]Node{
			P(Text(costText(def.Cost) + " · " + fmt.Sprintf("%d ticks", def.Ticks))),
			Button(Classes{
				"btn":               true,
				"btn--locked":       locked,
				"btn--insufficient": !locked && !canAfford,
			},
				If(buildDisabled, Disabled()),
				If(!buildDisabled, ds.On("click", startBuild(btype))),
				Text(btnText),
			),
		})),
	)
}

func buildingsErrorComponent(err error) Node {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return Div(ID("buildings-alert"), Text(msg))
}

func bonusText(def game.BuildingDef) string {
	b := def.BonusPctPer
	var parts []string
	if b.Wood != 0 {
		parts = append(parts, fmt.Sprintf("+%d%% wood", b.Wood))
	}
	if b.Stone != 0 {
		parts = append(parts, fmt.Sprintf("+%d%% stone", b.Stone))
	}
	if b.Food != 0 {
		parts = append(parts, fmt.Sprintf("+%d%% food", b.Food))
	}
	if b.Mana != 0 {
		parts = append(parts, fmt.Sprintf("+%d%% mana", b.Mana))
	}
	if b.Devotion != 0 {
		parts = append(parts, fmt.Sprintf("+%d%% devotion", b.Devotion))
	}
	if b.Knowledge != 0 {
		parts = append(parts, fmt.Sprintf("+%d%% knowledge", b.Knowledge))
	}
	return strings.Join(parts, ", ") + " per instance"
}

func prereqText(def game.BuildingDef) string {
	parts := make([]string, 0, len(def.Prerequisites))
	for _, p := range def.Prerequisites {
		d := game.BuildingDefs[p.Type]
		parts = append(parts, d.Name)
	}
	return "Requires: " + strings.Join(parts, " and ")
}

func costText(cost game.ResourceValues) string {
	parts := []string{}
	if cost.Wood > 0 {
		parts = append(parts, fmt.Sprintf("%d wood", cost.Wood))
	}
	if cost.Stone > 0 {
		parts = append(parts, fmt.Sprintf("%d stone", cost.Stone))
	}
	if cost.Food > 0 {
		parts = append(parts, fmt.Sprintf("%d food", cost.Food))
	}
	if cost.Mana > 0 {
		parts = append(parts, fmt.Sprintf("%d mana", cost.Mana))
	}
	if cost.Devotion > 0 {
		parts = append(parts, fmt.Sprintf("%d devotion", cost.Devotion))
	}
	if cost.Knowledge > 0 {
		parts = append(parts, fmt.Sprintf("%d knowledge", cost.Knowledge))
	}
	return strings.Join(parts, ", ")
}
