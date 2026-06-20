package worldmap

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/components"
	. "maragu.dev/gomponents/html"

	"github.com/jackc/pgx/v5"
	"github.com/starfederation/datastar-go/datastar"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/game"
	"bahago/internal/router"
	"bahago/internal/routes"
	. "bahago/internal/ui"
)

// RegisterRoutes wires the world map handler into the router.
func RegisterRoutes(r router.Router, queries db.Querier) {
	h := &handler{queries: queries}
	r.HandleFunc("GET "+routes.KingdomMapPath, h.handleMapPage())
	r.HandleFunc("POST "+routes.KingdomMapFindPath, h.handleMapFind())
}

type handler struct {
	queries db.Querier
}

// handleMapPage reads optional ?x=N&y=M tile-space query params.
// Both default to the current kingdom's position when absent or invalid.
// x and y are floored to the containing page boundary, so any tile within
// a page produces the same viewport (e.g. x=1,y=1 and x=7,y=7 show the same page).
func (h *handler) handleMapPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		tileX, tileY := parseTileParams(r, kingdom.X, kingdom.Y)
		pageX := tileX / game.PageSize
		pageY := tileY / game.PageSize
		tileX0 := pageX * game.PageSize
		tileY0 := pageY * game.PageSize

		kingdoms, err := h.queries.GetKingdomsInViewport(r.Context(), viewportParams(tileX0, tileY0))
		if err != nil {
			log.Printf("map page: get kingdoms: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		KingdomLayout(r, "World Map", r.URL.Path, kingdom, mapContent(kingdoms, kingdom.ID, pageX, pageY, tileX0, tileY0, r.URL.Query().Get("highlight"))).Render(w)
	}
}

type findInput struct {
	Name string `json:"find_name"`
}

// handleMapFind looks up a kingdom by name and returns a redirect URL signal
// pointing to the map page containing that kingdom's tile.
func (h *handler) handleMapFind() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input := &findInput{}
		if err := datastar.ReadSignals(r, input); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		sse := datastar.NewSSE(w, r)

		name := strings.TrimSpace(input.Name)
		if name == "" {
			sse.PatchElementGostar(findAlertComponent(P(Class("alert-error"), Text("Please enter a kingdom name."))))
			return
		}

		k, err := h.queries.GetKingdomByName(r.Context(), name)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				sse.PatchElementGostar(findAlertComponent(P(Class("alert-error"), Text("Kingdom not found."))))
				return
			}
			log.Printf("map find: get kingdom by name: %v", err)
			sse.PatchElementGostar(findAlertComponent(P(Class("alert-error"), Text("Something went wrong. Please try again."))))
			return
		}

		pageX := k.X / game.PageSize
		pageY := k.Y / game.PageSize
		tileX0 := pageX * game.PageSize
		tileY0 := pageY * game.PageSize

		sse.PatchElementGostar(findAlertComponent(nil))
		sse.Redirect(tileURL(tileX0, tileY0) + "&highlight=" + url.QueryEscape(k.Name))
	}
}

// parseTileParams reads ?x and ?y from the request. Values outside [0, WorldSize)
// or non-numeric values fall back to the provided defaults.
func parseTileParams(r *http.Request, defaultX, defaultY int) (int, int) {
	parse := func(key string, def int) int {
		s := r.URL.Query().Get(key)
		if s == "" {
			return def
		}
		n, err := strconv.Atoi(s)
		if err != nil || n < 0 || n >= game.WorldSize {
			return def
		}
		return n
	}
	return parse("x", defaultX), parse("y", defaultY)
}

// viewportParams converts tile origin coordinates to tile bounds for the DB query.
func viewportParams(tileX0, tileY0 int) db.GetKingdomsInViewportParams {
	return db.GetKingdomsInViewportParams{
		X:   tileX0,
		X_2: tileX0 + game.PageSize - 1,
		Y:   tileY0,
		Y_2: tileY0 + game.PageSize - 1,
	}
}

// tileURL returns a /kingdom/map URL for the page containing (tileX, tileY).
func tileURL(tileX, tileY int) string {
	return fmt.Sprintf("%s?x=%d&y=%d", routes.KingdomMapPath, tileX, tileY)
}

func mapContent(kingdoms []db.GetKingdomsInViewportRow, myKingdomID, pageX, pageY, tileX0, tileY0 int, highlight string) Node {
	initialSelectedID := 0
	for _, k := range kingdoms {
		if strings.EqualFold(k.Name, highlight) {
			initialSelectedID = k.ID
			break
		}
	}

	return Div(Class("world"),
		ds.Signals(map[string]any{
			"selected_kingdom_id": initialSelectedID,
		}),
		Div(Class("world-main"),
			Div(Class("world-board"),
				Div(Class("board-head"),
					H1(Class("board-name"), Text("World Map")),
					P(Class("board-region"), Text(fmt.Sprintf("Page %d · %d", pageX, pageY))),
				),
				Div(Class("board-stage"),
					flatBoard(kingdoms, myKingdomID, tileX0, tileY0, initialSelectedID, pageX, pageY),
				),
			),
			Div(Class("world-cmd"),
				Div(Class("cmd-nav"),
					Div(Class("cmd-nav-bar"),
						Span(Class("cmd-section-title"), Text("Region")),
						Span(Class("cmd-coords"),
							Text(fmt.Sprintf("X %d–%d  Y %d–%d", tileX0, tileX0+game.PageSize-1, tileY0, tileY0+game.PageSize-1)),
						),
					),
					Div(Class("cmd-minimap-wrap"), miniMap(pageX, pageY)),
					findBar(),
				),
				Div(Class("cmd-detail"),
					Div(
						If(initialSelectedID != 0, Style("display:none")),
						ds.Show("$selected_kingdom_id === 0"),
						emptyState(),
					),
					Map(kingdoms, func(k db.GetKingdomsInViewportRow) Node {
						return kingdomDetail(k, k.ID == myKingdomID, initialSelectedID)
					}),
				),
			),
		),
	)
}

// flatBoard renders the 8×8 flat tile grid with surrounding nav buttons and axis labels.
func flatBoard(kingdoms []db.GetKingdomsInViewportRow, myKingdomID, tileX0, tileY0, initialSelectedID, pageX, pageY int) Node {
	index := make(map[game.Coord]*db.GetKingdomsInViewportRow, len(kingdoms))
	for i := range kingdoms {
		k := &kingdoms[i]
		index[game.Coord{X: k.X, Y: k.Y}] = k
	}

	const maxPage = game.PageCount - 1

	colLabels := make([]Node, game.PageSize)
	for x := range game.PageSize {
		colLabels[x] = Div(Class("map-flat-axis"), Text(strconv.Itoa(tileX0+x)))
	}

	rowLabels := make([]Node, game.PageSize)
	for y := range game.PageSize {
		rowLabels[y] = Div(Class("map-flat-axis"), Text(strconv.Itoa(tileY0+(game.PageSize-1-y))))
	}

	// Visual y=0 is the top row, which corresponds to the highest tile Y coordinate.
	cells := make([]Node, 0, game.PageSize*game.PageSize)
	for y := range game.PageSize {
		for x := range game.PageSize {
			tx := tileX0 + x
			ty := tileY0 + (game.PageSize - 1 - y)
			k := index[game.Coord{X: tx, Y: ty}]
			isOwn := k != nil && k.ID == myKingdomID
			cells = append(cells, flatCell(k, isOwn, initialSelectedID, tx, ty))
		}
	}

	return Div(Class("map-grid-container"),
		navLink("N", tileX0, tileY0+game.PageSize, pageY < maxPage),
		Div(Class("map-grid-middle"),
			navLink("W", tileX0-game.PageSize, tileY0, pageX > 0),
			Div(Class("map-flat"),
				Div(Class("map-flat-corner")),
				Div(Class("map-flat-cols"), Group(colLabels)),
				Div(Class("map-flat-rows"), Group(rowLabels)),
				Div(Class("map-flat-grid"), Group(cells)),
			),
			navLink("E", tileX0+game.PageSize, tileY0, pageX < maxPage),
		),
		navLink("S", tileX0, tileY0-game.PageSize, pageY > 0),
	)
}

// flatCell renders a single tile in the flat board grid.
func flatCell(k *db.GetKingdomsInViewportRow, isOwn bool, initialSelectedID, tx, ty int) Node {
	tile, tileDeep := biomeColor(tx, ty)
	selected := k != nil && k.ID == initialSelectedID

	var dataAttr, onClickAttr Node
	if k != nil {
		dataAttr = Data("kingdom-id", strconv.Itoa(k.ID))
		onClickAttr = ds.On("click", "$selected_kingdom_id = +el.dataset.kingdomId")
	}

	return Div(
		Classes{
			"map-cell":            true,
			"map-cell--own":       isOwn,
			"map-cell--occupied":  k != nil,
			"map-cell--clickable": k != nil,
			"map-cell--selected":  selected,
		},
		Style(fmt.Sprintf("--tile:%s;--tile-deep:%s", tile, tileDeep)),
		dataAttr,
		onClickAttr,
		Div(Class("map-cell-content"),
			Iff(k != nil, func() Node { return crestMarker(k, isOwn) }),
		),
	)
}

// biomeTints provides deterministic terrain colours for map tiles.
// Each entry is [tile, tile-deep] (light face, shadow face).
var biomeTints = [][2]string{
	{"#a9c47e", "#7f9d5b"},
	{"#7f9d5b", "#5c7a40"},
	{"#c6b478", "#a08a50"},
	{"#8fb4c4", "#6a8a9a"},
	{"#9aaa7c", "#728a5a"},
	{"#ab9d82", "#8a7a60"},
}

// biomeColor returns CSS colour values for a tile at world position (x, y).
func biomeColor(x, y int) (tile, tileDeep string) {
	b := biomeTints[(x*3+y*5)%len(biomeTints)]
	return b[0], b[1]
}

// crestMarker renders the kingdom marker on an occupied tile.
// The Coin at 14px degrades to a bold relation-coloured dot; the inner
// symbol is omitted at map scale.
func crestMarker(k *db.GetKingdomsInViewportRow, isOwn bool) Node {
	relClass := "rel-neutral"
	dotColor := "#3a6390"
	if isOwn {
		relClass = "rel-self"
		dotColor = "var(--chrome-accent)"
	}
	return Div(
		Classes{
			"map-marker":       true,
			"map-marker--crest": true,
			relClass:           true,
		},
		Attr("data-tip", k.Name),
		Span(Class("marker-dot"), Style("background:"+dotColor)),
		Span(Class("marker-rel-dot"), Style("background:"+dotColor)),
	)
}

// kingdomDetail renders the right-panel detail card for one kingdom.
// The card is hidden until $selected_kingdom_id matches k.ID.
func kingdomDetail(k db.GetKingdomsInViewportRow, isOwn bool, initialSelectedID int) Node {
	selected := k.ID == initialSelectedID
	attackHref := routes.KingdomArmyPath + "?target=" + url.QueryEscape(k.Name) + "&action=attack"
	defendHref := routes.KingdomArmyPath + "?target=" + url.QueryEscape(k.Name) + "&action=defend"
	msgHref := routes.KingdomMessagesComposePath + "?to=" + url.QueryEscape(k.Name)

	return Div(
		Class("kd-panel"),
		If(!selected, Style("display:none")),
		ds.Show(fmt.Sprintf("$selected_kingdom_id === %d", k.ID)),
		Div(Class("kd-head"),
			Div(Classes{"kd-crest": true, "is-self": isOwn}, Span(Class("kd-crest-dot"))),
			Div(
				P(Class("kd-name"), Text(k.Name)),
				P(Class("kd-sub"), Text(fmt.Sprintf("%d, %d", k.X, k.Y))),
			),
		),
		If(!isOwn, Div(Class("kd-actions"),
			A(Class("btn kd-action-btn kd-action-btn--attack"), Href(attackHref), Text("Attack")),
			A(Class("btn kd-action-btn kd-action-btn--defend"), Href(defendHref), Text("Defend")),
			A(Class("btn kd-action-btn"), Href(msgHref), Text("Message")),
		)),
		Iff(isOwn, func() Node {
			return P(Class("kd-self-note"), Text("This is your home tile."))
		}),
	)
}

// emptyState renders the prompt shown in the command panel before any tile is selected.
func emptyState() Node {
	return Div(Class("cmd-empty"),
		compassRose("cmd-empty-rose"),
		P(Class("cmd-empty-h"), Text("Select a tile")),
		P(Class("cmd-empty-sub"), Text("Click any marker on the map to view kingdom details.")),
	)
}

// compassRose renders a decorative SVG compass rose.
func compassRose(class string) Node {
	return El("svg",
		Attr("viewBox", "0 0 40 40"),
		Attr("width", "40"),
		Attr("height", "40"),
		Class(class),
		Attr("fill", "none"),
		Attr("aria-hidden", "true"),
		Style("display:block"),
		El("circle", Attr("cx", "20"), Attr("cy", "20"), Attr("r", "17.5"), Attr("stroke", "currentColor"), Attr("stroke-width", "1")),
		El("circle", Attr("cx", "20"), Attr("cy", "20"), Attr("r", "12"), Attr("stroke", "currentColor"), Attr("stroke-width", ".6"), Attr("opacity", ".45")),
		El("path", Attr("d", "M20 3 L23.5 20 L20 37 L16.5 20 Z"), Attr("fill", "currentColor"), Attr("opacity", ".48")),
		El("path", Attr("d", "M3 20 L20 16.5 L37 20 L20 23.5 Z"), Attr("fill", "currentColor"), Attr("opacity", ".22")),
		El("text",
			Attr("x", "20"), Attr("y", "10.5"),
			Attr("text-anchor", "middle"),
			Attr("font-size", "6.5"),
			Attr("font-weight", "700"),
			Attr("fill", "currentColor"),
			Attr("font-family", "Cinzel, serif"),
			Text("N"),
		),
	)
}

// navLink renders a navigation link to an adjacent page. When disabled (at a
// world edge) it renders a non-interactive span instead.
func navLink(direction string, targetTileX, targetTileY int, enabled bool) Node {
	labels := map[string]string{
		"N": "▲", "S": "▼", "W": "◀", "E": "▶",
	}
	label := labels[direction]
	classes := Classes{
		"map-nav-btn":               true,
		"map-nav-btn--" + direction: true,
		"map-nav-btn--disabled":     !enabled,
	}
	if !enabled {
		return Span(classes, Text(label))
	}
	return A(Href(tileURL(targetTileX, targetTileY)), classes, Text(label))
}

// miniMap renders a compact PageCount×PageCount grid showing all world pages.
// The current page is highlighted; each other page is a navigation link.
// Rows are rendered top-to-bottom with py = PageCount-1-row so that py=0
// sits at the visual bottom, matching the Y-flipped main grid.
func miniMap(pageX, pageY int) Node {
	cells := make([]Node, 0, game.PageCount*game.PageCount)
	for row := range game.PageCount {
		py := game.PageCount - 1 - row
		for px := range game.PageCount {
			tile, _ := biomeColor(px, py)
			tileStyle := Style("--tile:" + tile)
			if px == pageX && py == pageY {
				cells = append(cells, Div(Class("map-minimap-cell map-minimap-cell--current"), tileStyle))
			} else {
				cells = append(cells, A(Href(tileURL(px*game.PageSize, py*game.PageSize)), Class("map-minimap-cell"), tileStyle))
			}
		}
	}
	return Div(Class("map-minimap"), Group(cells))
}

// findBar renders the kingdom search form in the command panel.
func findBar() Node {
	return Div(Class("cmd-search"),
		Form(
			ds.On("submit", datastar.PostSSE(routes.KingdomMapFindPath)),
			Input(
				Type("text"),
				Placeholder("Kingdom name…"),
				Class("cmd-search-input"),
				ds.Bind("find_name"),
			),
			Button(
				Class("btn cmd-search-btn"),
				Type("submit"),
				Disabled(),
				ds.Indicator("find_fetching"),
				ds.Attr("disabled", "$find_fetching || $find_name === ''"),
				Text("Find"),
			),
		),
		findAlertComponent(nil),
	)
}

// findAlertComponent is the SSE patch target for find errors.
func findAlertComponent(inner Node) Node {
	return Div(ID("map-find-alert"), inner)
}
