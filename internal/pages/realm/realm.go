package realm

import (
	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/router"
	"fmt"
	"net/http"

	. "bahago/internal/ui"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

func RegisterRoutes(router router.Router, queries *db.Queries) {

	h := newHandler(queries)

	router.HandleFunc("GET /realm", h.handleRealmPage())
}

type handler struct {
	queries *db.Queries
}

func newHandler(queries *db.Queries) *handler {
	return &handler{queries: queries}
}

func (h *handler) handleRealmPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("userid: %v\n", r.Context().Value(contextkeys.UserID))
		realmPage().Render(w)
	}
}

func realmPage() Node {
	return Layout(
		LayoutArgs{
			Title: "Realm",
		},
		Div(Text("bob")),
	)
}
