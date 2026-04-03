package kingdom

import (
	"net/http"

	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/routes"
	. "bahago/internal/ui"
)

func (h *handler) handleResourcePage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _ := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser)
		resourcePage(user, r).Render(w)
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

type resourceSignalsType struct {
	Wood  string `json:"wood"`
	Stone string `json:"stone"`
	Food  string `json:"food"`
}

var resourceSignals = resourceSignalsType{Wood: "wood", Stone: "stone", Food: "food"}

func resourcePage(user *contextkeys.SessionUser, r *http.Request) Node {
	return Layout(
		LayoutArgs{
			Title:       "Resources",
			User:        user,
			CurrentPath: r.URL.Path,
		},
		Div(Class("flex"), ds.Init(datastar.GetSSE(routes.KingdomResourcesLoadPath)),
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
