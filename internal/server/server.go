package server

import (
	"bahago/internal/database/db"
	"bahago/internal/email"
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
	pool    *pgxpool.Pool
	queries *db.Queries
	sender  *email.Sender
	appURL  string
}

func New(pool *pgxpool.Pool, sender *email.Sender, appURL string) *Server {
	s := &Server{
		router:  http.NewServeMux(),
		pool:    pool,
		queries: db.New(pool),
		sender:  sender,
		appURL:  appURL,
	}

	s.registerRoutes()

	return s
}

func (s *Server) registerRoutes() {

	// middleware
	authMiddleware := middleware.AuthMiddleware(s.queries)

	// public pages
	home.RegisterRoutes(s.router)
	login.RegisterRoutes(s.router, s.queries, s.pool, s.sender, s.appURL)

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
