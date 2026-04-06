package server

import (
	"bahago/internal/database/db"
	"bahago/internal/email"
	"bahago/internal/middleware"
	"bahago/internal/pages/auth"
	"bahago/internal/pages/chat"
	"bahago/internal/pages/home"
	"bahago/internal/pages/kingdom"
	"bahago/internal/router"
	"bahago/internal/routes"
	"bahago/web"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	mux     *http.ServeMux
	pool    *pgxpool.Pool
	queries *db.Queries
	sender  *email.Sender
	appURL  string
}

func New(pool *pgxpool.Pool, sender *email.Sender, appURL string) *Server {
	s := &Server{
		mux:     http.NewServeMux(),
		pool:    pool,
		queries: db.New(pool),
		sender:  sender,
		appURL:  appURL,
	}

	s.registerRoutes()

	return s
}

func (s *Server) registerRoutes() {
	// globalRouter applies LoadUser to every route — public or protected —
	// so all handlers can read the current user from context if present - needed in layout
	globalRouter := &router.MiddlewareRouter{
		Router:     s.mux,
		Middleware: middleware.LoadUser(s.queries),
	}

	// public pages
	home.RegisterRoutes(globalRouter)
	auth.RegisterRoutes(globalRouter, s.queries, s.pool, s.sender, s.appURL)
	chat.RegisterRoutes(globalRouter) // Experiment

	// routes requiring an authenticated user
	authRouter := globalRouter.Chain(middleware.RequireAuth)
	kingdomLoadRouter := authRouter.Chain(middleware.LoadKingdom(s.queries))
	kingdomRouter := kingdomLoadRouter.Chain(middleware.RequireKingdom)

	kingdom.RegisterSetupRoutes(kingdomLoadRouter, s.queries)
	kingdom.RegisterRoutes(kingdomRouter, s.queries)

	// static assets — embedded into the binary at compile time
	s.mux.Handle("GET /static/", http.FileServer(http.FS(web.Static)))

	// redirect to home
	s.mux.Handle("/", http.RedirectHandler(routes.HomePath, http.StatusMovedPermanently))
}

// implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
