package server

import (
	"net/http"

	"github.com/mad/bahago/internal/ui/pages"
)

func (s *Server) RegisterRoutes() {
	// pages
	s.router.HandleFunc("/", s.handleHomePage())
	s.router.HandleFunc("/about", s.handleAboutPage())
	s.router.HandleFunc("/login", s.handleLoginPage())
	s.router.HandleFunc("/test", s.handleTestPage())

	// actions
	s.router.HandleFunc("/read", pages.Read)
	s.router.HandleFunc("/write", pages.Write)

	// static assets
	fs := http.FileServer(http.Dir("web/static"))
	s.router.Handle("/static/", http.StripPrefix("/static/", fs))
}
