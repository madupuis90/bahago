package iconpreview

import (
	"net/http"

	. "bahago/internal/ui"
	"bahago/internal/router"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

func RegisterRoutes(r router.Router) {
	r.HandleFunc("GET /dev/icons", handleIconPreview())
}

func handleIconPreview() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		iconPreviewPage().Render(w)
	}
}

const iconSize = 44

type iconOpt struct {
	id    string
	label string
}

func iconPreviewPage() Node {
	return Doctype(
		HTML(Lang("en"),
			Head(
				TitleEl(Text("Icon Preview — Dev")),
				Link(Rel("icon"), Href("data:,")),
				Link(Rel("stylesheet"), Href("/static/styles.css")),
				El("style", Raw(`
					body { padding: 2rem; background: #fdf8ef; color: #2c1810; font-family: Georgia, serif; }
					h1 { margin: 0 0 0.25rem; font-size: 1.4rem; }
					.dev-note { color: #888; font-size: 0.8rem; margin-bottom: 2rem; }
					h2 { font-size: 0.8rem; text-transform: uppercase; letter-spacing: 0.1em;
					     margin: 2rem 0 1rem; border-bottom: 1px solid #c8b89a; padding-bottom: 0.4rem; }
					.icon-grid { display: flex; flex-wrap: wrap; gap: 2.5rem; }
					.icon-group { display: flex; flex-direction: column; gap: 0.6rem; }
					.icon-group-label { font-size: 0.7rem; text-transform: uppercase;
					                    letter-spacing: 0.08em; color: #8a7060; font-weight: bold; }
					.icon-row { display: flex; gap: 1.5rem; align-items: flex-start; }
					.icon-opt { display: flex; flex-direction: column; align-items: center; gap: 0.35rem; }
					.icon-opt-label { font-size: 0.65rem; text-align: center; max-width: 60px; color: #7a6050; }
					.icon-opt--current { background: rgba(160,130,90,0.15); border-radius: 6px; padding: 4px; }
				`)),
			),
			Body(
				H1(Text("Icon Preview")),
				P(Class("dev-note"), Text("Dev-only page. Highlighted option is the current icon.")),
				El("h2", Text("Resources")),
				Div(Class("icon-grid"),
					resourceGroup("Wood", "tree", []iconOpt{}),
					resourceGroup("Mana", "flame", []iconOpt{
						{"sun", "sun (devotion)"},
					}),
					resourceGroup("Knowledge", "book", []iconOpt{
						{"star", "star"},
						{"quill", "quill"},
						{"lantern", "lantern"},
						{"eye", "eye"},
					}),
				),
				El("h2", Text("Navigation Stones")),
				Div(Class("icon-grid"),
					navGroup("Kingdom", "crown", "", ""),
					iconGroup("Allocate", []iconOpt{
						{"wheat", "current"},
						{"sliders", "sliders"},
						{"scales", "scales"},
					}),
					navGroup("Builds", "mountain", "house", "house"),
					iconGroup("Levy → Units", []iconOpt{
						{"person", "current"},
						{"helmet", "helmet"},
						{"spear", "spear"},
						{"soldiers", "soldiers"},
					}),
					iconGroup("Host → Army", []iconOpt{
						{"cross", "current"},
						{"sword", "sword (45°)"},
						{"swords", "swords (×2)"},
					}),
					navGroup("Atlas → World Map", "sun", "globe", "globe"),
					navGroup("Prayers", "moon", "cross", "cross"),
					navGroup("Scrolls → Messages", "book", "envelope", "envelope"),
					navGroup("Order → Guild", "chevron", "flag", "flag"),
				),
			),
		),
	)
}

func resourceGroup(resource, currentID string, alts []iconOpt) Node {
	opts := append([]iconOpt{{currentID, "current"}}, alts...)
	return iconGroup(resource, opts)
}

func navGroup(name, currentID, newID, newLabel string) Node {
	if newID == "" {
		return iconGroup(name, []iconOpt{{currentID, "keep"}})
	}
	return iconGroup(name, []iconOpt{
		{currentID, "current"},
		{newID, newLabel},
	})
}

func iconGroup(label string, opts []iconOpt) Node {
	rows := make([]Node, len(opts))
	for i, opt := range opts {
		cls := "icon-opt"
		if i == 0 && len(opts) > 1 {
			cls = "icon-opt icon-opt--current"
		}
		rows[i] = Div(Class(cls),
			Shield(opt.id, iconSize, false),
			Div(Class("icon-opt-label"), Text(opt.label)),
		)
	}
	return Div(Class("icon-group"),
		Div(Class("icon-group-label"), Text(label)),
		Div(Class("icon-row"), Group(rows)),
	)
}
