package layoutrefresh

import (
	"log"
	"net/http"

	"github.com/starfederation/datastar-go/datastar"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/hub"
	"bahago/internal/router"
	"bahago/internal/routes"
	. "bahago/internal/ui"
)

// patchChrome re-renders the topbar (so resource values reflect the latest tick)
// and the bottom nav (so the messages badge reflects the latest unread count).
func patchChrome(sse *datastar.ServerSentEventGenerator, kingdom *db.Kingdom, currentPath string, unreadCount int) error {
	if err := sse.PatchElementGostar(KingdomTopbar(kingdom, currentPath)); err != nil {
		return err
	}
	return sse.PatchElementGostar(KingdomBottomNav(currentPath, unreadCount))
}

// ── Route registration ────────────────────────────────────────────────────────

func RegisterRoutes(r router.Router, queries db.Querier, tickHub *hub.Hub) {
	h := &handler{queries: queries, hub: tickHub}
	r.HandleFunc("GET "+routes.KingdomLayoutRefreshPath, h.handleLayoutRefresh())
}

type handler struct {
	queries db.Querier
	hub     *hub.Hub
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handleLayoutRefresh() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		currentPath := r.URL.Query().Get("path")

		unreadCount, err := h.queries.CountUnreadMessages(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("layout refresh: count unread: %v", err)
			unreadCount = 0
		}

		sse := datastar.NewSSE(w, r)
		if err := patchChrome(sse, kingdom, currentPath, unreadCount); err != nil {
			log.Printf("layout refresh: initial patch: %v", err)
			return
		}

		ch, cleanup := h.hub.Subscribe(kingdom.ID)
		defer cleanup()

		for {
			select {
			case <-r.Context().Done():
				return
			case k := <-ch:
				unreadCount, err := h.queries.CountUnreadMessages(r.Context(), k.ID)
				if err != nil {
					log.Printf("layout refresh: count unread: %v", err)
					unreadCount = 0
				}
				if err := patchChrome(sse, &k, currentPath, unreadCount); err != nil {
					log.Printf("layout refresh: patch: %v", err)
					return
				}
			}
		}
	}
}
