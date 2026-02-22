package server

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	router *http.ServeMux
	db     *pgxpool.Pool
}

func New(db *pgxpool.Pool) *Server {
	s := &Server{
		router: http.NewServeMux(),
		db:     db,
	}

	s.RegisterRoutes()

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
