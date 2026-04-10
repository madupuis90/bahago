package kingdom

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/router"
	"bahago/internal/routes"
	. "bahago/internal/ui"
)

// ── Signal definitions ────────────────────────────────────────────────────────

type kingdomCreateForm struct {
	Name Signal[string] `json:"kingdom_name"`
}

var createFormSignals = NewSignalDef[kingdomCreateForm]()

// ── Component IDs ─────────────────────────────────────────────────────────────

const kingdomCreateErrorID = "kingdom-create-error"

// ── Route registration ────────────────────────────────────────────────────────

func RegisterSetupRoutes(r router.Router, queries *db.Queries) {
	h := newHandler(queries)

	r.HandleFunc("GET "+routes.KingdomPath, h.handleKingdomPage())
	r.HandleFunc("POST "+routes.KingdomCreatePath, h.handleCreateKingdom())
}

func RegisterRoutes(r router.Router, queries *db.Queries) {
	h := newHandler(queries)

	r.HandleFunc("GET "+routes.KingdomAllocationPath, h.handleAllocationPage())
	r.HandleFunc("POST "+routes.KingdomAllocationSavePath, h.handleSaveAllocation())
}

type handler struct {
	queries *db.Queries
}

func newHandler(queries *db.Queries) *handler {
	return &handler{queries: queries}
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handleKingdomPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom, _ := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		title := "Found Your Kingdom"
		if kingdom != nil {
			title = kingdom.Name
		}
		NewPage(title, AppLayout(r), kingdomContent(kingdom)).Render(w)
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

		name := strings.TrimSpace(form.Name.Value)
		if name == "" {
			datastar.NewSSE(w, r).PatchElementGostar(kingdomCreateErrorComponent("Kingdom name is required"))
			return
		}

		_, err := h.queries.CreateKingdom(r.Context(), db.CreateKingdomParams{
			UserID: user.ID,
			Name:   name,
		})
		if err != nil {
			log.Printf("create kingdom: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(kingdomCreateErrorComponent("Failed to create kingdom"))
			return
		}
		sse := datastar.NewSSE(w, r)
		if err := sse.Redirect(routes.KingdomPath); err != nil {
			log.Printf("create kingdom: redirect: %v", err)
		}
	}
}

// ── Pages ─────────────────────────────────────────────────────────────────────

func kingdomContent(kingdom *db.Kingdom) Node {
	if kingdom == nil {
		return kingdomCreateSection(createFormSignals.New())
	}
	return kingdomOverviewSection(kingdom)
}

// ── Components ────────────────────────────────────────────────────────────────

func kingdomCreateSection(sigs kingdomCreateForm) Node {
	return Div(
		ds.Signals(SignalMap(sigs)),
		H1(Text("Found Your Kingdom")),
		P(Text("Give your kingdom a name to begin your reign.")),
		Div(ID(kingdomCreateErrorID)),
		Label(For("kingdom-name-input"), Text("Kingdom Name")),
		Input(
			ID("kingdom-name-input"),
			Type("text"),
			ds.Bind(sigs.Name.Key),
			Placeholder("Enter your kingdom name"),
		),
		Button(Class("btn"),
			Type("button"),
			ds.On("click", datastar.PostSSE(routes.KingdomCreatePath)),
			Text("Create Kingdom"),
		),
	)
}

func kingdomCreateErrorComponent(msg string) Node {
	return Div(ID(kingdomCreateErrorID), Text(msg))
}

func kingdomOverviewSection(kingdom *db.Kingdom) Node {
	return Div(
		H1(Text(kingdom.Name)),
		P(Text(fmt.Sprintf("Population: %d", kingdom.Population))),
	)
}
