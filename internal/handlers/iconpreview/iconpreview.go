package iconpreview

import (
	"encoding/xml"
	"io/fs"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"bahago/internal/router"
	"bahago/web"
	. "bahago/internal/ui"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

const spriteFile = "static/sprite.svg"

// symbolIDs is the list of every <symbol id="…"> found in the embedded
// web/static/sprite.svg. It is parsed once at package init so the preview
// page never goes stale when icons are added to or removed from the sprite.
var symbolIDs = loadSymbolIDs()

func loadSymbolIDs() []string {
	data, err := fs.ReadFile(web.Static, spriteFile)
	if err != nil {
		// The sprite is embedded at build time; failing here is a compile-time
		// packaging bug, not a runtime condition we can recover from.
		panic("iconpreview: cannot read embedded " + spriteFile + ": " + err.Error())
	}

	var ids []string
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "symbol" {
			continue
		}
		for _, attr := range start.Attr {
			if attr.Name.Local == "id" {
				ids = append(ids, attr.Value)
			}
		}
	}
	sort.Strings(ids)
	return ids
}

// RegisterRoutes mounts the dev icon-audit page at /dev/icons. It lists every
// <symbol> in sprite.svg so icons can be reviewed for renaming or removal.
// Intended to be deleted once icon cleanup is complete.
func RegisterRoutes(r router.Router) {
	r.HandleFunc("GET /dev/icons", handleIconPreview())
}

func handleIconPreview() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		iconPreviewPage(symbolIDs).Render(w)
	}
}

const iconSize = 48

func iconPreviewPage(ids []string) Node {
	glyphs, resources, utility := groupByPrefix(ids)
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
					h2 { font-size: 0.85rem; text-transform: uppercase; letter-spacing: 0.1em;
					     margin: 2rem 0 0.75rem; border-bottom: 1px solid #c8b89a; padding-bottom: 0.4rem;
					     display: flex; justify-content: space-between; }
					h2 .count { color: #8a7060; font-weight: normal; letter-spacing: 0; }
					.icon-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(90px, 1fr));
					             gap: 1.25rem 1rem; }
					.icon-cell { display: flex; flex-direction: column; align-items: center; gap: 0.4rem;
					             padding: 0.6rem 0.25rem; border-radius: 8px; }
					.icon-cell:hover { background: rgba(160,130,90,0.10); }
					.icon-cell .id { font-size: 0.7rem; text-align: center; color: #5a4a38;
					                 word-break: break-word; font-family: monospace; }
					.icon-cell .short { font-size: 0.6rem; color: #9a8a78; }
				`)),
			),
			Body(
				H1(Text("Icon Preview")),
				P(Class("dev-note"),
					Text("Dev-only audit page — lists every <symbol> in sprite.svg. Delete this route when icon cleanup is done.")),
				section("Glyphs", glyphs, len(ids)),
				section("Resources", resources, len(ids)),
				section("Utility", utility, len(ids)),
			),
		),
	)
}

func section(title string, ids []string, total int) Node {
	return Group([]Node{
		El("h2", Text(title), Span(Class("count"), Text(strconv.Itoa(len(ids))+" / "+strconv.Itoa(total)))),
		Div(Class("icon-grid"), Group(iconCells(ids))),
	})
}

func iconCells(ids []string) []Node {
	cells := make([]Node, len(ids))
	for i, id := range ids {
		cells[i] = Div(Class("icon-cell"),
			// Icon renders <use href="/static/sprite.svg#id"> — works for every
			// symbol regardless of shield-/res-/standalone prefix.
			Icon(id, iconSize, false),
			Div(Class("id"), Text(id)),
			Div(Class("short"), Text(shortName(id))),
		)
	}
	return cells
}

// groupByPrefix splits ids into the glyph, resource, and utility
// buckets, preserving sorted order within each. Glyphs are the standalone
// 24×24 symbols (crown); resources are the #res-* symbols; utility holds
// the sandglass and idle glyphs.
func groupByPrefix(ids []string) (glyphs, resources, utility []string) {
	for _, id := range ids {
		switch {
		case id == "sandglass" || id == "idle":
			utility = append(utility, id)
		case strings.HasPrefix(id, "res-"):
			resources = append(resources, id)
		default:
			glyphs = append(glyphs, id)
		}
	}
	return
}

// shortName is the id with its category prefix stripped, shown as a hint.
func shortName(id string) string {
	return strings.TrimPrefix(strings.TrimPrefix(id, "res-"), "")
}
