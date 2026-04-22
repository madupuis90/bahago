package worldmap

import (
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"strconv"

	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/components"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/game"
	. "bahago/internal/layout"
	"bahago/internal/router"
	"bahago/internal/routes"
)

// RegisterRoutes wires the world map handler into the router.
func RegisterRoutes(r router.Router, queries *db.Queries) {
	h := &handler{queries: queries}
	r.HandleFunc("GET "+routes.KingdomMapPath, h.handleMapPage())
}

type handler struct {
	queries *db.Queries
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

		KingdomLayout(r, "World Map", r.URL.Path, kingdom, mapContent(kingdoms, kingdom.ID, pageX, pageY, tileX0, tileY0)).Render(w)
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

func mapContent(kingdoms []db.GetKingdomsInViewportRow, myKingdomID, pageX, pageY, tileX0, tileY0 int) Node {
	return Group([]Node{
		H1(Class("page-title"), Text("World Map")),
		P(Class("map-coords"),
			Text(fmt.Sprintf("Page %d, %d  —  Tiles %d-%d, %d-%d",
				pageX+1, pageY+1,
				tileX0, tileX0+game.PageSize-1,
				tileY0, tileY0+game.PageSize-1,
			)),
		),
		mapGrid(kingdoms, myKingdomID, pageX, pageY, tileX0, tileY0),
	})
}

// mapGrid renders the 8×8 diamond isometric tile grid with surrounding nav links.
func mapGrid(kingdoms []db.GetKingdomsInViewportRow, myKingdomID, pageX, pageY, tileX0, tileY0 int) Node {
	index := make(map[game.Coord]*db.GetKingdomsInViewportRow, len(kingdoms))
	for i := range kingdoms {
		k := &kingdoms[i]
		index[game.Coord{X: k.X, Y: k.Y}] = k
	}

	const maxPage = game.PageCount - 1

	return Div(Class("map-grid-container"),
		navLink("N", tileX0, tileY0-game.PageSize, pageY > 0),
		Div(Class("map-grid-middle"),
			navLink("W", tileX0-game.PageSize, tileY0, pageX > 0),
			Div(Class("map-iso-container"),
				Div(Class("map-iso-stage"),
					Div(Class("map-grid"),
						Map(makeRange(game.PageSize*game.PageSize), func(i int) Node {
							col := i % game.PageSize
							row := i / game.PageSize
							tx := tileX0 + col
							ty := tileY0 + row
							k := index[game.Coord{X: tx, Y: ty}]
							return mapCell(k, k != nil && k.ID == myKingdomID)
						}),
					),
					Div(Class("map-box-wall--s")),
					Div(Class("map-box-wall--e")),
				),
			),
			navLink("E", tileX0+game.PageSize, tileY0, pageX < maxPage),
		),
		navLink("S", tileX0, tileY0+game.PageSize, pageY < maxPage),
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

func mapCell(kingdom *db.GetKingdomsInViewportRow, isOwn bool) Node {
	return Div(
		Classes{
			"map-cell":           true,
			"map-cell--own":      isOwn,
			"map-cell--occupied": kingdom != nil && !isOwn,
		},
		Div(Class("map-cell-content"),
			Iff(kingdom != nil, func() Node {
				icon := Span(Class("map-icon"), Text([]string{"🛖", "⛩️", "🏡"}[rand.IntN(3)]))
				if isOwn {
					icon = Span(Class("map-icon map-icon--own"), Text("🏰"))
				}
				return Group([]Node{
					icon,
					Span(Class("map-cell-tooltip"), Text(kingdom.Name)),
				})
			}),
		),
	)
}

// makeRange returns a slice of ints [0, n).
func makeRange(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i
	}
	return s
}
