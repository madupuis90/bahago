package ui

import (
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

type LayoutArgs struct {
	Title string
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
					A(Text("Login"), Href("/login")),
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
