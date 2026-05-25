package ui

import (
	"fmt"
	"net/http"
	"strconv"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/routes"

	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/components"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"
)

// GetSSENoSignals generates a datastar @get() action that sends no signals.
// Use this for refresh/subscribe endpoints that do not read signals from the request.
// openWhenHidden:true prevents datastar from adding a visibilitychange listener that
// aborts and reconnects the stream on tab focus changes — a pattern that causes Firefox
// to throw "Error in input stream" on the reconnect attempt.
func GetSSENoSignals(urlFormat string, args ...any) string {
	return fmt.Sprintf(`@get('%s', {openWhenHidden: true, filterSignals: {include: /^$/}})`, fmt.Sprintf(urlFormat, args...))
}

// ── Layout functions ──────────────────────────────────────────────────────────

// HomeLayout renders a full page with the home top-nav active and home side-nav.
func HomeLayout(r *http.Request, title string, content ...Node) Node {
	user, _ := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser)
	currentPath := r.URL.Path
	return shell(title, nil,
		homeTopNav(user, currentPath),
		Div(Class("content-area"),
			Nav(Class("side-nav panel"), HomeSideNav(currentPath)),
			MainContent(content...),
		),
		Footer(),
	)
}

// KingdomLayout renders a full page with the parchment topbar and bottom nav.
// currentPath drives which bottom-nav stone is highlighted.
func KingdomLayout(r *http.Request, title string, currentPath string, kingdom *db.Kingdom, content ...Node) Node {
	layoutStream := Div(ds.Init(GetSSENoSignals(routes.KingdomLayoutRefreshPath+"?path=%s", currentPath)))
	return shell(title, layoutStream,
		Div(Class("kingdom-page"),
			KingdomTopbar(kingdom),
			MainContent(content...),
			KingdomBottomNav(currentPath, 0),
		),
	)
}

// MainContent wraps page content in the main element used by all kingdom pages.
// Use this in SSE handlers when patching page content with WithSelector("#main-content").
func MainContent(content ...Node) Node {
	return Main(ID("main-content"), Group(content))
}

// shell is the shared HTML document structure. Body children are rendered in
// order; layoutStream is appended at the very end (kept separate so SSE init
// markers don't visually interleave with chrome).
func shell(title string, layoutStream Node, body ...Node) Node {
	bodyChildren := append([]Node{}, body...)
	if layoutStream != nil {
		bodyChildren = append(bodyChildren, layoutStream)
	}
	return Doctype(
		HTML(
			Lang("en"),
			Head(
				TitleEl(Text(title)),
				Link(Rel("icon"), Href("data:,")),
				Link(Rel("preconnect"), Href("https://fonts.googleapis.com")),
				Link(Rel("preconnect"), Href("https://fonts.gstatic.com"), Attr("crossorigin", "")),
				Link(Rel("stylesheet"), Href("https://fonts.googleapis.com/css2?family=Cormorant+Garamond:ital,wght@0,400;0,500;0,600;1,400;1,500&family=IM+Fell+English:ital@0;1&family=IM+Fell+English+SC&display=swap")),
				Link(Rel("stylesheet"), Href("/static/styles.css")),
				Script(Type("module"), Src("/static/datastar.js")),
			),
			Body(bodyChildren...),
		),
	)
}

func homeTopNav(user *contextkeys.SessionUser, currentPath string) Node {
	return Header(Class("top-nav panel"),
		Div(Class("top-nav-left"),
			A(Href(routes.HomePath), Attr("aria-current", "page"), Text("Home")),
			A(Href(routes.KingdomPath), Text("Kingdom")),
		),
		Div(Class("top-nav-right"),
			LoginNav(user, currentPath),
		),
	)
}

// ── Kingdom chrome (topbar + bottom nav) ──────────────────────────────────────

// KingdomTopbar renders the parchment topbar: realm badge + tick chip + 6 resource cartouches.
// Exported so SSE handlers can re-render it on tick.
func KingdomTopbar(kingdom *db.Kingdom) Node {
	if kingdom == nil {
		return Header(ID("kingdom-topbar"), Class("topbar"))
	}
	return Header(ID("kingdom-topbar"), Class("topbar"),
		Div(Class("topbar-row"),
			Div(Class("kingdom-badge"),
				Shield("crown", 36, false),
				Div(
					Div(Class("kingdom-name"), Text(kingdom.Name)),
					Div(Class("kingdom-sub text-muted"), Text(formatPopulation(kingdom.Population))),
				),
			),
			Div(Class("tick"),
				Div(Class("tick-chip"),
					Hourglass(9),
					Span(Class("caps-label"), Text("next tick")),
					Span(Class("tick-value"), Text("--:--")),
				),
			),
		),
		Div(Class("resources"),
			resourceCartouche("tree", "Wood", kingdom.Wood),
			resourceCartouche("mountain", "Stone", kingdom.Stone),
			resourceCartouche("wheat", "Food", kingdom.Food),
			resourceCartouche("flame", "Mana", kingdom.Mana),
			resourceCartouche("sun", "Devotion", kingdom.Devotion),
			resourceCartouche("star", "Knowledge", kingdom.Knowledge),
		),
	)
}

func resourceCartouche(shieldID, label string, value int) Node {
	return Div(Class("resource"),
		Shield(shieldID, 22, false),
		Div(Class("resource-text"),
			Span(Class("resource-label"), Text(label)),
			Span(Class("resource-value"), Text(FormatThousands(value))),
		),
	)
}

// stoneRoute pairs a bottom-nav stone with its destination.
type stoneRoute struct {
	label  string
	shield string
	href   string
}

var kingdomStones = []stoneRoute{
	{"Kingdom", "crown", routes.KingdomPath},
	{"Allocate", "sliders", routes.KingdomAllocationPath},
	{"Builds", "house", routes.KingdomBuildingsPath},
	{"Units", "person", routes.KingdomUnitsPath},
	{"Campaign", "swords", routes.KingdomArmyPath},
	{"World", "globe", routes.KingdomMapPath},
	{"Prayers", "cross", routes.KingdomPrayersPath},
	{"Messages", "envelope", routes.KingdomMessagesPath},
	{"Guild", "flag", routes.GuildPath},
}

// KingdomBottomNav renders the 9-stone bottom navigation. currentPath selects
// the active stone. unreadCount drives the "Scrolls" badge (0 hides it).
func KingdomBottomNav(currentPath string, unreadCount int) Node {
	stones := make([]Node, 0, len(kingdomStones))
	for _, s := range kingdomStones {
		stones = append(stones, navStone(s, currentPath, unreadCount))
	}
	return Nav(ID("kingdom-bottom-nav"), Class("bottom-nav"),
		Div(Class("bottom-nav-row"), Group(stones)),
	)
}

func navStone(s stoneRoute, currentPath string, unreadCount int) Node {
	active := currentPath == s.href
	cls := "nav-stone"
	if active {
		cls = "nav-stone nav-stone--active"
	}
	var badge Node
	if s.href == routes.KingdomMessagesPath && unreadCount > 0 {
		badge = Span(Class("nav-stone-badge"), Text(strconv.Itoa(unreadCount)))
	}
	return A(Href(s.href), Class(cls),
		Shield(s.shield, 32, active),
		Span(Class("nav-stone-label"), Text(s.label)),
		badge,
	)
}

// ── Home nav helpers (unchanged) ──────────────────────────────────────────────

func NavItem(href, name, currentPath string) Node {
	return A(Href(href), If(currentPath == href, Attr("aria-current", "page")), Text(name))
}

func NavGroup(name string, navItems ...Node) Node {
	return Div(Class("nav-group"),
		Div(P(Class("nav-group-name"), Text(name))),
		Div(Class("nav-group-content"), Group(navItems)),
	)
}

func LoginNav(user *contextkeys.SessionUser, currentPath string) Node {
	if user == nil {
		return Group([]Node{
			NavItem(routes.LoginPath, "Login", currentPath),
			NavItem(routes.RegisterPath, "Register", currentPath),
		})
	}
	return A(ds.On("click", datastar.PostSSE(routes.LogoutPath)), Text("Logout"))
}

// URLs are placeholder for now, no need to create routes
func HomeSideNav(currentPath string) Node {
	return Group([]Node{
		NavGroup("Active Players",
			P(Text("40")),
		),
		NavGroup("Lore",
			NavItem("/beginning", "The beginning", ""),
			NavItem("/state", "State of the World", ""),
		),
		NavGroup("Resources",
			NavItem("/how-to", "How to Play", ""),
			NavItem("/rules", "Rules", ""),
			NavItem("/tech-tree", "Tech. Tree", ""),
			NavItem("/units", "Units", ""),
		),
		NavGroup("Community",
			NavItem("/discord", "Discord", ""),
			NavItem(routes.ChatPath, "Chat", currentPath),
			NavItem("/about", "About", ""),
		),
	})
}

// ── Parchment page helpers ────────────────────────────────────────────────────

// PageHeader renders the page header: optional accent tag, italic H1, double rule.
// An empty tag omits the accent line.
func PageHeader(tag, body string) Node {
	return Div(Class("page-header"),
		If(tag != "", Div(Classes{"caps-label": true, "text-highlight": true, "page-header-tag": true}, Text(tag))),
		H1(Span(Class("italic"), Text(body))),
		Div(Class("rule-dbl")),
	)
}

// ── Formatting helpers ────────────────────────────────────────────────────────

func formatPopulation(n int) string {
	return FormatThousands(n) + " population"
}

// FormatThousands renders an integer with comma separators (e.g. 1028 → "1,028").
func FormatThousands(n int) string {
	if n < 0 {
		return "-" + FormatThousands(-n)
	}
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	first := len(s) % 3
	if first == 0 {
		first = 3
	}
	out := s[:first]
	for i := first; i < len(s); i += 3 {
		out += "," + s[i:i+3]
	}
	return out
}
