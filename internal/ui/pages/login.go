package pages

import (
	. "github.com/mad/bahago/internal/ui/layouts"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

func Login() Node {
	return Layout(
		LayoutArgs{
			Title: "Login Page",
		},
		H1(Text("Login")),
		Div(
			Label(Text("Username"),
				Input(Name("username")),
			),
			Label(Text("Password"),
				Input(Name("password")),
			),
		),
	)
}
