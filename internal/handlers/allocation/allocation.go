package allocation

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

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

// ── Input struct ─────────────────────────────────────────────────────────────

type allocationSignals struct {
	IdlePct      int `json:"idle_pct"`
	WoodPct      int `json:"wood_pct"`
	StonePct     int `json:"stone_pct"`
	FoodPct      int `json:"food_pct"`
	ManaPct      int `json:"mana_pct"`
	DevotionPct  int `json:"devotion_pct"`
	KnowledgePct int `json:"knowledge_pct"`
}

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrPercentageOutOfRange = errors.New("allocation values must be between 0 and 100")
	ErrAllocationExceeds100 = errors.New("allocation cannot exceed 100%")
)

// ── Route registration ────────────────────────────────────────────────────────

func RegisterRoutes(r router.Router, queries db.Querier, tickHub *hub.Hub) {
	h := newHandler(queries, tickHub)
	r.HandleFunc("GET "+routes.KingdomAllocationPath, h.handleAllocationPage())
	r.HandleFunc("POST "+routes.KingdomAllocationSavePath, h.handleSaveAllocation())
	r.HandleFunc("GET "+routes.KingdomAllocationRefreshPath, h.handleAllocationRefresh())
}

type handler struct {
	queries db.Querier
	hub     *hub.Hub
}

func newHandler(queries db.Querier, tickHub *hub.Hub) *handler {
	return &handler{queries: queries, hub: tickHub}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handleAllocationPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom, _ := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		buildings, err := h.queries.GetKingdomBuildings(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("allocation page: get buildings: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		prayers, err := h.queries.ListKingdomPrayers(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("allocation page: get prayers: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		targetedPrayers, err := h.queries.ListPrayersTargetingKingdom(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("allocation page: get targeted prayers: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		rates := game.ComputeRates(*kingdom, buildings, targetedPrayers, prayers)
		KingdomLayout(r, "Allocation", r.URL.Path, kingdom, allocationContent(*kingdom, rates)).Render(w)
	}
}

func (h *handler) handleAllocationRefresh() http.HandlerFunc {
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
					log.Printf("allocation refresh: get buildings: %v", err)
					sse.PatchElementGostar(allocationAlert(AlertError(errors.New("internal error"))))
					return
				}
				prayers, err := h.queries.ListKingdomPrayers(r.Context(), k.ID)
				if err != nil {
					log.Printf("allocation refresh: get prayers: %v", err)
					sse.PatchElementGostar(allocationAlert(AlertError(errors.New("internal error"))))
					return
				}
				targetedPrayers, err := h.queries.ListPrayersTargetingKingdom(r.Context(), k.ID)
				if err != nil {
					log.Printf("allocation refresh: get targeted prayers: %v", err)
					sse.PatchElementGostar(allocationAlert(AlertError(errors.New("internal error"))))
					return
				}
				rates := game.ComputeRates(k, buildings, targetedPrayers, prayers)
				page := allocationContent(k, rates)
				if err := sse.PatchElementGostar(MainContent(page)); err != nil {
					log.Printf("allocation refresh: patch: %v", err)
					return
				}
			}
		}
	}
}

func (h *handler) handleSaveAllocation() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _ := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser)

		input := &allocationSignals{}
		if err := datastar.ReadSignals(r, input); err != nil {
			log.Printf("save-allocation: read signals: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(allocationAlert(AlertError(errors.New("invalid request"))))
			return
		}

		if errs := validateAllocationInput(input); len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(allocationAlert(AlertError(errs...)))
			return
		}

		updatedKingdom, err := h.updateAllocations(r.Context(), user.ID, input)
		if err != nil {
			log.Printf("save-allocation: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(allocationAlert(AlertError(errors.New("failed to save allocation"))))
			return
		}

		buildings, err := h.queries.GetKingdomBuildings(r.Context(), updatedKingdom.ID)
		if err != nil {
			log.Printf("save-allocation: get buildings: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(allocationAlert(AlertError(errors.New("internal error"))))
			return
		}
		prayers, err := h.queries.ListKingdomPrayers(r.Context(), updatedKingdom.ID)
		if err != nil {
			log.Printf("save-allocation: get prayers: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(allocationAlert(AlertError(errors.New("internal error"))))
			return
		}
		targetedPrayers, err := h.queries.ListPrayersTargetingKingdom(r.Context(), updatedKingdom.ID)
		if err != nil {
			log.Printf("save-allocation: get targeted prayers: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(allocationAlert(AlertError(errors.New("internal error"))))
			return
		}
		rates := game.ComputeRates(updatedKingdom, buildings, targetedPrayers, prayers)
		page := allocationContent(updatedKingdom, rates)
		sse := datastar.NewSSE(w, r)
		if err := sse.PatchElementGostar(MainContent(page)); err != nil {
			log.Printf("save-allocation: patch: %v", err)
		}
	}
}

// ── Validation ────────────────────────────────────────────────────────────────

// validateAllocationInput runs the bounds and sum checks on the six resource
// percentages. Accumulates errors so the handler can show all problems at once.
// IdlePct is not validated — it is derived from 100 minus the others.
func validateAllocationInput(input *allocationSignals) []error {
	var errs []error
	values := []int{
		input.WoodPct, input.StonePct, input.FoodPct,
		input.ManaPct, input.DevotionPct, input.KnowledgePct,
	}
	for _, v := range values {
		if v < 0 || v > 100 {
			errs = append(errs, ErrPercentageOutOfRange)
			break // one violation is enough to communicate the rule
		}
	}
	total := input.WoodPct + input.StonePct + input.FoodPct +
		input.ManaPct + input.DevotionPct + input.KnowledgePct
	if total > 100 {
		errs = append(errs, ErrAllocationExceeds100)
	}
	return errs
}

// ── Orchestration ─────────────────────────────────────────────────────────────

// updateAllocations writes the new allocation percentages, computing idle as
// 100 minus the sum of the six resource slices. Returns the updated kingdom.
func (h *handler) updateAllocations(ctx context.Context, userID int, input *allocationSignals) (db.Kingdom, error) {
	total := input.WoodPct + input.StonePct + input.FoodPct +
		input.ManaPct + input.DevotionPct + input.KnowledgePct

	updated, err := h.queries.UpdateKingdomAllocations(ctx, db.UpdateKingdomAllocationsParams{
		UserID:       userID,
		WoodPct:      input.WoodPct,
		StonePct:     input.StonePct,
		FoodPct:      input.FoodPct,
		ManaPct:      input.ManaPct,
		DevotionPct:  input.DevotionPct,
		KnowledgePct: input.KnowledgePct,
		IdlePct:      100 - total,
	})
	if err != nil {
		return db.Kingdom{}, fmt.Errorf("update allocations: %w", err)
	}
	return updated, nil
}

// ── Page ──────────────────────────────────────────────────────────────────────

func allocationContent(kingdom db.Kingdom, rates game.ResourceRates) Node {
	totalExpr := "$wood_pct + $stone_pct + $food_pct + $mana_pct + $devotion_pct + $knowledge_pct"
	dirtyExpr := "$wood_pct !== $wood_saved || $stone_pct !== $stone_saved || $food_pct !== $food_saved || $mana_pct !== $mana_saved || $devotion_pct !== $devotion_saved || $knowledge_pct !== $knowledge_saved"
	return Div(
		ds.Signals(map[string]any{
			"wood_saved":      kingdom.WoodPct,
			"stone_saved":     kingdom.StonePct,
			"food_saved":      kingdom.FoodPct,
			"mana_saved":      kingdom.ManaPct,
			"devotion_saved":  kingdom.DevotionPct,
			"knowledge_saved": kingdom.KnowledgePct,
		}),
		ds.Signals(map[string]any{
			"wood_pct":      kingdom.WoodPct,
			"stone_pct":     kingdom.StonePct,
			"food_pct":      kingdom.FoodPct,
			"mana_pct":      kingdom.ManaPct,
			"devotion_pct":  kingdom.DevotionPct,
			"knowledge_pct": kingdom.KnowledgePct,
		}, ds.ModifierIfMissing),
		ds.Computed("alloc_total", totalExpr, "idle_pct", "100-("+totalExpr+")", "alloc_dirty", dirtyExpr),
		Div(ds.Init(GetSSENoSignals(routes.KingdomAllocationRefreshPath))),
		PageHeader("Allocation"),
		Div(Class("card"),
			Div(Class("card-inner"),
				allocationBar(),
				allocationLegend(),
				Div(Class("alloc-rule")),
				Div(Class("alloc-grid"),
					allocationHead(),
					allocationRow("tree", "Woodcutter", "wood_pct", "wood_saved", "Wood", kingdom.WoodPct, rates.WoodProduction-rates.WoodUpkeep),
					allocationRow("mountain", "Miner", "stone_pct", "stone_saved", "Stone", kingdom.StonePct, rates.StoneProduction-rates.StoneUpkeep),
					allocationRow("wheat", "Farmer", "food_pct", "food_saved", "Food", kingdom.FoodPct, rates.FoodProduction-rates.FoodUpkeep),
					allocationRow("flame", "Disciple", "mana_pct", "mana_saved", "Mana", kingdom.ManaPct, rates.ManaProduction-rates.ManaUpkeep),
					allocationRow("sun", "Clergy", "devotion_pct", "devotion_saved", "Devotion", kingdom.DevotionPct, rates.DevotionProduction-rates.DevotionUpkeep),
					allocationRow("star", "Scholar", "knowledge_pct", "knowledge_saved", "Lore", kingdom.KnowledgePct, rates.KnowledgeProduction-rates.KnowledgeUpkeep),
					idleRow(rates.PopulationProduction-rates.PopulationUpkeep),
				),
				Div(Class("alloc-foot"),
					Div(Class("alloc-total"),
						Span(Class("alloc-total-label"), Text("Allocated")),
						Span(
							Classes{"alloc-total-val": true},
							ds.Class("over", "$alloc_total > 100"),
							ds.Text("$alloc_total+'%'"),
						),
					),
					Div(Class("alloc-section"),
						Div(Class("alloc-error"),
							ds.Show("$alloc_total > 100"),
							Span(Class("alloc-alarm"), Text("⚠ Too many hands!")),
						),
						allocationAlert(nil),
						Button(
							Class("btn btn--primary"),
							Type("button"),
							ds.Class("'is-locked'", "$alloc_total > 100"),
							ds.Attr("disabled", "!$alloc_dirty || $alloc_total > 100"),
							ds.On("click", datastar.PostSSE(routes.KingdomAllocationSavePath)),
							Text("Allocate"),
						),
					),
				),
			),
		),
	)
}

func allocationAlert(inner Node) Node { return AlertContainer("allocation-alert", inner) }

func allocationBar() Node {
	return Div(Class("allocation-bar"),
		Span(Class("allocation-wood"), ds.Style("width", "$wood_pct+'%'")),
		Span(Class("allocation-stone"), ds.Style("width", "$stone_pct+'%'")),
		Span(Class("allocation-food"), ds.Style("width", "$food_pct+'%'")),
		Span(Class("allocation-mana"), ds.Style("width", "$mana_pct+'%'")),
		Span(Class("allocation-devotion"), ds.Style("width", "$devotion_pct+'%'")),
		Span(Class("allocation-knowledge"), ds.Style("width", "$knowledge_pct+'%'")),
		Span(Class("allocation-idle"), ds.Style("width", "$idle_pct+'%'")),
	)
}

func allocationLegend() Node {
	return Div(Class("allocation-legend"),
		allocationKey("allocation-wood", "Woodcutter"),
		allocationKey("allocation-stone", "Miner"),
		allocationKey("allocation-food", "Farmer"),
		allocationKey("allocation-mana", "Disciple"),
		allocationKey("allocation-devotion", "Clergy"),
		allocationKey("allocation-knowledge", "Scholar"),
		allocationKey("allocation-idle", "Idle"),
	)
}

func allocationKey(colorClass, label string) Node {
	return Span(Class("allocation-key"),
		Span(Class("allocation-dot "+colorClass)),
		Text(label),
	)
}

func allocationHead() Node {
	return Div(Class("alloc-head"),
		Div(),
		Div(Class("alloc-col-header"), Text("Craft")),
		Div(Class("alloc-col-header"), Text("Allocation")),
		Div(Class("alloc-col-header alloc-col-header--right"), Text("Share")),
		Div(Class("alloc-col-header alloc-col-header--right"), Text("Net/tick")),
	)
}

func allocationRow(gemID, roleName, key, savedKey, resourceLabel string, initialValue, net int) Node {
	pendingExpr := fmt.Sprintf("$%s !== $%s", key, savedKey)
	netClass := "alloc-net"
	if net < 0 {
		netClass += " neg"
	} else if net == 0 {
		netClass += " zero"
	}
	rowClass := "alloc-row alloc-row--" + strings.TrimSuffix(key, "_pct")
	return Div(Class(rowClass),
		ResourceGem(gemID, 36),
		Div(
			Div(Class("alloc-role-name"), Text(roleName)),
			Div(Class("alloc-role-res"), Text(resourceLabel)),
		),
		Div(Class("slider-controls v-diamond"),
			Button(Type("button"), Class("btn btn--sm"), ds.On("click", fmt.Sprintf("$%s = Math.max(0, $%s - 5)", key, key)), Text("−5")),
			Button(Type("button"), Class("btn btn--sm"), ds.On("click", fmt.Sprintf("$%s = Math.max(0, $%s - 1)", key, key)), Text("−")),
			Div(Class("slider-wrap"),
				Div(Class("slider-track"),
					Div(Class("slider-fill"), ds.Style("width", fmt.Sprintf("$%s+'%%'", key))),
					Div(Class("slider-ticks")),
				),
				Div(Class("slider-thumb"), ds.Style("left", fmt.Sprintf("$%s+'%%'", key))),
				Input(Class("slider-input"), Type("range"), Min("0"), Max("100"),
					Value(fmt.Sprintf("%d", initialValue)),
					ds.Bind(key),
				),
			),
			Button(Type("button"), Class("btn btn--sm"), ds.On("click", fmt.Sprintf("$%s = Math.min(100, $%s + 1)", key, key)), Text("+")),
			Button(Type("button"), Class("btn btn--sm"), ds.On("click", fmt.Sprintf("$%s = Math.min(100, $%s + 5)", key, key)), Text("+5")),
		),
		Div(Class("alloc-share"), ds.Text(fmt.Sprintf("$%s+'%%'", key))),
		Div(
			Class("alloc-net-cell"),
			ds.Class("'is-pending'", pendingExpr),
			Div(Class(netClass), Text(fmt.Sprintf("%+d", net))),
			Div(
				Class("alloc-rate"),
				ds.Class("'is-pending'", pendingExpr),
				ds.Text(fmt.Sprintf("$%s !== $%s ? 'pending save' : '%s/tick'", key, savedKey, resourceLabel)),
			),
		),
	)
}

func idleRow(net int) Node {
	netClass := "alloc-net"
	if net <= 0 {
		netClass += " zero"
	}
	netText := "—"
	if net > 0 {
		netText = fmt.Sprintf("+%d", net)
	}
	rateLabel := "no growth"
	if net > 0 {
		rateLabel = "pop / tick"
	}
	return Div(Class("alloc-row idle"),
		Div(Class("idle-gem"), Icon("idle", 36, false)),
		Div(
			Div(Class("alloc-role-name"), Text("Idle")),
		),
		P(Class("alloc-assign-note"), Text("An untasked realm breeds faster — idle hands speed your growth.")),
		Div(Class("alloc-share"), ds.Text("$idle_pct+'%'")),
		Div(
			Class("alloc-net-cell"),
			ds.Class("'is-pending'", "$alloc_dirty"),
			Div(Class(netClass), Text(netText)),
			Div(
				Class("alloc-rate"),
				ds.Class("'is-pending'", "$alloc_dirty"),
				ds.Text(fmt.Sprintf("$alloc_dirty ? 'pending save' : '%s'", rateLabel)),
			),
		),
	)
}
