package ui

import (
	"fmt"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

const spritePath = "/static/sprite.svg"

// Shield renders a small SVG shield from the sprite. id is the symbol id
// without the "shield-" prefix (e.g. "crown", "sword"). active toggles the
// accent color via the shield--active modifier.
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

