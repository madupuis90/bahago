package home

import (
	"net/http"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	. "bahago/internal/layout"
	"bahago/internal/router"
	"bahago/internal/routes"
)

func RegisterRoutes(router router.Router) {
	h := newHandler()
	router.HandleFunc("GET "+routes.HomePath, h.handleHomePage())
}

type handler struct {
}

func newHandler() *handler {
	return &handler{}
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
