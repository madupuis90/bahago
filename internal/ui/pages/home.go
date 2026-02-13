package pages

import (
	. "github.com/mad/bahago/internal/ui/layouts"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

func Home() Node {
	return Layout(
		LayoutArgs{
			Title: "Home Page",
		},
		H1(Text("Welcome!")),
		P(Text("This content is wrapped in a layout.")),
	)
}
