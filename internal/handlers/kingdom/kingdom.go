package kingdom

import (
	"encoding/json"
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
	. "bahago/internal/layout"
	"bahago/internal/router"
	"bahago/internal/routes"
)

// ── Route registration ────────────────────────────────────────────────────────

func RegisterRoutes(r router.Router, queries db.Querier, tickHub *hub.Hub) {
	h := newHandler(queries, tickHub)
	r.HandleFunc("GET "+routes.KingdomPath, h.handleKingdomPage())
	r.HandleFunc("GET "+routes.KingdomRefreshPath, h.handleKingdomRefresh())
}

type handler struct {
	queries db.Querier
	hub     *hub.Hub
}

func newHandler(queries db.Querier, tickHub *hub.Hub) *handler {
	return &handler{queries: queries, hub: tickHub}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handleKingdomPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		rows, err := h.queries.GetRecentCombatLogs(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("kingdom overview: get combat log: %v", err)
		}
		KingdomLayout(r, kingdom.Name, routes.KingdomPath, kingdom, kingdomOverviewSection(kingdom, groupCombatLog(rows))).Render(w)
	}
}

func (h *handler) handleKingdomRefresh() http.HandlerFunc {
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
				rows, err := h.queries.GetRecentCombatLogs(r.Context(), k.ID)
				if err != nil {
					log.Printf("kingdom refresh: get combat log: %v", err)
				}
				page := kingdomOverviewSection(&k, groupCombatLog(rows))
				if err := sse.PatchElementGostar(MainContent(page)); err != nil {
					log.Printf("kingdom refresh: patch: %v", err)
					return
				}
			}
		}
	}
}

// ── Components ────────────────────────────────────────────────────────────────

type combatParticipant struct {
	Name             string
	PopulationGained int
}

type combatLogDisplay struct {
	db.GetRecentCombatLogsRow
	Attackers []combatParticipant
	Defenders []combatParticipant
}

func groupCombatLog(rows []db.GetRecentCombatLogsRow) []combatLogDisplay {
	var entries []combatLogDisplay
	index := map[int]int{}
	for _, row := range rows {
		i, ok := index[row.ID]
		if !ok {
			i = len(entries)
			index[row.ID] = i
			entries = append(entries, combatLogDisplay{GetRecentCombatLogsRow: row})
		}
		p := combatParticipant{Name: row.ParticipantName, PopulationGained: row.PopulationGained}
		if row.ParticipantRole == "attacker" {
			entries[i].Attackers = append(entries[i].Attackers, p)
		} else {
			entries[i].Defenders = append(entries[i].Defenders, p)
		}
	}
	return entries
}

func kingdomOverviewSection(kingdom *db.Kingdom, combatLog []combatLogDisplay) Node {
	return Div(
		Div(ds.Init(GetSSENoSignals(routes.KingdomRefreshPath))),
		Div(Class("panel kingdom-overview"),
			H1(Class("kingdom-name"), Text(kingdom.Name)),
			kingdomStat("Population", fmt.Sprintf("%d", kingdom.Population)),
		),
		recentCombatSection(combatLog),
	)
}

func kingdomStat(label, value string) Node {
	return Div(Class("kingdom-stat"),
		Span(Class("kingdom-stat-label"), Text(label)),
		Span(Class("kingdom-stat-value"), Text(value)),
	)
}

func recentCombatSection(entries []combatLogDisplay) Node {
	return Div(Class("panel kingdom-combat-log"),
		P(Class("panel-title"), Text("Recent Combat")),
		If(len(entries) == 0,
			P(Class("kingdom-combat-log-empty"), Text("No recent combat.")),
		),
		Map(entries, combatLogEntry),
	)
}

func combatLogEntry(e combatLogDisplay) Node {
	var attackerUnits, defenderUnits []game.CombatUnitRecord
	if err := json.Unmarshal(e.AttackerUnits, &attackerUnits); err != nil {
		log.Printf("kingdom: unmarshal attacker units: %v", err)
	}
	if err := json.Unmarshal(e.DefenderUnits, &defenderUnits); err != nil {
		log.Printf("kingdom: unmarshal defender units: %v", err)
	}

	attackerNames := make([]string, len(e.Attackers))
	for i, p := range e.Attackers {
		attackerNames[i] = p.Name
	}
	defenderNames := make([]string, len(e.Defenders))
	for i, p := range e.Defenders {
		defenderNames[i] = p.Name
	}

	var gainers []combatParticipant
	for _, p := range e.Attackers {
		if p.PopulationGained > 0 {
			gainers = append(gainers, p)
		}
	}

	attackersWon := e.Winner == "attacker"

	return Div(Class("kingdom-combat-log-entry"),
		Div(Class("kingdom-combat-log-date"),
			Text(fmt.Sprintf("Tick %d — %s", e.TickID, e.OccurredAt.Format("Jan 2, 15:04"))),
		),
		Div(Class("kingdom-combat-log-sides"),
			combatSideTitle("Attackers", attackersWon),
			combatSideTitle("Defenders", !attackersWon),
			combatSideBlock("Kingdoms:", strings.Join(attackerNames, ", ")),
			combatSideBlock("Kingdoms:", strings.Join(defenderNames, ", ")),
			combatUnitsBlock("Armies:", attackerUnits, false),
			combatUnitsBlock("Armies:", defenderUnits, false),
			combatUnitsBlock("Casualties", attackerUnits, true),
			combatUnitsBlock("Casualties", defenderUnits, true),
		),
		Iff(len(gainers) > 0, func() Node {
			return Div(Class("kingdom-combat-log-footer"),
				Group(Map(gainers, func(g combatParticipant) Node {
					return P(Text(fmt.Sprintf("%s stole %d population", g.Name, g.PopulationGained)))
				})),
			)
		}),
	)
}

func combatSideTitle(title string, winner bool) Node {
	return P(Classes{
		"kingdom-combat-log-side-title":         true,
		"kingdom-combat-log-side-title--winner": winner,
	}, Text(title))
}

func combatSideBlock(label, value string) Node {
	return Div(Class("kingdom-combat-log-side-block"),
		P(Class("kingdom-combat-log-section-label"), Text(label)),
		P(Text(value)),
	)
}

func combatUnitsBlock(label string, units []game.CombatUnitRecord, showCasualties bool) Node {
	return Div(Class("kingdom-combat-log-side-block"),
		P(Class("kingdom-combat-log-section-label"), Text(label)),
		Group(Map(units, func(u game.CombatUnitRecord) Node {
			def := game.UnitDefs[u.UnitType]
			if showCasualties {
				return If(u.Casualties > 0,
					P(Text(fmt.Sprintf("%s: %d", def.Name, u.Casualties))),
				)
			}
			return P(Text(fmt.Sprintf("%s: %d", def.Name, u.Count)))
		})),
		If(showCasualties && noCasualties(units),
			P(Class("kingdom-combat-log-empty-note"), Text("No casualties")),
		),
		If(!showCasualties && len(units) == 0,
			P(Class("kingdom-combat-log-empty-note"), Text("No resistance")),
		),
	)
}

func noCasualties(units []game.CombatUnitRecord) bool {
	for _, u := range units {
		if u.Casualties > 0 {
			return false
		}
	}
	return true
}
