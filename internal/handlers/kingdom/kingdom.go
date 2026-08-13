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
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/game"
	"bahago/internal/hub"
	"bahago/internal/router"
	"bahago/internal/routes"
	. "bahago/internal/ui"
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

// ── Combat log grouping ───────────────────────────────────────────────────────

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

// ── Components ────────────────────────────────────────────────────────────────

func kingdomOverviewSection(kingdom *db.Kingdom, combatLog []combatLogDisplay) Node {
	return Div(
		Div(ds.Init(GetSSENoSignals(routes.KingdomRefreshPath))),
		PageHeader(kingdom.Name),
		Div(Class("overview-grid"),
			overviewCard("red", "Journal Log",
				Div(Class("combat-log"),
					If(len(combatLog) == 0,
						P(Class("combat-log-empty text-muted"), Text("No recent events.")),
					),
					Group(Map(combatLog, combatLogEntry)),
				),
			),
			overviewCard("green", "World Events",
				P(Class("text-muted"), Text("— placeholder —")),
			),
		),
	)
}

func overviewCard(tabVariant, tabLabel string, content Node) Node {
	return Div(Class("card"),
		Span(Class("card-tab card-tab--"+tabVariant), Text(tabLabel)),
		Div(Class("card-inner"), content),
	)
}

func combatLogEntry(e combatLogDisplay) Node {
	attackerNames := participantNames(e.Attackers)
	defenderNames := participantNames(e.Defenders)
	attackersWon := e.Winner == "attacker"

	var head string
	if attackersWon {
		head = fmt.Sprintf("%s prevailed against %s", attackerNames, defenderNames)
	} else {
		head = fmt.Sprintf("%s repelled %s", defenderNames, attackerNames)
	}

	var attackerUnits, defenderUnits []game.CombatUnitRecord
	if err := json.Unmarshal(e.AttackerUnits, &attackerUnits); err != nil {
		log.Printf("kingdom: unmarshal attacker units: %v", err)
	}
	if err := json.Unmarshal(e.DefenderUnits, &defenderUnits); err != nil {
		log.Printf("kingdom: unmarshal defender units: %v", err)
	}

	body := combatBodyText(attackerUnits, defenderUnits)

	marg := ""
	for _, p := range e.Attackers {
		if p.PopulationGained > 0 {
			marg = fmt.Sprintf("+%d population captured", p.PopulationGained)
			break
		}
	}
	if marg == "" {
		if attackersWon {
			marg = "attackers won"
		} else {
			marg = "defenders held"
		}
	}

	return El("article", Class("combat-log-entry"),
		Div(Class("combat-log-entry-main"),
			Div(Class("combat-log-entry-time"), Text(fmt.Sprintf("tick %d", e.TickID))),
			Div(Class("combat-log-entry-head"), Text(head)),
			P(Class("combat-log-entry-body"), Text(body)),
		),
		Div(Class("combat-log-entry-note"), Text(marg)),
	)
}

func participantNames(ps []combatParticipant) string {
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = p.Name
	}
	if len(names) == 0 {
		return "an unknown host"
	}
	return strings.Join(names, ", ")
}

func combatBodyText(attackers, defenders []game.CombatUnitRecord) string {
	att, attCas := summarizeUnits(attackers)
	def, defCas := summarizeUnits(defenders)

	if att == "" && def == "" {
		return "The field was empty; the day passed without combat."
	}
	parts := []string{}
	if att != "" {
		parts = append(parts, fmt.Sprintf("Attackers brought %s", att))
	} else {
		parts = append(parts, "Attackers brought no host")
	}
	if def != "" {
		parts = append(parts, fmt.Sprintf("defenders mustered %s", def))
	} else {
		parts = append(parts, "defenders mustered none")
	}
	sentence := strings.Join(parts, "; ") + "."
	if attCas > 0 || defCas > 0 {
		sentence += fmt.Sprintf(" Casualties: %d on the attack, %d in defence.", attCas, defCas)
	}
	return sentence
}

func summarizeUnits(units []game.CombatUnitRecord) (string, int) {
	parts := []string{}
	casualties := 0
	for _, u := range units {
		def := game.UnitDefs[u.UnitType]
		parts = append(parts, fmt.Sprintf("%d %s", u.Count, def.Name))
		casualties += u.Casualties
	}
	return strings.Join(parts, ", "), casualties
}
