package resources

import (
	"net/http"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/router"
	. "bahago/internal/ui"

	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"
)

func RegisterRoutes(router router.Router, queries *db.Queries) {

	h := newHandler(queries)

	router.HandleFunc("GET /resources", h.handleResourcePage())
	router.HandleFunc("GET /resources/load", h.handleLoadResource())
	router.HandleFunc("POST /resources/create", h.handleCreateResources())
}

type handler struct {
	queries *db.Queries
}

func newHandler(queries *db.Queries) *handler {
	return &handler{queries: queries}
}

func (h *handler) handleResourcePage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _ := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser)
		resourcePage(user).Render(w)
	}
}

func (h *handler) handleCreateResources() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

	}
}

func (h *handler) handleLoadResource() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

	}
}

func resourcePage(user *contextkeys.SessionUser) Node {
	return Layout(
		LayoutArgs{
			Title: "Resources",
			User:  user,
		},
		Div(Class("flex"), ds.Init(datastar.GetSSE("/resources/load")),
			Div(Class("flex col"),
				Div(Text("wood meter")),
				Div(
					Input(ds.Bind("wood")),
				),
			),
			Div(Class("flex col"),
				Div(Text("stone meter")),
				Div(
					Input(ds.Bind("stone")),
				),
			),
			Div(Class("flex col"),
				Div(Text("food meter")),
				Div(
					Input(ds.Bind("food")),
				),
			),
		),
	)
}
