package server

import (
	"net/http"

	"bahago/internal/ui/pages"
)

func (s *Server) RegisterRoutes() {
	// pages
	s.router.HandleFunc("GET /", s.handleHomePage())
	s.router.HandleFunc("GET /about", s.handleAboutPage())
	s.router.HandleFunc("GET /login", s.handleLoginPage())
	s.router.HandleFunc("GET /test", s.handleTestPage())

	// actions
	s.router.HandleFunc("GET /read", pages.Read)
	s.router.HandleFunc("POST /write", pages.Write)
	s.router.HandleFunc("POST /resources/create", s.handleCreateResources())

	// static assets
	fs := http.FileServer(http.Dir("web/static"))
	s.router.Handle("GET /static/", http.StripPrefix("/static/", fs))
}
