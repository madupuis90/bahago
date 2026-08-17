package ui

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"bahago/web"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/components"
	. "maragu.dev/gomponents/html"
)

const spritePath = "/static/sprite.svg"

// symbolSize maps each <symbol id="…"> in sprite.svg to its viewBox
// (width, height). It is parsed once at package init from the embedded FS so
// Icon can derive the correct height for any symbol's aspect ratio.
var symbolSize = loadSymbolSizes()

type viewBox struct{ w, h float64 }

func loadSymbolSizes() map[string]viewBox {
	data, err := fs.ReadFile(web.Static, "static/sprite.svg")
	if err != nil {
		panic("ui: cannot read embedded static/sprite.svg: " + err.Error())
	}
	sizes := map[string]viewBox{}
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
		var id string
		var vb viewBox
		for _, attr := range start.Attr {
			switch attr.Name.Local {
			case "id":
				id = attr.Value
			case "viewBox":
				// "min-x min-y width height" — we only keep w/h.
				var minX, minY float64
				fmt.Sscanf(attr.Value, "%f %f %f %f", &minX, &minY, &vb.w, &vb.h)
			}
		}
		if id != "" && vb.w > 0 && vb.h > 0 {
			sizes[id] = vb
		}
	}
	return sizes
}

// Icon renders a sprite glyph by id. id is the full symbol id (e.g. "crown",
// "res-wood", "sandglass", "idle"). The height is derived from the symbol's
// viewBox so every icon is proportioned correctly. active toggles the accent
// color via the icon--active modifier.
func Icon(id string, sizePx int, active bool) Node {
	cls := "icon"
	if active {
		cls = "icon icon--active"
	}
	return El("svg",
		Class(cls),
		Attr("width", strconv.Itoa(sizePx)),
		Attr("height", strconv.Itoa(iconHeight(id, sizePx))),
		Attr("aria-hidden", "true"),
		El("use", Attr("href", spritePath+"#"+id)),
	)
}

// Crest renders the kingdom identity mark: a bold coin (disc with a thick
// ink ring). The inner symbol is optional — when id is empty the coin is
// rendered without a glyph, which is the current state (the symbol is still
// TBD). extraClass (e.g. "crest-lg", "id-crest", "home-chrome-crest") selects
// the ring and fill colour via the .crest-* CSS tokens.
//
// The coin is a standalone frame, not composed from the sprite. When a symbol
// is chosen later it can be added as a child node without changing the frame.
func Crest(id string, sizePx int, extraClass string) Node {
	cls := "crest"
	if extraClass != "" {
		cls = "crest " + extraClass
	}
	children := []Node{
		Class(cls),
		Style(fmt.Sprintf("width:%dpx;height:%dpx;min-width:%dpx", sizePx, sizePx, sizePx)),
	}
	if id != "" {
		children = append(children, Icon(id, sizePx*58/100, false))
	}
	return Span(children...)
}

// iconHeight derives the pixel height for sizePx using id's viewBox aspect
// ratio. Unknown ids fall back to a square (sizePx × sizePx).
func iconHeight(id string, sizePx int) int {
	if vb, ok := symbolSize[id]; ok && vb.w > 0 {
		return int(float64(sizePx)*vb.h/vb.w + 0.5)
	}
	return sizePx
}

// gemIDToResKey maps a gem colour id (tree/mountain/wheat/flame/sun/star) to
// its resource symbol key (wood/stone/food/mana/devotion/knowledge) defined in
// sprite.svg as #res-<key>. Used by ResourceGem to render the canonical
// resource artwork (tree · mountain · wheat · drop · sun · star) on the gem.
var gemIDToResKey = map[string]string{
	"tree":     "wood",
	"mountain": "stone",
	"wheat":    "food",
	"flame":    "mana",
	"sun":      "devotion",
	"star":     "knowledge",
}

// resKeyToGemID is the inverse of gemIDToResKey: resource symbol key → gem
// colour id. Callers that think in resource keys use GemIDForResource to get
// the gem colour for ResourceGem.
var resKeyToGemID = map[string]string{
	"wood":      "tree",
	"stone":     "mountain",
	"food":      "wheat",
	"mana":      "flame",
	"devotion":  "sun",
	"knowledge": "star",
}

// GemIDForResource returns the gem colour id for a resource key
// (wood/stone/food/mana/devotion/knowledge). Unknown keys fall back to the
// key itself — ResourceGem tolerates either namespace.
func GemIDForResource(resKey string) string {
	if id, ok := resKeyToGemID[resKey]; ok {
		return id
	}
	return resKey
}

// ResourceGem renders a filled resource gem: the coloured disc (.gem-<gemID>)
// with the white resource glyph on top. gemID is the gem colour id
// (tree/mountain/wheat/flame/sun/star); sizePx is the gem diameter. Use this
// anywhere a resource is shown as a gem (chrome pills, allocation roster,
// units stat pills, building cost chips / banner).
func ResourceGem(gemID string, sizePx int) Node {
	resKey, ok := gemIDToResKey[gemID]
	if !ok {
		resKey = gemID // graceful fallback: assume caller passed a res key
	}
	return Span(
		Classes{"gem": true, "gem-" + gemID: true},
		Style(fmt.Sprintf("width:%dpx;height:%dpx;min-width:%dpx", sizePx, sizePx, sizePx)),
		Icon("res-"+resKey, sizePx*58/100, false),
	)
}
