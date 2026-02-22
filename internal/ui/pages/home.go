package pages

import (
	. "bahago/internal/ui/layouts"
	"fmt"

	database "bahago/internal/database/sqlc"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

func Home(resources []database.Resource) Node {
	return Layout(
		LayoutArgs{
			Title: "Home Page",
		},
		H1(Text("Welcome!")),
		Form(Action("/resources/create"), Method("POST"),
			Label(Text("Wood"),
				Input(Name("wood")),
			),
			Label(Text("Stone"),
				Input(Name("stone")),
			),
			Label(Text("Food"),
				Input(Name("food")),
			),
			Br(),
			Button(Text("Submit")),
		),
		Table(
			Tr(
				Th(Text("Wood")),
				Th(Text("Stone")),
				Th(Text("Food")),
			),
			TBody(
				ID("resource-list"),
				Map(resources, func(res database.Resource) Node {
					return Tr(
						Td(Text(fmt.Sprintf("%d", res.Wood))),
						Td(Text(fmt.Sprintf("%d", res.Stone))),
						Td(Text(fmt.Sprintf("%d", res.Food))),
					)
				}),
			),
		),
	)
}
