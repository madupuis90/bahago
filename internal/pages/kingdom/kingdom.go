package kingdom

import (
	"net/http"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/router"
	"bahago/internal/routes"
	. "bahago/internal/ui"
)

func RegisterRoutes(router router.Router, queries *db.Queries) {

	h := newHandler(queries)

	router.HandleFunc("GET "+routes.KingdomPath, h.handleKingdomPage())
	router.HandleFunc("GET "+routes.KingdomResourcesPath, h.handleResourcePage())
	router.HandleFunc("GET "+routes.KingdomResourcesLoadPath, h.handleLoadResource())
	router.HandleFunc("POST "+routes.KingdomResourcesCreatePath, h.handleCreateResources())
}

type handler struct {
	queries *db.Queries
}

func newHandler(queries *db.Queries) *handler {
	return &handler{queries: queries}
}

func (h *handler) handleKingdomPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _ := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser)
		kingdomPage(user, r).Render(w)
	}
}

func kingdomPage(user *contextkeys.SessionUser, r *http.Request) Node {
	return Layout(
		LayoutArgs{
			Title:       "Kingdom",
			User:        user,
			CurrentPath: r.URL.Path,
		},
		Div(Text("bob")),
	)
}
