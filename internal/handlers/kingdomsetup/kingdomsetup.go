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

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrNameRequired    = errors.New("kingdom name is required")
	ErrNameLettersOnly = errors.New("kingdom name may only contain letters")
	ErrNameTaken       = errors.New("that kingdom name is already taken")
	ErrMapFull         = errors.New("the map is full, please try again later")
)

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

		name, err := validateKingdomName(form.Name)
		if err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(kingdomAlert(AlertError(err)))
			return
		}

		if err := h.createKingdom(r.Context(), user.ID, name); err != nil {
			if isCreateKingdomUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(kingdomAlert(AlertError(err)))
				return
			}
			log.Printf("create kingdom: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(kingdomAlert(AlertError(errors.New("failed to create kingdom"))))
			return
		}

		sse := datastar.NewSSE(w, r)
		if err := sse.Redirect(routes.KingdomPath); err != nil {
			log.Printf("create kingdom: redirect: %v", err)
		}
	}
}

// ── Validation ────────────────────────────────────────────────────────────────

// validateKingdomName trims, checks the letters-only rule, and returns the
// title-cased name ready for insertion. Single-value parse shape: there is at
// most one rule that can fail (empty is "required", non-letters is its own
// error), so the (T, error) shape is the right one.
func validateKingdomName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", ErrNameRequired
	}
	for _, ch := range name {
		if !unicode.IsLetter(ch) {
			return "", ErrNameLettersOnly
		}
	}
	return cases.Title(language.English).String(name), nil
}

// ── Orchestration ─────────────────────────────────────────────────────────────

// createKingdom picks a free position and inserts the kingdom. Returns the
// ErrNameTaken sentinel for a unique-violation on the name; wraps other errors
// for logging.
func (h *handler) createKingdom(ctx context.Context, userID int, name string) error {
	x, y, err := pickFreePosition(ctx, h.queries)
	if err != nil {
		if errors.Is(err, ErrMapFull) {
			return ErrMapFull
		}
		return fmt.Errorf("pick position: %w", err)
	}

	if _, err := h.queries.CreateKingdom(ctx, db.CreateKingdomParams{
		UserID: userID,
		Name:   name,
		X:      x,
		Y:      y,
	}); err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
			return ErrNameTaken
		}
		return fmt.Errorf("create kingdom: %w", err)
	}
	return nil
}

func isCreateKingdomUserError(err error) bool {
	return errors.Is(err, ErrNameTaken) || errors.Is(err, ErrMapFull)
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
	return 0, 0, ErrMapFull
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
		kingdomAlert(nil),
	)
}

func kingdomAlert(inner Node) Node { return AlertContainer("kingdom-alert", inner) }
