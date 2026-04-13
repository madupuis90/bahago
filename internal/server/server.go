package server

import (
	"bahago/internal/database/db"
	"bahago/internal/email"
	"bahago/internal/game"
	"bahago/internal/handlers/allocation"
	"bahago/internal/handlers/auth"
	"bahago/internal/handlers/chat"
	"bahago/internal/handlers/home"
	"bahago/internal/handlers/kingdom"
	"bahago/internal/hub"
	"bahago/internal/middleware"
	"bahago/internal/router"
	"bahago/internal/routes"
	"bahago/web"
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	mux     *http.ServeMux
	pool    *pgxpool.Pool
	queries *db.Queries
	sender  *email.Sender
	appURL  string
	tickHub *hub.Hub
}

func New(pool *pgxpool.Pool, sender *email.Sender, appURL string) *Server {
	s := &Server{
		mux:     http.NewServeMux(),
		pool:    pool,
		queries: db.New(pool),
		sender:  sender,
		appURL:  appURL,
		tickHub: hub.New(),
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
	kingdom.RegisterRoutes(kingdomRouter, s.queries, s.tickHub)
	allocation.RegisterRoutes(kingdomRouter, s.queries, s.tickHub)

	// static assets — embedded into the binary at compile time
	s.mux.Handle("GET /static/", http.FileServer(http.FS(web.Static)))

	// redirect to home
	s.mux.Handle("/", http.RedirectHandler(routes.HomePath, http.StatusMovedPermanently))
}

// implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// StartGameTicker begins the game tick loop. It blocks until ctx is cancelled.
// Call as a goroutine from main.
func (s *Server) StartGameTicker(ctx context.Context) {
	game.StartTicker(ctx, s.pool, s.tickHub.Publish, 15*time.Second)
}
