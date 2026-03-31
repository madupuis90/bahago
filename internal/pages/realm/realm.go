package realm

import (
	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/router"
	"net/http"

	. "bahago/internal/ui"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

const (
	RealmPath = "/realm"
)

func RegisterRoutes(router router.Router, queries *db.Queries) {

	h := newHandler(queries)

	router.HandleFunc("GET "+RealmPath, h.handleRealmPage())
}

type handler struct {
	queries *db.Queries
}

func newHandler(queries *db.Queries) *handler {
	return &handler{queries: queries}
}

func (h *handler) handleRealmPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _ := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser)
		realmPage(user).Render(w)
	}
}

func realmPage(user *contextkeys.SessionUser) Node {
	return Layout(
		LayoutArgs{
			Title: "Realm",
			User:  user,
		},
		Div(Text("bob")),
	)
}
