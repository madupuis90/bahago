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
		Footer(Text("✧ Bahago · All rights reserved")),
	)
}

// AuthLayout renders an auth page with the home chrome and side nav.
func AuthLayout(r *http.Request, title string, content ...Node) Node {
	user, _ := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser)
	currentPath := r.URL.Path
	return shell(title, nil,
		homeTopNav(user, currentPath),
		Div(Class("content-area"),
			Nav(Class("side-nav panel"), HomeSideNav(currentPath)),
			Main(ID("main-content"), Class("auth-stage"), Group(content)),
		),
		Footer(Text("✧ Bahago · All rights reserved")),
	)
}

// KingdomLayout renders a full page with the CommandBar chrome.
// currentPath drives which nav link is highlighted and is forwarded to the layout refresh SSE stream.
func KingdomLayout(r *http.Request, title string, currentPath string, kingdom *db.Kingdom, content ...Node) Node {
	layoutStream := Div(ds.Init(GetSSENoSignals(routes.KingdomLayoutRefreshPath+"?path=%s", currentPath)))
	return shell(title, layoutStream,
		Div(Class("kingdom-page"),
			KingdomTopbar(kingdom, currentPath, 0),
			MainContent(content...),
		),
	)
}

// MainContent wraps page content in the main element used by all kingdom pages.
// Use this in SSE handlers when patching page content with WithSelector("#main-content").
func MainContent(content ...Node) Node {
	return Main(ID("main-content"), Group(content))
}

// shell is the shared HTML document structure. Glyph symbols live in the
// external sprite at /static/sprite.svg (referenced via <use href>), so no
// inline <defs> block is needed on each page.
func shell(title string, layoutStream Node, body ...Node) Node {
	bodyChildren := body
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
				Link(Rel("stylesheet"), Href("https://fonts.googleapis.com/css2?family=Lilita+One&family=Nunito:ital,wght@0,400;0,600;0,700;0,800;0,900;1,600;1,700&display=swap")),
				Link(Rel("stylesheet"), Href("/static/styles.css")),
				Script(Type("module"), Src("/static/datastar.js")),
			),
			Body(bodyChildren...),
		),
	)
}

func homeTopNav(user *contextkeys.SessionUser, currentPath string) Node {
	var rightContent Node
	if user == nil {
		rightContent = Group([]Node{
			A(Class("home-chrome-register"), Href(routes.RegisterPath), Text("Join the Realm")),
			A(Class("home-chrome-login"), Href(routes.LoginPath), Text("Sign In")),
		})
	} else {
		rightContent = A(Class("home-chrome-login"),
			ds.On("click", datastar.PostSSE(routes.LogoutPath)),
			Text("Leave"),
		)
	}
	return Header(Class("home-chrome bar"),
		A(Class("home-chrome-brand"), Href(routes.HomePath),
			Crest("crown", 38, "home-chrome-crest"),
			Span(Class("home-chrome-name"), Text("Bahago")),
		),
		Span(Class("home-chrome-sep")),
		Nav(Class("home-chrome-nav"),
			A(Classes{"nav-link": true, "is-on": currentPath == routes.HomePath},
				Href(routes.HomePath), Text("Home")),
			A(Classes{"nav-link": true},
				Href(routes.KingdomPath),
				If(user == nil, Attr("aria-disabled", "true")),
				Text("Kingdom")),
		),
		Div(Class("home-chrome-right"), rightContent),
	)
}

// ── Kingdom chrome (CommandBar) ───────────────────────────────────────────────

// KingdomTopbar renders the unified CommandBar. Exported for SSE re-render on tick.
func KingdomTopbar(kingdom *db.Kingdom, currentPath string, msgCount int) Node {
	if kingdom == nil {
		return Header(ID("kingdom-topbar"), Classes{"bar": true, "barB2": true})
	}
	return Header(ID("kingdom-topbar"), Classes{"bar": true, "barB2": true},
		Div(Class("barB2-info"),
			commandBarIdentity(kingdom),
			Div(Class("barB2-res"),
				resourcePill("tree", "Wood", kingdom.Wood),
				resourcePill("mountain", "Stone", kingdom.Stone),
				resourcePill("wheat", "Food", kingdom.Food),
				resourcePill("flame", "Mana", kingdom.Mana),
				resourcePill("sun", "Devotion", kingdom.Devotion),
				resourcePill("star", "Lore", kingdom.Knowledge),
			),
			Div(Class("barB2-right"),
				commandBarTick(),
				commandBarLeave(),
			),
		),
		commandBarNav(currentPath, msgCount),
	)
}

func commandBarIdentity(kingdom *db.Kingdom) Node {
	return Div(Class("id"),
		Crest("", 40, "id-crest"),
		Div(
			Div(Class("id-name"), Text(kingdom.Name)),
			Div(Class("id-sub"), Text(formatPopulation(kingdom.Population))),
		),
	)
}

func commandBarTick() Node {
	return Span(Class("tick"),
		Icon("sandglass", 9, false),
		Span(Class("tick-l"), Text("Tick")),
		Span(Class("tick-v"), Text("--:--")),
	)
}

func commandBarLeave() Node {
	return A(Class("leave"), Href(routes.HomePath), Attr("title", "Leave the kingdom"),
		Raw(`<svg width="15" height="15" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M6.5 2.5 H3.4 A1 1 0 0 0 2.4 3.5 V12.5 A1 1 0 0 0 3.4 13.5 H6.5"/><path d="M10 5 L13 8 L10 11"/><path d="M13 8 H6.4"/></svg>`),
		Span(Class("leave-l"), Text("Leave")),
	)
}

func resourcePill(id, label string, value int) Node {
	return Span(Class("pill"),
		ResourceGem(id, 30),
		Span(Class("pill-txt"),
			Span(Class("pill-l"), Text(label)),
			Span(Classes{"pill-v": true, "is-zero": value == 0}, Text(FormatThousands(value))),
		),
	)
}

type navItem struct {
	label string
	href  string
}

var kingdomNavItems = []navItem{
	{"Kingdom", routes.KingdomPath},
	{"Allocate", routes.KingdomAllocationPath},
	{"Builds", routes.KingdomBuildingsPath},
	{"Units", routes.KingdomUnitsPath},
	{"Campaign", routes.KingdomArmyPath},
	{"World", routes.KingdomMapPath},
	{"Prayers", routes.KingdomPrayersPath},
	{"Messages", routes.KingdomMessagesPath},
	{"Guild", routes.GuildPath},
}

func commandBarNav(currentPath string, msgCount int) Node {
	links := make([]Node, len(kingdomNavItems))
	for i, item := range kingdomNavItems {
		isMessages := item.label == "Messages"
		badgeNode := Iff(isMessages && msgCount > 0, func() Node {
			badgeText := "99+"
			if msgCount <= 99 {
				badgeText = strconv.Itoa(msgCount)
			}
			return Span(Class("nav-badge"), Text(badgeText))
		})
		links[i] = A(
			Classes{"nav-link": true, "is-on": currentPath == item.href, "is-alert": isMessages && msgCount > 0},
			Href(item.href),
			If(badgeNode != nil, badgeNode),
			Span(Class("nav-link-l"), Text(item.label)),
		)
	}
	return Nav(Class("barB2-nav"), Group(links))
}

// ── Home nav helpers ──────────────────────────────────────────────────────────

func NavItem(href, name, currentPath string) Node {
	return A(Href(href), If(currentPath == href, Attr("aria-current", "page")), Text(name))
}

func NavGroup(name string, items ...Node) Node {
	return Div(Class("nav-group"),
		Span(Class("nav-group-name"), Text(name)),
		Div(Class("nav-group-content"), Group(items)),
	)
}

// URLs are placeholder for now, no need to create routes
func HomeSideNav(currentPath string) Node {
	return Group([]Node{
		Div(Class("nav-live"),
			Span(Class("nav-live-dot")),
			Div(
				Div(Class("nav-live-n"), Text("40")),
				Div(Class("nav-live-l"), Text("active")),
			),
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

// ── Formatting helpers ────────────────────────────────────────────────────────

func formatPopulation(n int) string {
	return "Pop. " + FormatThousands(n)
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
