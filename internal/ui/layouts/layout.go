package layouts

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
				Script(Type("module"), Src("https://cdn.jsdelivr.net/gh/starfederation/datastar@1.0.0-RC.7/bundles/datastar.js")),
			),
			Body(
				Nav(
					A(Text("Home"), Href("/")),
					A(Text("Login"), Href("/login")),
					A(Text("About"), Href("/about")),
					A(Text("Test"), Href("/test")),
				),
				Main(body...), // Inject the content here
				Footer(),
			),
		),
	)
}

func Layout2(args LayoutArgs) {

}
