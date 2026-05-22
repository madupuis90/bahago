package ui

import (
	"fmt"
	"net/http"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/routes"

	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
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
	return shell(
		title,
		nil,
		homeTopNav(user, currentPath),
		Nav(Class("side-nav panel"), HomeSideNav(currentPath)),
		content...,
	)
}

// KingdomLayout renders a full page with the kingdom top-nav active and kingdom side-nav.
func KingdomLayout(r *http.Request, title string, currentPath string, kingdom *db.Kingdom, content ...Node) Node {
	user, _ := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser)

	var sideNav Node
	if kingdom != nil {
		sideNav = KingdomSideNav(currentPath, kingdom, 0)
	} else {
		sideNav = Nav(ID("kingdom-sidenav"), Class("side-nav panel"))
	}

	layoutStream := Div(ds.Init(GetSSENoSignals(routes.KingdomLayoutRefreshPath+"?path=%s", currentPath)))
	return shell(
		title,
		layoutStream,
		kingdomTopNav(user, currentPath),
		sideNav,
		content...,
	)
}

// MainContent wraps page content in the main element used by all kingdom pages.
// Use this in SSE handlers when patching page content with WithSelector("#main-content").
func MainContent(content ...Node) Node {
	return Main(ID("main-content"), Group(content))
}

// shell is the shared HTML document structure used by both layouts.
func shell(title string, layoutStream Node, topNav, sideNav Node, content ...Node) Node {
	return Doctype(
		HTML(
			Lang("en"),
			Head(
				TitleEl(Text(title)),
				Link(Rel("icon"), Href("data:,")),
				Link(Rel("stylesheet"), Href("/static/styles.css")),
				Script(Type("module"), Src("/static/datastar.js")),
			),
			Body(
				topNav,
				Div(Class("content-area"),
					sideNav,
					MainContent(content...),
				),
				Footer(),
				layoutStream,
			),
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

func kingdomTopNav(user *contextkeys.SessionUser, currentPath string) Node {
	return Header(Class("top-nav panel"),
		Div(Class("top-nav-left"),
			A(Href(routes.HomePath), Text("Home")),
			A(Href(routes.KingdomPath), Attr("aria-current", "page"), Text("Kingdom")),
		),
		Div(Class("top-nav-right"),
			LoginNav(user, currentPath),
		),
	)
}

// ── Nav components ────────────────────────────────────────────────────────────

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

// KingdomSideNav renders the kingdom sidebar navigation wrapped in its Nav element.
// unreadCount is the number of unread messages; 0 hides the badge.
func KingdomSideNav(currentPath string, kingdom *db.Kingdom, unreadCount int) Node {
	return Nav(ID("kingdom-sidenav"), Class("side-nav panel"),
		NavGroup("Kingdom",
			P(Span(Text(kingdom.Name))),
			P(Span(Text("Population: ")), Span(Text(fmt.Sprintf("%d", kingdom.Population)))),
		),
		NavGroup("Resources",
			P(Span(Text("Wood: ")), Span(Text(fmt.Sprintf("%d", kingdom.Wood)))),
			P(Span(Text("Stone: ")), Span(Text(fmt.Sprintf("%d", kingdom.Stone)))),
			P(Span(Text("Food: ")), Span(Text(fmt.Sprintf("%d", kingdom.Food)))),
			P(Span(Text("Mana: ")), Span(Text(fmt.Sprintf("%d", kingdom.Mana)))),
			P(Span(Text("Devotion: ")), Span(Text(fmt.Sprintf("%d", kingdom.Devotion)))),
			P(Span(Text("Knowledge: ")), Span(Text(fmt.Sprintf("%d", kingdom.Knowledge)))),
		),
		NavGroup("Kingdom",
			NavItem(routes.KingdomPath, "Overview", currentPath),
			NavItem(routes.KingdomAllocationPath, "Allocation", currentPath),
			NavItem(routes.KingdomBuildingsPath, "Buildings", currentPath),
			NavItem(routes.KingdomUnitsPath, "Units", currentPath),
			NavItem(routes.KingdomArmyPath, "Army", currentPath),
			NavItem(routes.KingdomMapPath, "World Map", currentPath),
			NavItem(routes.KingdomPrayersPath, "Prayers", currentPath),
			messagesNavItem(currentPath, unreadCount),
			NavItem(routes.GuildPath, "Guild", currentPath),
		),
	)
}

func messagesNavItem(currentPath string, unreadCount int) Node {
	return Span(Class("nav-item-with-badge"),
		NavItem(routes.KingdomMessagesPath, "Messages", currentPath),
		Iff(unreadCount > 0, func() Node {
			return Span(Class("nav-badge"), Text(fmt.Sprintf("%d", unreadCount)))
		}),
	)
}
