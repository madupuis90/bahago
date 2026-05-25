package allocation

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

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
	return Div(
		Div(ds.Init(GetSSENoSignals(routes.KingdomAllocationRefreshPath))),
		ds.Signals(map[string]any{
			"idle_pct":      kingdom.IdlePct,
			"wood_pct":      kingdom.WoodPct,
			"stone_pct":     kingdom.StonePct,
			"food_pct":      kingdom.FoodPct,
			"mana_pct":      kingdom.ManaPct,
			"devotion_pct":  kingdom.DevotionPct,
			"knowledge_pct": kingdom.KnowledgePct,
		}, ds.ModifierIfMissing),
		PageHeader("", "Population Allocation"),
		Div(Class("panel allocation-table"),
			allocationHead(),
			allocationRow("chevron", "Woodcutter", "wood_pct", "Wood", kingdom.WoodPct, rates.WoodProduction, rates.WoodUpkeep),
			allocationRow("mountain", "Miner", "stone_pct", "Stone", kingdom.StonePct, rates.StoneProduction, rates.StoneUpkeep),
			allocationRow("wheat", "Farmer", "food_pct", "Food", kingdom.FoodPct, rates.FoodProduction, rates.FoodUpkeep),
			allocationRow("sun", "Clergy", "devotion_pct", "Devotion", kingdom.DevotionPct, rates.DevotionProduction, rates.DevotionUpkeep),
			allocationRow("moon", "Disciple", "mana_pct", "Mana", kingdom.ManaPct, rates.ManaProduction, rates.ManaUpkeep),
			allocationRow("book", "Scholar", "knowledge_pct", "Knowledge", kingdom.KnowledgePct, rates.KnowledgeProduction, rates.KnowledgeUpkeep),
			idleRow(kingdom.IdlePct, rates.PopulationProduction, rates.PopulationUpkeep),
		),
		Div(Class("allocation-footer"),
			P(Class("allocation-hint text-muted"), Text("✦ Unassigned population encourages the growth of population.")),
			Button(Class("btn btn--primary"),
				ds.On("click", datastar.PostSSE(routes.KingdomAllocationSavePath)),
				Text("Assign"),
			),
		),
		allocationAlert(nil),
	)
}

func allocationAlert(inner Node) Node { return AlertContainer("allocation-alert", inner) }

func allocationHead() Node {
	return Div(Class("allocation-head"),
		Div(),
		Div(Class("caps-label"), Text("Role")),
		Div(Class("caps-label"), Text("Assignment")),
		Div(Class("caps-label"), Text("Percentage")),
		Div(Class("caps-label"), Text("Production")),
		Div(Class("caps-label"), Text("Upkeep")),
		Div(Class("caps-label"), Text("Total")),
		Div(Class("caps-label"), Text("Resource")),
	)
}

func allocationRow(shieldID, roleName, key, resourceLabel string, initialValue, production, upkeep int) Node {
	ref := "$" + key
	net := production - upkeep
	return Div(Class("allocation-row"),
		Div(Class("allocation-shield"), Shield(shieldID, 22, false)),
		Div(Class("allocation-role"), Text(roleName)),
		Div(Class("allocation-assign"),
			Button(Class("btn btn--sm"), ds.On("click", fmt.Sprintf("%s = Math.max(0, %s - 5)", ref, ref)), Text("−5")),
			Button(Class("btn btn--sm"), ds.On("click", fmt.Sprintf("%s = Math.max(0, %s - 1)", ref, ref)), Text("−")),
			Input(Class("allocation-slider"), Type("range"), Min("0"), Max("100"), Value(fmt.Sprintf("%d", initialValue)),
				ds.Bind(key),
			),
			Button(Class("btn btn--sm"), ds.On("click", fmt.Sprintf("%s = Math.min(100, %s + 1)", ref, ref)), Text("+")),
			Button(Class("btn btn--sm"), ds.On("click", fmt.Sprintf("%s = Math.min(100, %s + 5)", ref, ref)), Text("+5")),
		),
		Div(Class("allocation-pct"),
			Span(ds.Text(ref), Text(fmt.Sprintf("%d", initialValue))),
			Span(Text("%")),
		),
		Div(Classes{"allocation-num": true, "allocation-num--pos": production > 0},
			Text(fmt.Sprintf("+%d", production))),
		Div(Classes{"allocation-num": true, "allocation-num--neg": upkeep > 0},
			Text(fmt.Sprintf("-%d", upkeep))),
		Div(Classes{"allocation-num": true, "allocation-num--bold": true, "allocation-num--pos": net > 0, "allocation-num--neg": net < 0},
			Text(fmt.Sprintf("%+d", net))),
		Div(Class("allocation-res"), Text(resourceLabel+"/tick")),
	)
}

func idleRow(initialValue, production, upkeep int) Node {
	net := production - upkeep
	idleExpr := "100 - ($wood_pct + $stone_pct + $knowledge_pct + $devotion_pct + $mana_pct + $food_pct)"
	return Div(Class("allocation-row allocation-row--idle"),
		ds.Computed("idle_pct", idleExpr),
		Div(Class("allocation-shield"), Shield("crown", 20, false)),
		Div(Class("allocation-role"), Text("Idle")),
		Div(Class("text-muted"), Text("remain unbound")),
		Div(Class("allocation-pct"),
			Span(ds.Text("$idle_pct"), Text(fmt.Sprintf("%d", initialValue))),
			Span(Text("%")),
		),
		Div(Classes{"allocation-num": true, "allocation-num--pos": production > 0},
			Text(fmt.Sprintf("+%d", production))),
		Div(Classes{"allocation-num": true, "allocation-num--neg": upkeep > 0},
			Text(fmt.Sprintf("-%d", upkeep))),
		Div(Classes{"allocation-num": true, "allocation-num--bold": true, "allocation-num--pos": net > 0, "allocation-num--neg": net < 0},
			Text(fmt.Sprintf("%+d", net))),
		Div(Class("allocation-res"), Text("Population/tick")),
	)
}
