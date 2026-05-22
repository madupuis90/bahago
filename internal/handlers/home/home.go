package home

import (
	"net/http"

	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	. "bahago/internal/ui"
	"bahago/internal/router"
	"bahago/internal/routes"
)

func RegisterRoutes(r router.Router) {
	h := newHandler()
	r.HandleFunc("GET /", h.handleRoot())
	r.HandleFunc("GET "+routes.HomePath, h.handleHomePage())
}

type handler struct {
}

func newHandler() *handler {
	return &handler{}
}

func (h *handler) handleRoot() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _ := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser)
		if user != nil {
			http.Redirect(w, r, routes.KingdomPath, http.StatusFound)
			return
		}
		http.Redirect(w, r, routes.HomePath, http.StatusFound)
	}
}

func (h *handler) handleHomePage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		HomeLayout(r, "Home", homeContent()).Render(w)
	}
}

func homeContent() Node {
	return Group([]Node{
		H1(Class("page-title"), Text("Home Page")),
		flipCard(),
	})
}

func flipCard() Node {
	return Div(Class("flip-scene"),
		ds.Signals(map[string]any{
			"flipped": true,
		}),
		Div(Class("flip-card flip-card--flipped"),
			ds.Class("'flip-card--flipped'", "$flipped"),
			ds.On("click", "$flipped = !$flipped"),
			Div(Class("flip-card__face flip-card__face--front"),
				Img(Src("/static/swordman.png"), Alt("Knight")),
			),
			Div(Class("flip-card__face flip-card__face--back")),
		),
	)
}
