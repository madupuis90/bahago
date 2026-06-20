package identitypreview

import (
	"fmt"
	"net/http"

	"bahago/internal/router"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/components"
	. "maragu.dev/gomponents/html"
)

// RegisterRoutes mounts the dev identity-mark preview at /dev/identity.
func RegisterRoutes(r router.Router) {
	r.HandleFunc("GET /dev/identity", handleIdentityPreview())
}

func handleIdentityPreview() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identityPreviewPage().Render(w)
	}
}

// identityPreviewPage shows The Coin — the accepted identity mark — at all
// real sizes and in all chrome contexts. The inner symbol is empty (TBD).
func identityPreviewPage() Node {
	return Doctype(
		HTML(Lang("en"),
			Head(
				TitleEl(Text("Identity Mark — The Coin (Dev)")),
				Link(Rel("icon"), Href("data:,")),
				Link(Rel("stylesheet"), Href("/static/styles.css")),
				El("style", Raw(`
					body { padding: 2rem; background: #fdf8ef; color: #2c1810; font-family: var(--font-body), Nunito, sans-serif; }
					h1 { margin: 0 0 .25rem; font-size: 1.5rem; font-family: var(--font-display), Lilita One, sans-serif; }
					.dev-note { color: #8a7060; font-size: .82rem; margin-bottom: 2rem; }
					.status { display: inline-block; padding: 2px 8px; border-radius: 4px; background: var(--green); color: #fff; font-size: .7rem; font-weight: 700; text-transform: uppercase; letter-spacing: .06em; margin-left: .5rem; vertical-align: middle; }
					.section { margin-bottom: 2rem; }
					.section-h { font-size: .85rem; text-transform: uppercase; letter-spacing: .1em; color: #8a7060; margin: 0 0 .75rem; border-bottom: 1px solid #c8b89a; padding-bottom: .4rem; }
					.size-row { display: flex; align-items: flex-start; gap: 2rem; flex-wrap: wrap; }
					.size-cell { display: flex; flex-direction: column; align-items: center; gap: .4rem; }
					.size-label { font-size: .68rem; color: #8a7060; font-family: monospace; }
					.context-row { display: flex; gap: 1.5rem; margin-top: .5rem; flex-wrap: wrap; }
					.context-label { font-size: .6rem; text-transform: uppercase; letter-spacing: .06em; color: #9a8a78; }

					/* Skin tokens for the dev preview */
					.skin-id       { --crest-ring: var(--primary-ink); --crest-fill: rgba(255,246,230,.18); }
					.skin-home     { --crest-ring: var(--chrome-accent); --crest-fill: rgba(247,238,213,.6); }
					.skin-auth     { --crest-ring: var(--chrome-accent); --crest-fill: rgba(247,238,213,.72); }
					.skin-map-self { --crest-ring: var(--ink); --crest-fill: var(--chrome-accent); }
					.skin-map-other{ --crest-ring: var(--ink); --crest-fill: #3a6390; }

					/* Context strips */
					.bar-strip {
						background: var(--red);
						padding: 8px 14px;
						border-radius: 6px;
						display: inline-flex;
					}
					.parchment-strip {
						background: linear-gradient(175deg, #fffaf0, #f5ecda);
						padding: 8px 14px;
						border-radius: 6px;
						border: 1.5px solid var(--edge);
						display: inline-flex;
					}
					.auth-strip {
						background: linear-gradient(175deg, #fffaf0, #f5ecda);
						padding: 10px 16px;
						border-radius: 6px;
						border: 1.5px solid var(--edge);
						display: inline-flex;
						box-shadow: 0 4px 10px rgba(60,40,20,.12);
					}
					.map-strip {
						background: #a9c47e;
						padding: 6px 10px;
						border-radius: 4px;
						display: inline-flex;
					}
				`)),
			),
			Body(
				H1(Text("Identity Mark — The Coin"), Span(Class("status"), Text("Accepted"))),
				P(Class("dev-note"),
					Text("The Coin is the settled form. The inner symbol is empty (TBD). "),
					Text("This page verifies the coin reads at every size and in every chrome context.")),

				Div(Class("section"),
					Div(Class("section-h"), Text("Sizes")),
					Div(Class("size-row"),
						Div(Class("size-cell"), coinMark(54, "skin-auth"), Span(Class("size-label"), Text("54px — auth hero"))),
						Div(Class("size-cell"), coinMark(40, "skin-id"), Span(Class("size-label"), Text("40px — command bar"))),
						Div(Class("size-cell"), coinMark(32, "skin-home"), Span(Class("size-label"), Text("32px — home chrome"))),
						Div(Class("size-cell"), coinMark(14, "skin-map-self"), Span(Class("size-label"), Text("14px — map (self)"))),
						Div(Class("size-cell"), coinMark(14, "skin-map-other"), Span(Class("size-label"), Text("14px — map (other)"))),
					),
				),

				Div(Class("section"),
					Div(Class("section-h"), Text("Contexts")),
					Div(Class("context-row"),
						Div(Class("size-cell"), Div(Class("bar-strip"), coinMark(40, "skin-id")), Span(Class("context-label"), Text("CommandBar"))),
						Div(Class("size-cell"), Div(Class("parchment-strip"), coinMark(32, "skin-home")), Span(Class("context-label"), Text("Home chrome"))),
						Div(Class("size-cell"), Div(Class("auth-strip"), coinMark(54, "skin-auth")), Span(Class("context-label"), Text("Auth hero"))),
						Div(Class("size-cell"), Div(Class("map-strip"), coinMark(14, "skin-map-self")), Span(Class("context-label"), Text("Map — self"))),
						Div(Class("size-cell"), Div(Class("map-strip"), coinMark(14, "skin-map-other")), Span(Class("context-label"), Text("Map — other"))),
					),
				),
			),
		),
	)
}

// coinMark renders The Coin: a bold disc with a thick ink ring and flat fill.
// The inner symbol is omitted (empty) — this is the current accepted state.
func coinMark(size int, skin string) Node {
	return Span(
		Classes{"crest": true, skin: true},
		Style(fmt.Sprintf("width:%dpx;height:%dpx;min-width:%dpx", size, size, size)),
	)
}
