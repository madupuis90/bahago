package ui

import (
	"bahago/internal/contextkeys"

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
					A(Text("Home"), Href("/")),
					If(args.User == nil,
						A(Text("Login"), Href("/login")),
					),
					If(args.User != nil,
						Form(Method("POST"), Action("/logout"),
							Button(Type("submit"), Text("Logout"), Class("nav-link-btn")),
						),
					),
					A(Text("Resources"), Href("/resources")),
					A(Text("Chat"), Href("/chat")),
					A(Text("Realm"), Href("/realm")),
				),
				Main(body...), // Inject the content here
				Footer(),
			),
		),
	)
}
