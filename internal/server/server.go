package server

import "net/http"

type Server struct {
	router *http.ServeMux
	// db
}

func New() *Server {
	s := &Server{
		router: http.NewServeMux(),
	}

	s.RegisterRoutes()

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
