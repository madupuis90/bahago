package home

import (
	"net/http"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	. "bahago/internal/layout"
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
		H1(Text("Home Page")),
	})
}
