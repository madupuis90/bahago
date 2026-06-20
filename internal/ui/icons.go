package ui

import (
	"fmt"
	"strconv"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/components"
	. "maragu.dev/gomponents/html"
)

const spritePath = "/static/sprite.svg"

// Icon renders only the inner glyph of a sprite symbol — no shield background.
// id is the full symbol ID (e.g. "shield-crown", "res-wood"). active toggles
// the accent color via the icon--active modifier.
func Icon(id string, sizePx int, active bool) Node {
	cls := "icon"
	if active {
		cls = "icon icon--active"
	}
	return El("svg",
		Class(cls),
		Attr("width", fmt.Sprintf("%d", sizePx)),
		Attr("height", fmt.Sprintf("%d", sizePx*23/20)),
		Attr("aria-hidden", "true"),
		El("use", Attr("href", spritePath+"#"+id)),
	)
}

// Shield renders a sprite icon inside its shield background. id is the symbol
// short name without the "shield-" prefix (e.g. "crown", "sword"). active
// toggles the accent color via the shield--active modifier.
func Shield(id string, sizePx int, active bool) Node {
	cls := "shield"
	if active {
		cls = "shield shield--active"
	}
	return El("svg",
		Class(cls),
		Attr("width", fmt.Sprintf("%d", sizePx)),
		Attr("height", fmt.Sprintf("%d", sizePx*23/20)),
		Attr("aria-hidden", "true"),
		El("use", Attr("href", spritePath+"#shield-"+id)),
	)
}

// Hourglass renders the small hourglass icon used in the tick chip.
// Colour inherits via currentColor — set it on the parent.
func Hourglass(widthPx int) Node {
	return El("svg",
		Class("hourglass"),
		Attr("width", fmt.Sprintf("%d", widthPx)),
		Attr("height", fmt.Sprintf("%d", widthPx*14/10)),
		Attr("viewBox", "0 0 10 14"),
		Attr("aria-hidden", "true"),
		El("use", Attr("href", spritePath+"#sandglass")),
	)
}

// gemIDToResKey maps a gem colour id (tree/mountain/wheat/flame/sun/star) to
// its resource symbol key (wood/stone/food/mana/devotion/knowledge) defined in
// sprite.svg as #res-<key>. Used by ResourceGem / ResourceGlyph to render the
// canonical ligne-claire resource artwork (tree · mountain · carrot · drop ·
// sun · star) instead of the legacy heraldic shield glyphs.
var gemIDToResKey = map[string]string{
	"tree":     "wood",
	"mountain": "stone",
	"wheat":    "food",
	"flame":    "mana",
	"sun":      "devotion",
	"star":     "knowledge",
}

// ResourceGlyph renders the ligne-claire resource symbol (#res-<resKey>) from
// sprite.svg — the canonical resource artwork. Square 24×24 viewBox; colour
// inherits via currentColor (set to #fff on a .gem).
func ResourceGlyph(resKey string, sizePx int) Node {
	sz := strconv.Itoa(sizePx)
	return El("svg",
		Class("gly"),
		Attr("width", sz),
		Attr("height", sz),
		Attr("viewBox", "0 0 24 24"),
		Attr("aria-hidden", "true"),
		El("use", Attr("href", spritePath+"#res-"+resKey)),
	)
}

// ResourceGem renders a filled resource gem: the coloured disc (.gem-<gemID>)
// with the white ligne-claire resource glyph on top. gemID is the gem colour
// id (tree/mountain/wheat/flame/sun/star); sizePx is the gem diameter. Use
// this anywhere a resource is shown as a gem (chrome pills, allocation roster,
// units stat pills, building cost chips / banner).
func ResourceGem(gemID string, sizePx int) Node {
	resKey, ok := gemIDToResKey[gemID]
	if !ok {
		resKey = gemID // graceful fallback: assume caller passed a res key
	}
	return Span(
		Classes{"gem": true, "gem-" + gemID: true},
		Style(fmt.Sprintf("width:%dpx;height:%dpx;min-width:%dpx", sizePx, sizePx, sizePx)),
		ResourceGlyph(resKey, sizePx*58/100),
	)
}

