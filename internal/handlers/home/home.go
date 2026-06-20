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
				A(Class("auth-btn"), Style("width:auto"), Href(routes.RegisterPath), Text("Found a Kingdom")),
				Span(Class("home-signin-prompt"),
					Text("Already a ruler? "),
					A(Href(routes.LoginPath), Text("Sign in →")),
				),
			),
			homeHeroScene(),
		),
		Div(Class("home-cards"),
			realmStatusCard(),
			dispatchesCard(),
		),
	})
}

// homeHeroScene is the additive decorative SVG band for the hero: a flat
// ligne-claire landscape (meadow + castle + banners) with a sun, distant
// birds, and a "FREE to play" starburst. Verbatim from the Comic Lab proof;
// the hero reads fine without it. All strokes inherit var(--ink) via the
// .home-hero-* CSS.
func homeHeroScene() Node {
	return Group([]Node{
		Raw(`<svg class="home-hero-sun" viewBox="0 0 80 80" fill="none" aria-hidden="true">
  <circle cx="40" cy="40" r="16" fill="#f4c12e" stroke-width="3"></circle>
  <g stroke-width="3" stroke-linecap="round">
    <line x1="40" y1="6" x2="40" y2="16"></line><line x1="40" y1="64" x2="40" y2="74"></line>
    <line x1="6" y1="40" x2="16" y2="40"></line><line x1="64" y1="40" x2="74" y2="40"></line>
    <line x1="16" y1="16" x2="23" y2="23"></line><line x1="57" y1="57" x2="64" y2="64"></line>
    <line x1="16" y1="64" x2="23" y2="57"></line><line x1="57" y1="23" x2="64" y2="16"></line>
  </g>
</svg>`),
		Raw(`<svg class="home-hero-birds" viewBox="0 0 92 36" fill="none" aria-hidden="true" stroke-width="2.4" stroke-linecap="round">
  <path d="M4 16 Q12 6 20 16 Q28 6 36 16"></path>
  <path d="M50 26 Q56 18 62 26 Q68 18 74 26"></path>
</svg>`),
		Div(Class("home-hero-burst"),
			Raw(`<svg viewBox="0 0 100 100" fill="none" aria-hidden="true">
  <path d="M50 3 L59 20 L78 13 L74 33 L94 38 L80 52 L95 66 L75 70 L78 90 L60 81 L50 98 L40 81 L22 90 L25 70 L5 66 L20 52 L6 38 L26 33 L22 13 L41 20 Z" fill="#3f9d51" stroke-width="3" stroke-linejoin="round"></path>
</svg>`),
			Span(Class("home-hero-burst-txt"),
				El("b", Text("FREE")),
				Span(Text("to play")),
			),
		),
		Raw(`<svg class="home-hero-scene" viewBox="0 0 1200 230" fill="none" aria-hidden="true">
  <path d="M0 150 C 180 116 360 132 560 124 C 780 116 1000 142 1200 116 L1200 230 L0 230 Z" fill="#9ad29a" stroke-width="3"></path>
  <path d="M0 188 C 240 166 420 198 700 176 C 940 158 1080 190 1200 168 L1200 230 L0 230 Z" fill="#4aa85d" stroke-width="3"></path>
  <g stroke-width="3" stroke-linejoin="round">
    <line x1="300" y1="152" x2="300" y2="178" stroke="#3a2a18" stroke-linecap="round"></line>
    <path d="M300 114 L323 160 L277 160 Z" fill="#6abf76"></path>
    <line x1="1010" y1="150" x2="1010" y2="174" stroke="#3a2a18" stroke-linecap="round"></line>
    <path d="M1010 116 L1030 158 L990 158 Z" fill="#6abf76"></path>
  </g>
  <g stroke-width="3" stroke-linejoin="round">
    <path d="M470 188 L470 120 L506 120 L506 188 Z" fill="#cdd3d8"></path>
    <path d="M470 120 L470 110 L478 110 L478 118 L484 118 L484 110 L492 110 L492 118 L498 118 L498 110 L506 110 L506 120 Z" fill="#cdd3d8"></path>
    <path d="M610 188 L610 120 L646 120 L646 188 Z" fill="#cdd3d8"></path>
    <path d="M610 120 L610 110 L618 110 L618 118 L624 118 L624 110 L632 110 L632 118 L638 118 L638 110 L646 110 L646 120 Z" fill="#cdd3d8"></path>
    <path d="M506 188 L506 96 L610 96 L610 188 Z" fill="#e7ebee"></path>
    <path d="M506 96 L506 84 L516 84 L516 94 L524 94 L524 84 L534 84 L534 94 L542 94 L542 84 L552 84 L552 94 L564 94 L564 84 L574 94 L574 84 L584 84 L584 94 L592 94 L592 84 L602 84 L602 94 L610 94 L610 96 Z" fill="#e7ebee"></path>
    <path d="M540 188 L540 150 Q558 134 576 150 L576 188 Z" fill="#8a6a3f"></path>
    <rect x="482" y="138" width="12" height="18" rx="1.5" fill="#5a6b78"></rect>
    <rect x="622" y="138" width="12" height="18" rx="1.5" fill="#5a6b78"></rect>
    <line x1="488" y1="110" x2="488" y2="86" stroke-linecap="round"></line>
    <path d="M488 88 L506 93 L488 98 Z" fill="#d63a2f"></path>
    <line x1="628" y1="110" x2="628" y2="86" stroke-linecap="round"></line>
    <path d="M628 88 L646 93 L628 98 Z" fill="#2f6fb0"></path>
    <line x1="558" y1="84" x2="558" y2="56" stroke-linecap="round"></line>
    <path d="M558 58 L582 64 L558 70 Z" fill="#f4c12e"></path>
  </g>
  <g stroke-width="3" stroke-linejoin="round" fill="#3f9d51">
    <path d="M560 212 Q572 196 586 204 Q598 194 610 206 Z"></path>
    <path d="M700 210 Q710 192 724 200 Q736 188 750 200 Q764 196 762 210 Z"></path>
  </g>
</svg>`),
	})
}

func realmStatusCard() Node {
	return Div(Class("panel"),
		Span(Class("panel-title panel-title--red"), Text("Realm Status")),
		Div(Class("ci"),
			H3(Class("c-hed"), Text("Round I · Dawn of the Realm")),
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
	return Div(Class("panel"),
		Span(Class("panel-title panel-title--blue"), Text("Latest Dispatches")),
		Div(Class("ci"),
			Div(Class("news"),
				Div(Class("news-item"),
					Span(Class("news-date"), Text("14 Jun 2026")),
					Div(Class("news-head"), Text("The Realm Opens")),
					P(Class("news-body"), Text("New kingdoms are being founded. The realm stirs and ancient powers await.")),
				),
			),
		),
	)
}
