package server

import (
	"bahago/internal/database/db"
	"bahago/internal/middleware"
	"bahago/internal/pages/chat"
	"bahago/internal/pages/home"
	"bahago/internal/pages/login"
	"bahago/internal/pages/realm"
	"bahago/internal/pages/resources"
	"bahago/internal/router"
	"bahago/web"
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

	// middleware
	authMiddleware := middleware.AuthMiddleware(s.queries)

	// public pages
	home.RegisterRoutes(s.router)
	login.RegisterRoutes(s.router, s.queries)

	// protected pages
	protectedRouter := &router.MiddlewareRouter{
		Router:     s.router,
		Middleware: authMiddleware,
	}
	realm.RegisterRoutes(protectedRouter, s.queries)
	chat.RegisterRoutes(protectedRouter)
	resources.RegisterRoutes(protectedRouter, s.queries)

	// static assets — embedded into the binary at compile time
	s.router.Handle("GET /static/", http.FileServer(http.FS(web.Static)))

	// redirect to home
	s.router.Handle("/", http.RedirectHandler("/home", http.StatusMovedPermanently))
}

// implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}
