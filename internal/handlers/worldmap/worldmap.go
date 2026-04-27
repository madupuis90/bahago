package worldmap

import (
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
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
	. "bahago/internal/layout"
	"bahago/internal/router"
	"bahago/internal/routes"
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
	return Div(
		ds.Signals(map[string]any{
			"selected_kingdom_name": "",
		}),
		H1(Class("page-title"), Text("World Map")),
		Div(Class("map-info panel"),
			P(Class("map-coords"),
				Text(fmt.Sprintf("X: %d-%d, Y: %d-%d", tileX0, tileX0+game.PageSize-1, tileY0, tileY0+game.PageSize-1)),
			),
			findBar(),
		),
		Div(Class("map-main-row"),
			mapGrid(kingdoms, myKingdomID, pageX, pageY, tileX0, tileY0, highlight),
			miniMap(pageX, pageY),
		),
		kingdomPopup(),
	)
}

// mapGrid renders the 8×8 diamond isometric tile grid with surrounding nav links.
func mapGrid(kingdoms []db.GetKingdomsInViewportRow, myKingdomID, pageX, pageY, tileX0, tileY0 int, highlight string) Node {
	index := make(map[game.Coord]*db.GetKingdomsInViewportRow, len(kingdoms))
	for i := range kingdoms {
		k := &kingdoms[i]
		index[game.Coord{X: k.X, Y: k.Y}] = k
	}

	const maxPage = game.PageCount - 1

	return Div(Class("map-grid-container"),
		navLink("N", tileX0, tileY0+game.PageSize, pageY < maxPage),
		Div(Class("map-grid-middle"),
			navLink("W", tileX0-game.PageSize, tileY0, pageX > 0),
			Div(Class("map-iso-container"),
				Div(Class("map-iso-stage"),
					Div(Class("map-grid"),
						Map(makeRange((game.PageSize+2)*(game.PageSize+2)), func(i int) Node {
							col := i % (game.PageSize + 2)
							row := i / (game.PageSize + 2)
							// Top-left: X axis name
							if row == game.PageSize+1 && col == 0 {
								return axisNameLabelCell("X")
							}
							// Top-right: Y axis name
							if row == 0 && col == game.PageSize+1 {
								return axisNameLabelCell("Y")
							}
							// Top row (rest): blank
							if row == 0 {
								return Div(Class("map-axis-label"))
							}
							// Left column (rest): blank
							if col == 0 {
								return Div(Class("map-axis-label"))
							}
							// Right column: Y coord numbers, blank at bottom corner
							if col == game.PageSize+1 {
								if row == game.PageSize+1 {
									return Div(Class("map-axis-label"))
								}
								return axisLabelCell(tileY0 + (game.PageSize - row))
							}
							// Bottom row: X coord numbers (cols 1..PageSize)
							if row == game.PageSize+1 {
								return axisLabelCell(tileX0 + col - 1)
							}
							// Tile cells (rows 1..PageSize, cols 1..PageSize)
							tx := tileX0 + col - 1
							ty := tileY0 + (game.PageSize - row)
							k := index[game.Coord{X: tx, Y: ty}]
							isOwn := k != nil && k.ID == myKingdomID
							isHighlighted := k != nil && strings.EqualFold(k.Name, highlight)
							return mapCell(k, isOwn, isHighlighted)
						}),
					),
					Div(Class("map-box-wall--s")),
					Div(Class("map-box-wall--e")),
				),
			),
			navLink("E", tileX0+game.PageSize, tileY0, pageX < maxPage),
		),
		navLink("S", tileX0, tileY0-game.PageSize, pageY > 0),
	)
}

// navLink renders a navigation link to an adjacent page. When disabled (at a
// world edge) it renders a non-interactive span instead.
func navLink(direction string, targetTileX, targetTileY int, enabled bool) Node {
	labels := map[string]string{
		"N": "↑", "S": "↓", "W": "←", "E": "→",
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

func mapCell(kingdom *db.GetKingdomsInViewportRow, isOwn, isHighlighted bool) Node {
	clickable := kingdom != nil && !isOwn
	var nameAttr, onClickAttr Node
	if clickable {
		nameAttr = Data("kingdom-name", kingdom.Name)
		onClickAttr = ds.On("click", "$selected_kingdom_name = el.dataset.kingdomName")
	}
	return Div(
		Classes{
			"map-cell":              true,
			"map-cell--own":         isOwn,
			"map-cell--occupied":    kingdom != nil && !isOwn,
			"map-cell--clickable":   clickable,
			"map-cell--highlighted": isHighlighted,
		},
		nameAttr,
		onClickAttr,
		Div(Class("map-cell-content"),
			Iff(kingdom != nil, func() Node {
				icon := Span(Class("map-icon"), Text([]string{"🛖", "⛩️", "🏡"}[rand.IntN(3)]))
				if isOwn {
					icon = Span(Class("map-icon map-icon--own"), Text("🏰"))
				}
				return Group([]Node{
					icon,
					Span(Class("map-cell-tooltip"), Text(fmt.Sprintf("%s (%d, %d)", kingdom.Name, kingdom.X, kingdom.Y))),
				})
			}),
		),
	)
}

// miniMap renders a compact PageCount×PageCount grid showing all world pages.
// The current page is highlighted; each other page is a navigation link.
// Rows are rendered top-to-bottom with py = PageCount-1-row so that py=0
// sits at the visual bottom, matching the Y-flipped main grid.
func miniMap(pageX, pageY int) Node {
	cells := make([]Node, 0, game.PageCount*game.PageCount)
	for row := 0; row < game.PageCount; row++ {
		py := game.PageCount - 1 - row
		for px := 0; px < game.PageCount; px++ {
			if px == pageX && py == pageY {
				cell := Div(Class("map-minimap-cell map-minimap-cell--current"))
				cells = append(cells, cell)
			} else {
				cell := A(Href(tileURL(px*game.PageSize, py*game.PageSize)), Class("map-minimap-cell"))
				cells = append(cells, cell)
			}
		}
	}
	return Div(Class("map-minimap"), Group(cells))
}

// makeRange returns a slice of ints [0, n).
func makeRange(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i
	}
	return s
}

// axisLabelCell renders a coordinate number on the grid edge.
func axisLabelCell(coord int) Node {
	return Div(Class("map-axis-label"),
		Div(Class("map-axis-label-content"), Text(strconv.Itoa(coord))),
	)
}

// axisNameLabelCell renders an axis name label ("X" or "Y") on the outer edge.
func axisNameLabelCell(name string) Node {
	return Div(Class("map-axis-label"),
		Div(Class("map-axis-label-content map-axis-label-content--name"), Text(name)),
	)
}

// findBar renders the kingdom search input and button.
func findBar() Node {
	return Div(Class("map-find-bar"),
		Form(
			ds.On("submit", datastar.PostSSE(routes.KingdomMapFindPath)),
			Input(
				Type("text"),
				Placeholder("Kingdom name"),
				Class("map-find-input"),
				ds.Bind("find_name"),
			),
			Button(
				Class("btn"),
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

// kingdomPopup renders a fixed overlay popup that appears when a non-own kingdom
// tile is clicked. Three action buttons navigate to compose, attack, or defend
// pages with the target kingdom name pre-filled via query params.
func kingdomPopup() Node {
	msgHref := "'" + routes.KingdomMessagesComposePath + "?to='+encodeURIComponent($selected_kingdom_name)"
	attackHref := "'" + routes.KingdomArmyPath + "?target='+encodeURIComponent($selected_kingdom_name)+'&action=attack'"
	defendHref := "'" + routes.KingdomArmyPath + "?target='+encodeURIComponent($selected_kingdom_name)+'&action=defend'"

	return Div(Class("map-popup-overlay"),
		Style("display:none"),
		ds.Show("$selected_kingdom_name !== ''"),
		ds.On("click", "$selected_kingdom_name = ''"),
		Div(Class("map-popup"),
			ds.On("click", "{}", ds.ModifierStop),
			Div(Class("map-popup-header"),
				P(Class("map-popup-name"), ds.Text("$selected_kingdom_name")),
				Button(Class("map-popup-close"), ds.On("click", "$selected_kingdom_name = ''"), Text("×")),
			),
			Div(Class("map-popup-actions"),
				A(Class("btn map-popup-btn"), Href("#"), ds.Attr("href", msgHref), Text("✉️ Send Message")),
				A(Class("btn map-popup-btn map-popup-btn--attack"), Href("#"), ds.Attr("href", attackHref), Text("⚔️ Attack")),
				A(Class("btn map-popup-btn map-popup-btn--defend"), Href("#"), ds.Attr("href", defendHref), Text("🛡 Defend")),
			),
		),
	)
}
