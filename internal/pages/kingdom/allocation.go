package kingdom

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
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

const allocationErrorID = "allocation-error"

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handleResourcePage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom, _ := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		sigs := sigDef.New()
		sigs.WoodPct.Value = kingdom.WoodPct
		sigs.StonePct.Value = kingdom.StonePct
		sigs.FoodPct.Value = kingdom.FoodPct
		sigs.ManaPct.Value = kingdom.ManaPct
		sigs.DevotionPct.Value = kingdom.DevotionPct
		sigs.KnowledgePct.Value = kingdom.KnowledgePct
		sigs.IdlePct.Value = kingdom.IdlePct

		NewPage("Resources", AppLayout(r), resourceContent(sigs)).Render(w)
	}
}

func (h *handler) handleSaveAllocation() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _ := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser)

		input := &allocationSignals{}
		if err := datastar.ReadSignals(r, input); err != nil {
			log.Printf("save-allocation: read signals: %v", err)
			sse := datastar.NewSSE(w, r)
			sse.PatchElementGostar(allocationErrorComponent(errors.New("invalid request")))
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
			sse := datastar.NewSSE(w, r)
			sse.PatchElementGostar(allocationErrorComponent(errors.New("allocation cannot exceed 100%")))
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
		if _, err := h.queries.UpdateKingdomAllocations(r.Context(), params); err != nil {
			log.Printf("save-allocation: update allocations: %v", err)
			sse := datastar.NewSSE(w, r)
			sse.PatchElementGostar(allocationErrorComponent(errors.New("failed to save allocation")))
			return
		}

		sse := datastar.NewSSE(w, r)
		sse.PatchElementGostar(allocationErrorComponent(nil))
	}
}

// ── Page ──────────────────────────────────────────────────────────────────────

func resourceContent(sigs allocationSignals) Node {
	return Div(
		ds.Signals(SignalMap(sigs)),
		Div(Class("allocation-card panel"),
			Table(Class("allocation-table"),
				THead(
					Tr(
						Th(Text("Role")),
						Th(Text("Assignment")),
						Th(Text("Percentage")),
						Th(Text("Rate")),
						Th(Text("Upkeep")),
						Th(Text("Total")),
						Th(Text("Resource")),
					),
				),
				TBody(
					allocationRow("Logger", sigs.WoodPct, "Wood/hour"),
					allocationRow("Miner", sigs.StonePct, "Stone/hour"),
					allocationRow("Scholar", sigs.KnowledgePct, "Knowledge/hour"),
					allocationRow("Clergy", sigs.DevotionPct, "Devotion/hour"),
					allocationRow("Disciple", sigs.ManaPct, "Mana/hour"),
					allocationRow("Farmer", sigs.FoodPct, "Food/hour"),
					idleRow(sigs),
				),
			),
			Div(Class("allocation-footer"),
				Button(
					ds.On("click", datastar.PostSSE(routes.KingdomResourcesSavePath)),
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
func allocationRow(roleName string, sig Signal[int], resourceLabel string) Node {
	return Tr(
		Td(Class("allocation-role"), Text(roleName)),
		Td(Class("allocation-assignment"),
			Button(Class("allocation-btn"), ds.On("click", sig.Ref+" = Math.max(0, "+sig.Ref+" - 1)"), Text("-")),
			Input(Type("range"), Min("0"), Max("100"), ds.Bind(sig.Key)),
			Button(Class("allocation-btn"), ds.On("click", sig.Ref+" = Math.min(100, "+sig.Ref+" + 1)"), Text("+")),
		),
		Td(Class("allocation-percentage"),
			Span(ds.Text(sig.Ref)),
		),
		Td(Class("allocation-rate"), Text("-")),
		Td(Class("allocation-upkeep"), Text("-")),
		Td(Class("allocation-total"), Text("-")),
		Td(Class("allocation-resource"), Text(resourceLabel)),
	)
}

func idleRow(sigs allocationSignals) Node {
	idleExpr := fmt.Sprintf("100 - (%s + %s + %s + %s + %s + %s)",
		sigs.WoodPct.Ref,
		sigs.StonePct.Ref,
		sigs.KnowledgePct.Ref,
		sigs.DevotionPct.Ref,
		sigs.ManaPct.Ref,
		sigs.FoodPct.Ref,
	)
	return Tr(
		ds.Computed(sigs.IdlePct.Key, idleExpr),
		Td(Class("allocation-role"), Text("Idle")),
		Td(Class("allocation-assignment")),
		Td(Class("allocation-percentage"),
			Span(ds.Text(sigs.IdlePct.Ref)),
		),
		Td(Class("allocation-rate"), Text("-")),
		Td(Class("allocation-upkeep"), Text("-")),
		Td(Class("allocation-total"), Text("-")),
		Td(Class("allocation-resource"), Text("Population/hour")),
	)
}
