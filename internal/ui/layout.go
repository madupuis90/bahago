package ui

import (
	"bahago/internal/contextkeys"
	"bahago/internal/routes"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

type LayoutArgs struct {
	Title string
	User  *contextkeys.SessionUser
}

func Layout(args LayoutArgs, body ...Node) Node {
	return Doctype(
		HTML(
			Lang("en"),
			Head(
				TitleEl(Text(args.Title)),
				Link(Rel("stylesheet"), Href("/static/styles.css")),
				Script(Type("module"), Src("/static/datastar.js")),
			),
			Body(
				Nav(
					A(Text("Home"), Href(routes.HomePath)),
					If(args.User == nil,
						A(Text("Login"), Href(routes.LoginPath)),
					),
					If(args.User != nil,
						Form(Method("POST"), Action(routes.LogoutPath),
							Button(Type("submit"), Text("Logout"), Class("nav-link-btn")),
						),
					),
					A(Text("Resources"), Href(routes.ResourcesPath)),
					A(Text("Chat"), Href(routes.ChatPath)),
					A(Text("Realm"), Href(routes.RealmPath)),
				),
				Main(body...), // Inject the content here
				Footer(),
			),
		),
	)
}
