package home

import (
	"bahago/internal/contextkeys"
	"bahago/internal/router"
	. "bahago/internal/ui"
	"net/http"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

func RegisterRoutes(router router.Router) {
	h := newHandler()
	router.HandleFunc("GET /home", h.handleHomePage())
}

type handler struct {
}

func newHandler() *handler {
	return &handler{}
}

func (h *handler) handleHomePage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _ := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser)
		homePage(user).Render(w)
	}
}

func homePage(user *contextkeys.SessionUser) Node {
	return Layout(
		LayoutArgs{
			Title: "Home",
			User:  user,
		},
		H1(Text("Home Page")),
	)
}
