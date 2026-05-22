package kingdomsetup

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"net/http"
	"strings"
	"unicode"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/starfederation/datastar-go/datastar"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/game"
	. "bahago/internal/ui"
	"bahago/internal/router"
	"bahago/internal/routes"
)

// ── Input struct ─────────────────────────────────────────────────────────────

type kingdomCreateForm struct {
	Name string `json:"kingdom_name"`
}

// ── Route registration ────────────────────────────────────────────────────────

func RegisterRoutes(r router.Router, queries db.Querier) {
	h := &handler{queries: queries}
	r.HandleFunc("GET "+routes.KingdomSetupPath, h.handleSetupPage())
	r.HandleFunc("POST "+routes.KingdomCreatePath, h.handleCreateKingdom())
}

type handler struct {
	queries db.Querier
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handleSetupPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// If user already has a kingdom, send them to the kingdom page.
		if kingdom, ok := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom); ok && kingdom != nil {
			http.Redirect(w, r, routes.KingdomPath, http.StatusSeeOther)
			return
		}
		KingdomLayout(r, "Found Your Kingdom", r.URL.Path, nil, setupContent()).Render(w)
	}
}

func (h *handler) handleCreateKingdom() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _ := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser)

		form := &kingdomCreateForm{}
		if err := datastar.ReadSignals(r, form); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(form.Name)
		if name == "" {
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("kingdom name is required")})))
			return
		}
		for _, ch := range name {
			if !unicode.IsLetter(ch) {
				datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("kingdom name may only contain letters")})))
				return
			}
		}
		name = cases.Title(language.English).String(name)

		x, y, err := pickFreePosition(r.Context(), h.queries)
		if err != nil {
			log.Printf("create kingdom: pick position: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("failed to find a position on the map")})))
			return
		}

		if _, err := h.queries.CreateKingdom(r.Context(), db.CreateKingdomParams{
			UserID: user.ID,
			Name:   name,
			X:      x,
			Y:      y,
		}); err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
				datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("that kingdom name is already taken")})))
				return
			}
			log.Printf("create kingdom: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(alertComponent(errorComponent([]error{errors.New("failed to create kingdom")})))
			return
		}

		sse := datastar.NewSSE(w, r)
		if err := sse.Redirect(routes.KingdomPath); err != nil {
			log.Printf("create kingdom: redirect: %v", err)
		}
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// pickFreePosition finds a free (x, y) tile for a new kingdom. It loads all
// currently occupied positions into a set, then samples from a normal
// distribution centred on WorldSize/2 with a sigma that grows with population
// (sqrt-scaled, floored at 5) until it finds a tile that is free.
// One DB round-trip; no error-code inspection; no retry on DB errors.
func pickFreePosition(ctx context.Context, queries db.Querier) (int, int, error) {
	const centre = float64(game.WorldSize) / 2

	taken, err := queries.GetKingdomsInViewport(ctx, db.GetKingdomsInViewportParams{
		X:   0,
		X_2: game.WorldSize - 1,
		Y:   0,
		Y_2: game.WorldSize - 1,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("load occupied positions: %w", err)
	}

	occupied := make(map[game.Coord]struct{}, len(taken))
	for _, k := range taken {
		occupied[game.Coord{X: k.X, Y: k.Y}] = struct{}{}
	}

	// sigma grows with sqrt(population) so early players cluster tightly near the
	// centre while the map expands outward as new kingdoms join. The floor of 5
	// prevents the initial cluster from being so small that free tiles are hard
	// to find. At ~1000 kingdoms sigma≈19, which covers the full 64-tile map.
	sigma := math.Max(5.0, math.Sqrt(float64(len(taken)))*0.6)

	clamp := func(v float64) int {
		n := int(math.Round(centre + sigma*v))
		if n < 0 {
			return 0
		}
		if n > game.WorldSize-1 {
			return game.WorldSize - 1
		}
		return n
	}

	// Safety cap: exits on the first try in all practical conditions.
	// The central zone alone has ~14 000 tiles; this only fails once
	// essentially all of them are taken.
	for range 10_000 {
		x, y := clamp(rand.NormFloat64()), clamp(rand.NormFloat64())
		if _, ok := occupied[game.Coord{X: x, Y: y}]; !ok {
			return x, y, nil
		}
	}
	return 0, 0, errors.New("world map is full")
}

// ── Components ────────────────────────────────────────────────────────────────

func setupContent() Node {
	return Div(Class("auth-card panel"),
		H1(Text("Found Your Kingdom")),
		P(Text("Give your kingdom a name to begin your reign.")),
		Div(Class("form-fields"),
			Label(
				Text("Kingdom Name"),
				Input(
					Type("text"),
					ds.Bind("kingdom_name"),
					Placeholder("Enter your kingdom name"),
				),
			),
		),
		Button(Class("btn"),
			Type("button"),
			ds.On("click", datastar.PostSSE(routes.KingdomCreatePath)),
			Text("Create Kingdom"),
		),
		alertComponent(nil),
	)
}

func alertComponent(inner Node) Node {
	return Div(ID("kingdom-alert"), inner)
}

func errorComponent(errs []error) Node {
	if len(errs) == 0 {
		return nil
	}
	return Div(Class("alert--error"),
		Map(errs, func(e error) Node {
			return P(Text(e.Error()))
		}),
	)
}
