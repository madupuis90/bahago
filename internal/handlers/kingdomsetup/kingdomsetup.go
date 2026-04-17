package kingdomsetup

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	. "bahago/internal/layout"
	"bahago/internal/router"
	"bahago/internal/routes"
)

// ── Input struct ─────────────────────────────────────────────────────────────

type kingdomCreateForm struct {
	Name string `json:"kingdom_name"`
}

// ── Route registration ────────────────────────────────────────────────────────

func RegisterRoutes(r router.Router, queries *db.Queries) {
	h := &handler{queries: queries}
	r.HandleFunc("GET "+routes.KingdomSetupPath, h.handleSetupPage())
	r.HandleFunc("POST "+routes.KingdomCreatePath, h.handleCreateKingdom())
}

type handler struct {
	queries *db.Queries
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

		_, err := h.queries.CreateKingdom(r.Context(), db.CreateKingdomParams{
			UserID: user.ID,
			Name:   name,
		})
		if err != nil {
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
	return Div(Class("alert-error"),
		Map(errs, func(e error) Node {
			return P(Text(e.Error()))
		}),
	)
}
