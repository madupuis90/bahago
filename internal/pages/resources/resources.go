package resources

import (
	"net/http"

	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/router"
	"bahago/internal/routes"
	. "bahago/internal/ui"
)

func RegisterRoutes(router router.Router, queries *db.Queries) {

	h := newHandler(queries)

	router.HandleFunc("GET "+routes.ResourcesPath, h.handleResourcePage())
	router.HandleFunc("GET "+routes.ResourcesLoadPath, h.handleLoadResource())
	router.HandleFunc("POST "+routes.ResourcesCreatePath, h.handleCreateResources())
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

type ResourceSignals struct {
	Wood  string `json:"wood"`
	Stone string `json:"stone"`
	Food  string `json:"food"`
}

var resourceSignals = ResourceSignals{Wood: "wood", Stone: "stone", Food: "food"}

func resourcePage(user *contextkeys.SessionUser) Node {
	return Layout(
		LayoutArgs{
			Title: "Resources",
			User:  user,
		},
		Div(Class("flex"), ds.Init(datastar.GetSSE(routes.ResourcesLoadPath)),
			Div(Class("flex col"),
				Div(Text("wood meter")),
				Div(
					Input(ds.Bind(resourceSignals.Wood)),
				),
			),
			Div(Class("flex col"),
				Div(Text("stone meter")),
				Div(
					Input(ds.Bind(resourceSignals.Stone)),
				),
			),
			Div(Class("flex col"),
				Div(Text("food meter")),
				Div(
					Input(ds.Bind(resourceSignals.Food)),
				),
			),
		),
	)
}
