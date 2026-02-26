package login

import (
	. "bahago/internal/ui"
	"net/http"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

func RegisterRoutes(router *http.ServeMux) {

	h := newHandler()

	router.HandleFunc("GET /login", h.handleLoginPage())
}

type handler struct {
}

func newHandler() *handler {
	return &handler{}
}

func (h *handler) handleLoginPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		loginPage().Render(w)
	}
}

func loginPage() Node {
	return Layout(
		LayoutArgs{
			Title: "Login",
		},
		H1(Text("Login Page")),
	)
}
