package home

import (
	"net/http"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	. "bahago/internal/ui"
	"bahago/internal/router"
	"bahago/internal/routes"
)

func RegisterRoutes(r router.Router) {
	h := newHandler()
	r.HandleFunc("GET /", h.handleRoot())
	r.HandleFunc("GET "+routes.HomePath, h.handleHomePage())
}

type handler struct {
}

func newHandler() *handler {
	return &handler{}
}

func (h *handler) handleRoot() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, _ := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser)
		if user != nil {
			http.Redirect(w, r, routes.KingdomPath, http.StatusFound)
			return
		}
		http.Redirect(w, r, routes.HomePath, http.StatusFound)
	}
}

func (h *handler) handleHomePage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		HomeLayout(r, "Home", homeContent()).Render(w)
	}
}

func homeContent() Node {
	return Group([]Node{
		Section(Class("home-hero"),
			P(Class("home-kicker"), Text("✧ A realm awaits your rule")),
			H1(Class("home-title"), Text("Bahago")),
			P(Class("home-sub"), Text("Command a kingdom. Gather ancient resources, raise mighty armies, forge alliances in guilds, and carve your name into the annals of the realm.")),
			Div(Class("home-cta"),
				A(Class("btn btn--primary"), Href(routes.RegisterPath), Text("Found a Kingdom")),
			),
			P(Class("home-signin-prompt"),
				Text("Already of the realm? "),
				A(Href(routes.LoginPath), Text("Sign in →")),
			),
		),
		Hr(Class("home-rule")),
		Div(Class("home-cards"),
			realmStatusCard(),
			dispatchesCard(),
		),
	})
}

func realmStatusCard() Node {
	return Div(Class("card is-lit"),
		Div(Class("ci"),
			Div(Class("c-eye"), Text("Realm Status")),
			Div(Class("c-hed"), Text("Round I · Dawn of the Realm")),
			P(Class("c-p"), Text("The realm stirs. New kingdoms rise from the soil. Alliances form and ancient grudges ignite.")),
			Div(Class("stat-row"),
				Div(Class("stat"), Div(Class("stat-n"), Text("0")), Div(Class("stat-l"), Text("Kingdoms"))),
				Div(Class("stat"), Div(Class("stat-n"), Text("0")), Div(Class("stat-l"), Text("Guilds"))),
				Div(Class("stat"), Div(Class("stat-n"), Text("Day 1")), Div(Class("stat-l"), Text("of the Round"))),
			),
		),
	)
}

func dispatchesCard() Node {
	return Div(Class("card is-lit"),
		Div(Class("ci"),
			Div(Class("c-eye"), Text("Latest Dispatches")),
			Div(Class("news-item"),
				Div(Class("news-date"), Text("14 Jun 2026")),
				Div(Class("news-head"), Text("The Realm Opens")),
				Div(Class("news-body"), Text("New kingdoms are being founded. The realm stirs and ancient powers await.")),
			),
		),
	)
}
