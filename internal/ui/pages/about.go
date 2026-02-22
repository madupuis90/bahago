package pages

import (
	. "bahago/internal/ui/layouts"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

func About() Node {
	return Layout(
		LayoutArgs{
			Title: "About Page",
		},
		H1(Text("Welcome!")),
		P(Text("This content is wrapped in a layout.")),
	)
}
