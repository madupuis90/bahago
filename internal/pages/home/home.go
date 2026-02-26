package home

import (
	. "bahago/internal/ui"
	"net/http"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

func RegisterRoutes(router *http.ServeMux) {
	h := newHandler()
	router.HandleFunc("GET /", h.handleHomePage())
}

type handler struct {
}

func newHandler() *handler {
	return &handler{}
}

func (h *handler) handleHomePage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		homePage().Render(w)
	}
}

func homePage() Node {
	return Layout(
		LayoutArgs{
			Title: "Home",
		},
		H1(Text("Home Page")),
	)
}
