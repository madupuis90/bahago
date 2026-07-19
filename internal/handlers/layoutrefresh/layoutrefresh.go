package layoutrefresh

import (
	"fmt"
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

// patchChrome re-renders the topbar so resource values reflect the latest tick
// and the messages badge reflects the latest unread count. It also patches the
// layout-scoped resource signals ($wood/etc.) so DynamicCostPill availability
// recomputes client-side. The topbar text itself stays server-rendered (patched
// HTML) for display; the signals are the logic source for cost pills. Both are
// written from the same kingdom in the same tick so display and logic agree.
func patchChrome(sse *datastar.ServerSentEventGenerator, kingdom *db.Kingdom, currentPath string, unreadCount int) error {
	if err := sse.PatchElementGostar(KingdomTopbar(kingdom, currentPath, unreadCount)); err != nil {
		return fmt.Errorf("patch topbar: %w", err)
	}
	if err := sse.MarshalAndPatchSignals(ResourceSignals(kingdom)); err != nil {
		return fmt.Errorf("patch resource signals: %w", err)
	}
	return nil
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
