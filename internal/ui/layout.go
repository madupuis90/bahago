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

// svgDefs is the inline SVG symbol library. Prepended to <body> so all glyph
// symbols are defined before any element references them.
const svgDefs = `<svg xmlns="http://www.w3.org/2000/svg" width="0" height="0"
     style="position:absolute" aria-hidden="true">
  <defs>
    <symbol id="sandglass" viewBox="0 0 10 14">
      <path d="M1 1 H9 L5 6 L9 11 V13 H1 V11 L5 6 Z"
            fill="none" stroke="currentColor" stroke-width="0.8"/>
      <path d="M3 11 L7 11 L5 8 Z" fill="currentColor"/>
    </symbol>
    <g id="g-crown"><path d="M5 14.5 L5 10 L7.5 12 L10 8 L12.5 12 L15 10 L15 14.5 Z"
      fill="none" stroke="currentColor" stroke-width="0.9"
      stroke-linecap="square" stroke-linejoin="round"/></g>
    <g id="g-cross"><g fill="none" stroke="currentColor" stroke-width="1.1"
      stroke-linecap="square" stroke-linejoin="round">
      <line x1="10" y1="6" x2="10" y2="15"/>
      <line x1="6.5" y1="10" x2="13.5" y2="10"/></g></g>
    <g id="g-swords"><g fill="none" stroke="currentColor" stroke-width="0.85"
      stroke-linecap="round" stroke-linejoin="round">
      <line x1="7.5" y1="7.5" x2="12.5" y2="16"/>
      <line x1="7.9" y1="11.1" x2="10.5" y2="9.6"/>
      <line x1="12.5" y1="7.5" x2="7.5" y2="16"/>
      <line x1="9.5" y1="9.6" x2="12.1" y2="11.1"/></g></g>
    <g id="g-sliders"><g stroke="currentColor" stroke-linecap="round">
      <line x1="5" y1="9.5" x2="15" y2="9.5" fill="none" stroke-width="0.8"/>
      <circle cx="8.5" cy="9.5" r="1.5" fill="rgba(255,250,236,0.8)" stroke-width="0.9"/>
      <line x1="5" y1="12.5" x2="15" y2="12.5" fill="none" stroke-width="0.8"/>
      <circle cx="11.5" cy="12.5" r="1.5" fill="rgba(255,250,236,0.8)" stroke-width="0.9"/>
      <line x1="5" y1="15.5" x2="15" y2="15.5" fill="none" stroke-width="0.8"/>
      <circle cx="9.5" cy="15.5" r="1.5" fill="rgba(255,250,236,0.8)" stroke-width="0.9"/></g></g>
    <g id="g-house"><g fill="none" stroke="currentColor" stroke-width="0.95"
      stroke-linecap="round" stroke-linejoin="round">
      <path d="M5 13.5 L10 8.5 L15 13.5"/>
      <path d="M6 13.5 L6 17 L14 17 L14 13.5"/>
      <path d="M8.5 17 L8.5 14.5 L11.5 14.5 L11.5 17"/></g></g>
    <g id="g-person"><g fill="none" stroke="currentColor" stroke-width="0.95"
      stroke-linecap="round" stroke-linejoin="round">
      <circle cx="10" cy="9" r="2.2"/>
      <path d="M6.5 17.5 C6.5 14 8 12.5 10 12.5 C12 12.5 13.5 14 13.5 17.5"/></g></g>
    <g id="g-globe"><g fill="none" stroke="currentColor" stroke-linecap="round"
      stroke-linejoin="round">
      <circle cx="10" cy="12" r="4.5" stroke-width="0.9"/>
      <line x1="5.5" y1="12" x2="14.5" y2="12" stroke-width="0.65"/>
      <path d="M10 7.5 C8 9.5 8 14.5 10 16.5" stroke-width="0.65"/>
      <path d="M10 7.5 C12 9.5 12 14.5 10 16.5" stroke-width="0.65"/></g></g>
    <g id="g-envelope"><g fill="none" stroke="currentColor" stroke-width="0.95"
      stroke-linecap="round" stroke-linejoin="round">
      <rect x="5" y="8.5" width="10" height="7" rx="0.3"/>
      <path d="M5 8.5 L10 13 L15 8.5"/></g></g>
    <g id="g-tree"><g fill="none" stroke="currentColor"
      stroke-linecap="round" stroke-linejoin="round">
      <path d="M10 8 L14.5 15 L5.5 15 Z" stroke-width="0.95"/>
      <line x1="10" y1="15" x2="10" y2="17.5" stroke-width="1.2"/></g></g>
    <g id="g-mountain"><path d="M4.5 15.5 L8 11 L10.5 13 L13.5 9 L15.5 15.5 Z"
      fill="none" stroke="currentColor" stroke-width="1.1"
      stroke-linecap="square" stroke-linejoin="round"/></g>
    <g id="g-wheat"><g fill="none" stroke="currentColor" stroke-width="0.9"
      stroke-linecap="round" stroke-linejoin="round">
      <line x1="10" y1="6.5" x2="10" y2="15.5"/>
      <line x1="10" y1="9.5" x2="7.5" y2="11"/>
      <line x1="10" y1="9.5" x2="12.5" y2="11"/>
      <line x1="10" y1="12" x2="7.5" y2="13.5"/>
      <line x1="10" y1="12" x2="12.5" y2="13.5"/></g></g>
    <g id="g-flame"><path d="M10 8 C11 10 12.5 11.5 12.5 13.5 C12.5 15.8 11.4 17 10 17
      C8.6 17 7.5 15.8 7.5 13.5 C7.5 11.5 9 10 10 8 Z"
      fill="none" stroke="currentColor" stroke-width="0.9"
      stroke-linecap="round" stroke-linejoin="round"/>
      <path d="M10 16 C9.5 14.5 9.5 12.5 10.5 11"
      fill="none" stroke="currentColor" stroke-width="0.65" stroke-linecap="round"/></g>
    <g id="g-sun"><g fill="none" stroke="currentColor" stroke-width="0.9"
      stroke-linecap="square" stroke-linejoin="round">
      <circle cx="10" cy="11.5" r="2.4"/>
      <g stroke-linecap="round">
        <line x1="10" y1="6.5" x2="10" y2="8"/>
        <line x1="10" y1="15" x2="10" y2="16.5"/>
        <line x1="5.5" y1="11.5" x2="7" y2="11.5"/>
        <line x1="13" y1="11.5" x2="14.5" y2="11.5"/>
        <line x1="7" y1="8.5" x2="8" y2="9.5"/>
        <line x1="12" y1="13.5" x2="13" y2="14.5"/>
        <line x1="7" y1="14.5" x2="8" y2="13.5"/>
        <line x1="12" y1="9.5" x2="13" y2="8.5"/></g></g></g>
    <g id="g-star"><path d="M10,7 L11,10.1 L14.2,10.1 L11.6,12 L12.6,15.1
      L10,13.2 L7.4,15.1 L8.4,12 L5.8,10.1 L9,10.1 Z"
      fill="none" stroke="currentColor" stroke-width="0.85" stroke-linejoin="round"/></g>
  </defs>
</svg>`

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

// shell is the shared HTML document structure. The SVG defs block is prepended to <body>
// so all glyph symbols are available before any element references them.
func shell(title string, layoutStream Node, body ...Node) Node {
	bodyChildren := append([]Node{Raw(svgDefs)}, body...)
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
			Raw(`<svg class="crest home-chrome-crest" width="32" height="37" viewBox="0 0 20 23" aria-hidden="true"><g class="crest-frame"><path class="crest-shield" d="M2 2 L18 2 L18 11 C18 17 14 21 10 22 C6 21 2 17 2 11 Z" stroke="currentColor" stroke-width="0.9" stroke-linejoin="round"/><path d="M3.5 3.5 L16.5 3.5 L16.5 10.8 C16.5 16 13 19.5 10 20.4 C7 19.5 3.5 16 3.5 10.8 Z" fill="none" stroke="currentColor" stroke-width="0.35" stroke-linejoin="round" opacity="0.5"/></g><use class="crest-glyph" href="#g-crown"/></svg>`),
			Span(Class("home-chrome-name"), Text("Bahago")),
		),
		Span(Class("home-chrome-sep vrule")),
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

// Glyph renders a bare SVG glyph from the inline symbol library.
func Glyph(id string, sizePx int) Node {
	sz := strconv.Itoa(sizePx)
	return El("svg",
		Class("gly"),
		Attr("width", sz),
		Attr("height", sz),
		Attr("viewBox", "4 6 12 12"),
		Attr("aria-hidden", "true"),
		El("use", Attr("href", "#g-"+id)),
	)
}

func commandBarIdentity(kingdom *db.Kingdom) Node {
	return Div(Class("id"),
		Raw(`<svg class="crest id-crest" width="40" height="46" viewBox="0 0 20 23" aria-hidden="true"><g class="crest-frame"><path class="crest-shield" d="M2 2 L18 2 L18 11 C18 17 14 21 10 22 C6 21 2 17 2 11 Z" stroke="currentColor" stroke-width="0.9" stroke-linejoin="round"/><path d="M3.5 3.5 L16.5 3.5 L16.5 10.8 C16.5 16 13 19.5 10 20.4 C7 19.5 3.5 16 3.5 10.8 Z" fill="none" stroke="currentColor" stroke-width="0.35" stroke-linejoin="round" opacity="0.5"/></g><use class="crest-glyph" href="#g-crown"/></svg>`),
		Div(
			Div(Class("id-name"), Text(kingdom.Name)),
			Div(Class("id-sub"), Text(formatPopulation(kingdom.Population))),
		),
	)
}

func commandBarTick() Node {
	return Span(Class("tick"),
		Raw(`<svg width="9" height="13" viewBox="0 0 10 14" aria-hidden="true"><use href="#sandglass"/></svg>`),
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
	glyph string
	href  string
}

var kingdomNavItems = []navItem{
	{"Kingdom", "crown", routes.KingdomPath},
	{"Allocate", "sliders", routes.KingdomAllocationPath},
	{"Builds", "house", routes.KingdomBuildingsPath},
	{"Units", "person", routes.KingdomUnitsPath},
	{"Campaign", "swords", routes.KingdomArmyPath},
	{"World", "globe", routes.KingdomMapPath},
	{"Prayers", "cross", routes.KingdomPrayersPath},
	{"Messages", "envelope", routes.KingdomMessagesPath},
	{"Guild", "star", routes.GuildPath},
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
			Span(Class("nav-link-ico"), Glyph(item.glyph, 16), badgeNode),
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
		Div(P(Class("nav-group-name"), Text(name))),
		Div(Class("nav-group-content"), Group(items)),
	)
}

// URLs are placeholder for now, no need to create routes
func HomeSideNav(currentPath string) Node {
	return Group([]Node{
		Div(Class("nav-live"),
			Span(Class("nav-live-dot")),
			Span(Class("nav-live-n"), Text("40")),
			Span(Class("nav-live-l"), Text("active")),
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
