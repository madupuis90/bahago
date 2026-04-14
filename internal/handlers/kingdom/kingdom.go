package kingdom

import (
	"fmt"
	"log"
	"net/http"

	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/hub"
	. "bahago/internal/layout"
	"bahago/internal/router"
	"bahago/internal/routes"
)

// ── Route registration ────────────────────────────────────────────────────────

func RegisterRoutes(r router.Router, queries *db.Queries, tickHub *hub.Hub) {
	h := newHandler(queries, tickHub)
	r.HandleFunc("GET "+routes.KingdomPath, h.handleKingdomPage())
	r.HandleFunc("GET "+routes.KingdomRefreshPath, h.handleKingdomRefresh())
}

type handler struct {
	queries *db.Queries
	hub     *hub.Hub
}

func newHandler(queries *db.Queries, tickHub *hub.Hub) *handler {
	return &handler{queries: queries, hub: tickHub}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handleKingdomPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		KingdomLayout(r, kingdom.Name, r.URL.Path, kingdom, kingdomOverviewSection(kingdom)).Render(w)
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
				page := KingdomLayout(r, k.Name, routes.KingdomPath, &k, kingdomOverviewSection(&k))
				if err := sse.PatchElementGostar(page, datastar.WithSelector("html")); err != nil {
					log.Printf("kingdom refresh: patch: %v", err)
					return
				}
			}
		}
	}
}

// ── Components ────────────────────────────────────────────────────────────────

func kingdomOverviewSection(kingdom *db.Kingdom) Node {
	return Div(
		Div(ds.Init(datastar.GetSSE(routes.KingdomRefreshPath))),
		Div(Class("panel kingdom-overview"),
			P(Class("kingdom-name"), Text(kingdom.Name)),
			kingdomStat("Population", fmt.Sprintf("%d", kingdom.Population)),
		),
	)
}

func kingdomStat(label, value string) Node {
	return Div(Class("kingdom-stat"),
		Span(Class("kingdom-stat-label"), Text(label)),
		Span(Class("kingdom-stat-value"), Text(value)),
	)
}
