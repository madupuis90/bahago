package server

import (
	"bahago/internal/database/db"
	"bahago/internal/email"
	"bahago/internal/game"
	"bahago/internal/handlers/allocation"
	"bahago/internal/handlers/army"
	"bahago/internal/handlers/auth"
	"bahago/internal/handlers/buildings"
	"bahago/internal/handlers/chat"
	"bahago/internal/handlers/guild"
	"bahago/internal/handlers/home"
	"bahago/internal/handlers/iconpreview"
	"bahago/internal/handlers/kingdom"
	"bahago/internal/handlers/kingdomsetup"
	"bahago/internal/handlers/layoutrefresh"
	"bahago/internal/handlers/messages"
	"bahago/internal/handlers/prayers"
	"bahago/internal/handlers/units"
	"bahago/internal/handlers/worldmap"
	"bahago/internal/hub"
	"bahago/internal/middleware"
	"bahago/internal/router"
	"bahago/web"
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const tickInterval = 120 * time.Second

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
	iconpreview.RegisterRoutes(globalRouter)
	home.RegisterRoutes(globalRouter)
	auth.RegisterRoutes(globalRouter, s.queries, s.pool, s.sender, s.appURL)
	chat.RegisterRoutes(globalRouter) // Experiment

	// routes requiring an authenticated user
	reqAuthRouter := globalRouter.Chain(middleware.RequireAuth)
	loadKingdomRouter := reqAuthRouter.Chain(middleware.LoadKingdom(s.queries))
	reqKingdomRouter := loadKingdomRouter.Chain(middleware.RequireKingdom)

	kingdomsetup.RegisterRoutes(loadKingdomRouter, s.queries)
	kingdom.RegisterRoutes(reqKingdomRouter, s.queries, s.tickHub)
	allocation.RegisterRoutes(reqKingdomRouter, s.queries, s.tickHub)
	buildings.RegisterRoutes(reqKingdomRouter, s.queries, s.pool, s.tickHub)
	units.RegisterRoutes(reqKingdomRouter, s.queries, s.pool, s.tickHub)
	army.RegisterRoutes(reqKingdomRouter, s.queries, s.pool, s.tickHub)
	worldmap.RegisterRoutes(reqKingdomRouter, s.queries)
	layoutrefresh.RegisterRoutes(reqKingdomRouter, s.queries, s.tickHub)
	messages.RegisterRoutes(reqKingdomRouter, s.queries, s.tickHub)
	prayers.RegisterRoutes(reqKingdomRouter, s.queries, s.pool, s.tickHub)
	guild.RegisterRoutes(reqKingdomRouter, s.queries, s.pool, s.tickHub)

	// static assets — embedded into the binary at compile time
	// Cache-Control is set explicitly because embed.FS has no ModTime, so
	// http.FileServer cannot emit Last-Modified or a reliable ETag, leaving
	// browsers with no freshness signal and forcing a full re-fetch every time.
	staticHandler := http.FileServer(http.FS(web.Static))
	s.mux.Handle("GET /static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		staticHandler.ServeHTTP(w, r)
	}))

}

// implements http.Handler
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// StartGameTicker begins the game tick loop. It blocks until ctx is cancelled.
// Call as a goroutine from main.
func (s *Server) StartGameTicker(ctx context.Context) {
	game.StartTicker(ctx, s.pool, s.tickHub.Publish, tickInterval)
}
