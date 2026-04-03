package ui

import (
	"strings"

	"bahago/internal/contextkeys"
	"bahago/internal/routes"

	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"
)

type LayoutArgs struct {
	Title       string
	User        *contextkeys.SessionUser
	CurrentPath string
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
						If(!isKingdom, HomeSideNav(currentPath)),
						If(isKingdom, KingdomSideNav(currentPath)),
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

func KingdomSideNav(currentPath string) Node {
	return Group([]Node{
		NavItem(routes.KingdomPath, "Overview", currentPath),
		NavItem(routes.KingdomResourcesPath, "Resources", currentPath),
	})
}
