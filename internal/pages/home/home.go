package home

import (
	"net/http"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/router"
	"bahago/internal/routes"
	. "bahago/internal/ui"
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
		user, _ := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser)
		homePage(user, r).Render(w)
	}
}

func homePage(user *contextkeys.SessionUser, r *http.Request) Node {
	return Layout(
		LayoutArgs{
			Title:       "Home",
			User:        user,
			CurrentPath: r.URL.Path,
		},
		H1(Text("Home Page")),
	)
}
