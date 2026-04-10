package kingdom

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
	"bahago/internal/routes"
	. "bahago/internal/ui"
)

// ── Signal struct ────────────────────────────────────────────────────────────

type allocationSignals struct {
	IdlePct      Signal[int] `json:"idle_pct"`
	WoodPct      Signal[int] `json:"wood_pct"`
	StonePct     Signal[int] `json:"stone_pct"`
	FoodPct      Signal[int] `json:"food_pct"`
	ManaPct      Signal[int] `json:"mana_pct"`
	DevotionPct  Signal[int] `json:"devotion_pct"`
	KnowledgePct Signal[int] `json:"knowledge_pct"`
}

var sigDef = NewSignalDef[allocationSignals]()

// ── Component IDs ─────────────────────────────────────────────────────────────

const (
	allocationContentID = "allocation-content"
	allocationErrorID   = "allocation-error"
)

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handleAllocationPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom, _ := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		rates := game.ComputeRates(*kingdom)
		sigs := sigsFromKingdom(*kingdom)
		NewPage("Allocation", AppLayout(r), allocationContent(sigs, rates)).Render(w)
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
			input.WoodPct.Value, input.StonePct.Value, input.FoodPct.Value,
			input.ManaPct.Value, input.DevotionPct.Value, input.KnowledgePct.Value,
		}
		for _, v := range values {
			if v < 0 || v > 100 {
				datastar.NewSSE(w, r).PatchElementGostar(allocationErrorComponent(errors.New("allocation values must be between 0 and 100")))
				return
			}
		}

		total := input.WoodPct.Value + input.StonePct.Value + input.FoodPct.Value +
			input.ManaPct.Value + input.DevotionPct.Value + input.KnowledgePct.Value
		if total > 100 {
			datastar.NewSSE(w, r).PatchElementGostar(allocationErrorComponent(errors.New("allocation cannot exceed 100%")))
			return
		}

		params := db.UpdateKingdomAllocationsParams{
			UserID:       user.ID,
			WoodPct:      input.WoodPct.Value,
			StonePct:     input.StonePct.Value,
			FoodPct:      input.FoodPct.Value,
			ManaPct:      input.ManaPct.Value,
			DevotionPct:  input.DevotionPct.Value,
			KnowledgePct: input.KnowledgePct.Value,
			IdlePct:      100 - total,
		}
		updatedKingdom, err := h.queries.UpdateKingdomAllocations(r.Context(), params)
		if err != nil {
			log.Printf("save-allocation: update allocations: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(allocationErrorComponent(errors.New("failed to save allocation")))
			return
		}

		rates := game.ComputeRates(updatedKingdom)
		sse := datastar.NewSSE(w, r)
		sse.PatchElementGostar(allocationContent(sigsFromKingdom(updatedKingdom), rates))
		sse.PatchElementGostar(allocationErrorComponent(nil))
	}
}

// sigsFromKingdom builds an allocationSignals from stored kingdom percentages.
func sigsFromKingdom(k db.Kingdom) allocationSignals {
	sigs := sigDef.New()
	sigs.WoodPct.Value = k.WoodPct
	sigs.StonePct.Value = k.StonePct
	sigs.FoodPct.Value = k.FoodPct
	sigs.ManaPct.Value = k.ManaPct
	sigs.DevotionPct.Value = k.DevotionPct
	sigs.KnowledgePct.Value = k.KnowledgePct
	sigs.IdlePct.Value = k.IdlePct
	return sigs
}

// ── Page ──────────────────────────────────────────────────────────────────────

func allocationContent(sigs allocationSignals, rates game.ResourceRates) Node {
	return Div(ID(allocationContentID),
		ds.Signals(SignalMap(sigs)),
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
					allocationRow("Woodcutter", sigs.WoodPct, "Wood", rates.WoodProduction, rates.WoodUpkeep),
					allocationRow("Miner", sigs.StonePct, "Stone", rates.StoneProduction, rates.StoneUpkeep),
					allocationRow("Farmer", sigs.FoodPct, "Food", rates.FoodProduction, rates.FoodUpkeep),
					allocationRow("Clergy", sigs.DevotionPct, "Devotion", rates.DevotionProduction, rates.DevotionUpkeep),
					allocationRow("Disciple", sigs.ManaPct, "Mana", rates.ManaProduction, rates.ManaUpkeep),
					allocationRow("Scholar", sigs.KnowledgePct, "Knowledge", rates.KnowledgeProduction, rates.KnowledgeUpkeep),
					idleRow(sigs, rates.PopulationProduction, rates.PopulationUpkeep),
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
	return Div(ID(allocationErrorID), Text(msg))
}

func allocationRow(roleName string, sig Signal[int], resourceLabel string, production, upkeep int) Node {
	net := production - upkeep
	return Tr(
		Td(Class("allocation-role"), Text(roleName)),
		Td(Class("allocation-assignment"),
			Button(Class("btn allocation-btn plus-five"), ds.On("click", sig.Ref+" = Math.max(0, "+sig.Ref+" - 5)"), Text("−5")),
			Button(Class("btn allocation-btn"), ds.On("click", sig.Ref+" = Math.max(0, "+sig.Ref+" - 1)"), Text("−")),
			Input(Type("range"), Min("0"), Max("100"), ds.Bind(sig.Key)),
			Button(Class("btn allocation-btn"), ds.On("click", sig.Ref+" = Math.min(100, "+sig.Ref+" + 1)"), Text("+")),
			Button(Class("btn allocation-btn plus-five"), ds.On("click", sig.Ref+" = Math.min(100, "+sig.Ref+" + 5)"), Text("+5")),
		),
		Td(Class("allocation-percentage"),
			Span(ds.Text(sig.Ref)),
			Span(Text("%")),
		),
		Td(Classes{"allocation-production": true, "text-positive": production > 0}, Text(fmt.Sprintf("+%d", production))),
		Td(Classes{"allocation-upkeep": true, "text-negative": upkeep > 0}, Text(fmt.Sprintf("-%d", upkeep))),
		Td(Classes{"allocation-total": true, "text-positive": net > 0, "text-negative": net < 0}, Text(fmt.Sprintf("%+d", net))),
		Td(Class("allocation-resource"), Text(resourceLabel+"/tick")),
	)
}

func idleRow(sigs allocationSignals, production, upkeep int) Node {
	net := production - upkeep
	idleExpr := fmt.Sprintf("100 - (%s + %s + %s + %s + %s + %s)",
		sigs.WoodPct.Ref,
		sigs.StonePct.Ref,
		sigs.KnowledgePct.Ref,
		sigs.DevotionPct.Ref,
		sigs.ManaPct.Ref,
		sigs.FoodPct.Ref,
	)
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
			ds.Computed(sigs.IdlePct.Key, idleExpr),
			Td(Class("allocation-role"), Text("Idle")),
			Td(Class("allocation-assignment")),
			Td(Class("allocation-percentage"),
				Span(ds.Text(sigs.IdlePct.Ref)),
				Span(Text("%")),
			),
			Td(Classes{"allocation-production": true, "text-positive": production > 0}, Text(fmt.Sprintf("+%d", production))),
			Td(Classes{"allocation-upkeep": true, "text-negative": upkeep > 0}, Text(fmt.Sprintf("-%d", upkeep))),
			Td(Classes{"allocation-total": true, "text-positive": net > 0, "text-negative": net < 0}, Text(fmt.Sprintf("%+d", net))),
			Td(Class("allocation-resource"), Text("Population/tick")),
		),
	})
}
