package server

import (
	"bahago/internal/database/db"
	"bahago/internal/pages/chat"
	"bahago/internal/pages/home"
	"bahago/internal/pages/login"
	"bahago/internal/pages/resources"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	router  *http.ServeMux
	db      *pgxpool.Pool
	queries *db.Queries
}

func New(pool *pgxpool.Pool) *Server {
	s := &Server{
		router:  http.NewServeMux(),
		db:      pool,
		queries: db.New(pool),
	}

	s.registerRoutes()

	return s
}

func (s *Server) registerRoutes() {

	// pages
	home.RegisterRoutes(s.router)
	login.RegisterRoutes(s.router)
	chat.RegisterRoutes(s.router)
	resources.RegisterRoutes(s.router, s.queries)

	// static assets
	fs := http.FileServer(http.Dir("web/static"))
	s.router.Handle("GET /static/", http.StripPrefix("/static/", fs))
}

// implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
