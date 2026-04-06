package ui

import (
	"fmt"
	"net/http"
	"strings"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/routes"

	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"
)

type LayoutArgs struct {
	Title       string
	User        *contextkeys.SessionUser
	Kingdom     *db.Kingdom
	CurrentPath string
}

// ── Request-scoped layout ─────────────────────────────────────────────────────

// LayoutFn is a configured layout ready to wrap page content.
type LayoutFn func(title string, content ...Node) Node

// AppLayout captures the current user, kingdom, and path from the request
// context and returns a LayoutFn backed by the standard application layout.
func AppLayout(r *http.Request) LayoutFn {
	user, _ := r.Context().Value(contextkeys.User).(*contextkeys.SessionUser)
	kingdom, _ := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
	return func(title string, content ...Node) Node {
		return Layout(LayoutArgs{
			Title:       title,
			User:        user,
			Kingdom:     kingdom,
			CurrentPath: r.URL.Path,
		}, content...)
	}
}

// NewPage assembles a full page from a title, layout, and content nodes.
func NewPage(title string, l LayoutFn, content ...Node) Node {
	return l(title, content...)
}

func Layout(args LayoutArgs, content ...Node) Node {
	currentPath := args.CurrentPath
	isKingdom := strings.HasPrefix(currentPath, routes.KingdomPath)
	return Doctype(
		HTML(
			Lang("en"),
			Head(
				TitleEl(Text(args.Title)),
				Link(Rel("stylesheet"), Href("/static/styles.css")),
				Script(Type("module"), Src("/static/datastar.js")),
			),
			Body(
				Nav(Class("top-nav panel"),
					Div(Class("top-nav-left"),
						NavItem(routes.HomePath, "Home", currentPath),
						NavItem(routes.KingdomPath, "Kingdom", currentPath),
					),
					Div(Class("top-nav-right"),
						LoginNav(args.User, currentPath),
					),
				),
				Div(Class("content-area"),
					Nav(Class("side-nav panel"),
						Iff(!isKingdom, func() Node { return HomeSideNav(currentPath) }),
						Iff(isKingdom, func() Node { return KingdomSideNav(currentPath, args.Kingdom) }),
					),
					Main(content...),
				),
				Footer(),
			),
		),
	)
}

func NavItem(href, name, currentPath string) Node {
	return A(Href(href), If(currentPath == href, Attr("aria-current", "page")), Text(name))
}

func LoginNav(user *contextkeys.SessionUser, currentPath string) Node {
	return Group([]Node{
		If(user == nil,
			Group([]Node{
				NavItem(routes.LoginPath, "Login", currentPath),
				NavItem(routes.RegisterPath, "Register", currentPath),
			}),
		),
		If(user != nil,
			A(ds.On("click", datastar.PostSSE(routes.LogoutPath)), Text("Logout")),
		),
	})
}

func NavGroup(name string, navItems ...Node) Node {
	return Div(Class("nav-group"),
		Div(P(Class("nav-group-name"), Text(name))),
		Div(Class("nav-group-content"), Group(navItems)),
	)
}

func Resource(kingdom *db.Kingdom) {

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

func KingdomSideNav(currentPath string, kingdom *db.Kingdom) Node {

	return Group([]Node{
		NavGroup("Resources",
			P(Text(fmt.Sprintf("Wood: %v", kingdom.Wood))),
			P(Text(fmt.Sprintf("Stone: %v", kingdom.Stone))),
			P(Text(fmt.Sprintf("Food: %v", kingdom.Food))),
			P(Text(fmt.Sprintf("Mana: %v", kingdom.Mana))),
			P(Text(fmt.Sprintf("Devotion: %v", kingdom.Devotion))),
			P(Text(fmt.Sprintf("Knowledge: %v", kingdom.Knowledge))),
		),
		NavItem(routes.KingdomPath, "Overview", currentPath),
		NavItem(routes.KingdomResourcesPath, "Resources", currentPath),
	})
}
