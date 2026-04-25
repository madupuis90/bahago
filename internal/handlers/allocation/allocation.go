package allocation

import (
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
	. "bahago/internal/layout"
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
		rates := game.ComputeRates(*kingdom, buildings)
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
					sse.PatchElementGostar(allocationErrorComponent(errors.New("internal error")))
					return
				}
				rates := game.ComputeRates(k, buildings)
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
			datastar.NewSSE(w, r).PatchElementGostar(allocationErrorComponent(errors.New("invalid request")))
			return
		}

		values := []int{
			input.WoodPct, input.StonePct, input.FoodPct,
			input.ManaPct, input.DevotionPct, input.KnowledgePct,
		}
		for _, v := range values {
			if v < 0 || v > 100 {
				datastar.NewSSE(w, r).PatchElementGostar(allocationErrorComponent(errors.New("allocation values must be between 0 and 100")))
				return
			}
		}

		total := input.WoodPct + input.StonePct + input.FoodPct +
			input.ManaPct + input.DevotionPct + input.KnowledgePct
		if total > 100 {
			datastar.NewSSE(w, r).PatchElementGostar(allocationErrorComponent(errors.New("allocation cannot exceed 100%")))
			return
		}

		params := db.UpdateKingdomAllocationsParams{
			UserID:       user.ID,
			WoodPct:      input.WoodPct,
			StonePct:     input.StonePct,
			FoodPct:      input.FoodPct,
			ManaPct:      input.ManaPct,
			DevotionPct:  input.DevotionPct,
			KnowledgePct: input.KnowledgePct,
			IdlePct:      100 - total,
		}
		updatedKingdom, err := h.queries.UpdateKingdomAllocations(r.Context(), params)
		if err != nil {
			log.Printf("save-allocation: update allocations: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(allocationErrorComponent(errors.New("failed to save allocation")))
			return
		}

		buildings, err := h.queries.GetKingdomBuildings(r.Context(), updatedKingdom.ID)
		if err != nil {
			log.Printf("save-allocation: get buildings: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(allocationErrorComponent(errors.New("internal error")))
			return
		}
		rates := game.ComputeRates(updatedKingdom, buildings)
		page := allocationContent(updatedKingdom, rates)
		sse := datastar.NewSSE(w, r)
		if err := sse.PatchElementGostar(MainContent(page)); err != nil {
			log.Printf("save-allocation: patch: %v", err)
		}
	}
}

// ── Page ──────────────────────────────────────────────────────────────────────

func allocationContent(kingdom db.Kingdom, rates game.ResourceRates) Node {
	return Div(
		H1(Class("page-title"), Text("Allocation")),
		Div(ds.Init(GetSSENoSignals(routes.KingdomAllocationRefreshPath))),
		ds.Signals(map[string]any{
			"idle_pct":      kingdom.IdlePct,
			"wood_pct":      kingdom.WoodPct,
			"stone_pct":     kingdom.StonePct,
			"food_pct":      kingdom.FoodPct,
			"mana_pct":      kingdom.ManaPct,
			"devotion_pct":  kingdom.DevotionPct,
			"knowledge_pct": kingdom.KnowledgePct,
		}),
		Div(Class("allocation-card panel"),
			Table(Class("allocation-table"),
				THead(
					Tr(
						Th(Text("Role")),
						Th(Text("Assignment")),
						Th(Text("Percentage")),
						Th(Text("Production")),
						Th(Text("Upkeep")),
						Th(Text("Total")),
						Th(Text("Resource")),
					),
				),
				TBody(
					allocationRow("Woodcutter", "wood_pct", "Wood", kingdom.WoodPct, rates.WoodProduction, rates.WoodUpkeep),
					allocationRow("Miner", "stone_pct", "Stone", kingdom.StonePct, rates.StoneProduction, rates.StoneUpkeep),
					allocationRow("Farmer", "food_pct", "Food", kingdom.FoodPct, rates.FoodProduction, rates.FoodUpkeep),
					allocationRow("Clergy", "devotion_pct", "Devotion", kingdom.DevotionPct, rates.DevotionProduction, rates.DevotionUpkeep),
					allocationRow("Disciple", "mana_pct", "Mana", kingdom.ManaPct, rates.ManaProduction, rates.ManaUpkeep),
					allocationRow("Scholar", "knowledge_pct", "Knowledge", kingdom.KnowledgePct, rates.KnowledgeProduction, rates.KnowledgeUpkeep),
					idleRow(kingdom.IdlePct, rates.PopulationProduction, rates.PopulationUpkeep),
				),
			),
			Div(Class("allocation-footer"),
				Button(Class("btn"),
					ds.On("click", datastar.PostSSE(routes.KingdomAllocationSavePath)),
					Text("Save"),
				),
			),
			allocationErrorComponent(nil),
		),
	)
}

func allocationErrorComponent(err error) Node {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return Div(ID("allocation-alert"), Text(msg))
}

func allocationRow(roleName string, key string, resourceLabel string, initialValue int, production, upkeep int) Node {
	ref := "$" + key
	net := production - upkeep
	return Tr(
		Td(Class("allocation-role"), Text(roleName)),
		Td(Class("allocation-assignment"),
			Button(Class("btn allocation-btn allocation-btn--plus-five"), ds.On("click", fmt.Sprintf("%s = Math.max(0, %s - 5)", ref, ref)), Text("−5")),
			Button(Class("btn allocation-btn"), ds.On("click", fmt.Sprintf("%s = Math.max(0, %s - 1)", ref, ref)), Text("−")),
			Input(Type("range"), Min("0"), Max("100"), Value(fmt.Sprintf("%d", initialValue)), ds.Bind(key)),
			Button(Class("btn allocation-btn"), ds.On("click", fmt.Sprintf("%s = Math.min(100, %s + 1)", ref, ref)), Text("+")),
			Button(Class("btn allocation-btn allocation-btn--plus-five"), ds.On("click", fmt.Sprintf("%s = Math.min(100, %s + 5)", ref, ref)), Text("+5")),
		),
		Td(Class("allocation-percentage"),
			Span(ds.Text(ref), Text(fmt.Sprintf("%d", initialValue))),
			Span(Text("%")),
		),
		Td(Classes{"allocation-production": true, "text-positive": production > 0}, Text(fmt.Sprintf("+%d", production))),
		Td(Classes{"allocation-upkeep": true, "text-negative": upkeep > 0}, Text(fmt.Sprintf("-%d", upkeep))),
		Td(Classes{"allocation-total": true, "text-positive": net > 0, "text-negative": net < 0}, Text(fmt.Sprintf("%+d", net))),
		Td(Class("allocation-resource"), Text(resourceLabel+"/tick")),
	)
}

func idleRow(initialValue int, production, upkeep int) Node {
	net := production - upkeep
	idleExpr := "100 - ($wood_pct + $stone_pct + $knowledge_pct + $devotion_pct + $mana_pct + $food_pct)"
	return Group([]Node{
		Tr(
			Td(Text("---------")),
			Td(Text("---------")),
			Td(Text("---------")),
			Td(Text("---------")),
			Td(Text("---------")),
			Td(Text("---------")),
			Td(Text("---------")),
		),
		Tr(
			ds.Computed("idle_pct", idleExpr),
			Td(Class("allocation-role"), Text("Idle")),
			Td(Class("allocation-assignment")),
			Td(Class("allocation-percentage"),
				Span(ds.Text("$idle_pct"), Text(fmt.Sprintf("%d", initialValue))),
				Span(Text("%")),
			),
			Td(Classes{"allocation-production": true, "text-positive": production > 0}, Text(fmt.Sprintf("+%d", production))),
			Td(Classes{"allocation-upkeep": true, "text-negative": upkeep > 0}, Text(fmt.Sprintf("-%d", upkeep))),
			Td(Classes{"allocation-total": true, "text-positive": net > 0, "text-negative": net < 0}, Text(fmt.Sprintf("%+d", net))),
			Td(Class("allocation-resource"), Text("Population/tick")),
		),
	})
}
