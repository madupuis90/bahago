package server

import (
	"net/http"

	"github.com/mad/bahago/internal/ui/pages"
)

func (s *Server) handleHomePage() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		p := pages.Home()
		p.Render(w)
	}
}

func (s *Server) handleAboutPage() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		p := pages.About()
		p.Render(w)
	}
}

func (s *Server) handleLoginPage() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		p := pages.Login()
		p.Render(w)
	}
}

func (s *Server) handleTestPage() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		p := pages.Test()
		p.Render(w)
	}
}
