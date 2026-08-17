package worldmap

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/components"
	. "maragu.dev/gomponents/html"

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

		guilds, err := h.queries.GetGuildsForKingdoms(r.Context(), kingdomIDs(kingdoms))
		if err != nil {
			log.Printf("map page: get guilds: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		KingdomLayout(r, "World Map", r.URL.Path, kingdom, mapContent(kingdoms, guildIndex(guilds), kingdom.ID, pageX, pageY, tileX0, tileY0, r.URL.Query().Get("highlight"))).Render(w)
	}
}

// kingdomIDs returns the IDs of the viewport kingdoms, for the guild lookup.
// An empty slice is fine (ANY of empty array returns no rows), so the guild
// query is skipped implicitly when the viewport is empty.
func kingdomIDs(ks []db.GetKingdomsInViewportRow) []int {
	ids := make([]int, 0, len(ks))
	for i := range ks {
		ids = append(ids, ks[i].ID)
	}
	return ids
}

// guildInfo is the guild affiliation shown for a single kingdom in the detail panel.
type guildInfo struct {
	Slug string
	Name string
}

// guildIndex builds a kingdom ID → guild lookup from the bulk query result.
func guildIndex(rows []db.GetGuildsForKingdomsRow) map[int]guildInfo {
	m := make(map[int]guildInfo, len(rows))
	for _, r := range rows {
		m[r.KingdomID] = guildInfo{Slug: r.GuildSlug, Name: r.GuildName}
	}
	return m
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
			sse.PatchElementGostar(findAlert(AlertError(errors.New("Please enter a kingdom name."))))
			return
		}

		k, err := h.queries.GetKingdomByName(r.Context(), name)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				sse.PatchElementGostar(findAlert(AlertError(errors.New("Kingdom not found."))))
				return
			}
			log.Printf("map find: get kingdom by name: %v", err)
			sse.PatchElementGostar(findAlert(AlertError(errors.New("Something went wrong. Please try again."))))
			return
		}

		pageX := k.X / game.PageSize
		pageY := k.Y / game.PageSize
		tileX0 := pageX * game.PageSize
		tileY0 := pageY * game.PageSize

		sse.PatchElementGostar(findAlert(nil))
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

func mapContent(kingdoms []db.GetKingdomsInViewportRow, guilds map[int]guildInfo, myKingdomID, pageX, pageY, tileX0, tileY0 int, highlight string) Node {
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
			Div(Class("card world-board"),
				Div(Class("board-head"),
					H1(Class("board-name"), Text(game.RegionAt(tileX0, tileY0).Name)),
					P(Class("board-region"), Text(fmt.Sprintf("Region %d · %d", pageX, pageY))),
				),
				Div(Class("board-stage"),
					flatBoard(kingdoms, myKingdomID, tileX0, tileY0, initialSelectedID, pageX, pageY),
				),
			),
			Div(Class("card world-cmd"),
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
						return kingdomDetail(k, guilds[k.ID], k.ID == myKingdomID, initialSelectedID)
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
		rowLabels[y] = Div(Class("map-flat-axis"), Text(strconv.Itoa(tileY0+y)))
	}

	// Top-left origin (ADR 0004): row 0 is the north (lowest tile Y). X
	// increases east, Y increases south.
	cells := make([]Node, 0, game.PageSize*game.PageSize)
	for y := range game.PageSize {
		for x := range game.PageSize {
			tx := tileX0 + x
			ty := tileY0 + y
			k := index[game.Coord{X: tx, Y: ty}]
			isOwn := k != nil && k.ID == myKingdomID
			cells = append(cells, flatCell(k, isOwn, initialSelectedID, tx, ty))
		}
	}

	return Div(Class("map-grid-container"),
		navLink("N", tileX0, tileY0-game.PageSize, pageY > 0),
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
		navLink("S", tileX0, tileY0+game.PageSize, pageY < maxPage),
	)
}

// flatCell renders a single tile in the flat board grid.
func flatCell(k *db.GetKingdomsInViewportRow, isOwn bool, initialSelectedID, tx, ty int) Node {
	tile := biomeFill(game.BiomeAt(tx, ty))
	selected := k != nil && k.ID == initialSelectedID

	var cellAttrs []Node
	if k != nil {
		cellAttrs = []Node{
			Data("kingdom-id", strconv.Itoa(k.ID)),
			// Reactive selected ring. The key is quoted (hyphens) and emitted raw
			// because ds.Class's object builder doesn't quote keys. Keeps the
			// ring in sync with $selected_kingdom_id for find-a-kingdom, marker
			// clicks, and the panel × button — not just the initial SSR paint.
			Attr("data-class", fmt.Sprintf(`{"map-cell--selected":$selected_kingdom_id===%d}`, k.ID)),
			ds.On("click", "$selected_kingdom_id = +el.dataset.kingdomId"),
			ds.On("keydown", "evt.key === 'Enter' && ($selected_kingdom_id = +el.dataset.kingdomId)"),
			TabIndex("0"),
			Role("button"),
		}
	}

	return Div(
		Classes{
			"map-cell":            true,
			"map-cell--own":       isOwn,
			"map-cell--occupied":  k != nil,
			"map-cell--clickable": k != nil,
			"map-cell--selected":  selected,
		},
		Style(fmt.Sprintf("--tile:%s", tile)),
		Group(cellAttrs),
		Div(Class("map-cell-content"),
			Iff(k != nil, func() Node { return crestMarker(k, isOwn) }),
		),
	)
}

// biomeFill maps a game.Biome to its heraldic wash CSS token (defined in
// 01-tokens.css). The board reads per-tile biomes via game.BiomeAt; the
// minimap reads each region's game.RegionDef.MainBiome.
func biomeFill(b game.Biome) string {
	switch b {
	case game.BiomePlains:
		return "var(--bio-p)"
	case game.BiomeForest:
		return "var(--bio-f)"
	case game.BiomeWater:
		return "var(--bio-w)"
	case game.BiomeMountain:
		return "var(--bio-r)" // retinted gray in 01-tokens.css
	case game.BiomeMarsh:
		return "var(--bio-m)"
	default:
		return "var(--bio-p)"
	}
}

// crestMarker renders the kingdom marker on an occupied tile.
// A flat crest sticker (.marker-crest) bearing a crown glyph. The medallion
// fill distinguishes owner (brass, rel-self) from others (parchment,
// rel-neutral); the diplomacy relation dot is parked until war state is wired.
func crestMarker(k *db.GetKingdomsInViewportRow, isOwn bool) Node {
	relClass := "rel-neutral"
	if isOwn {
		relClass = "rel-self"
	}
	return Div(
		Classes{
			"map-marker":        true,
			"map-marker--crest": true,
			relClass:            true,
		},
		Attr("data-tip", k.Name),
		Div(Class("marker-crest"), Icon("crown", 18, false)),
	)
}

// kingdomDetail renders the right-panel detail card for one kingdom.
// The card is hidden until $selected_kingdom_id matches k.ID.
// guild is the selected kingdom's guild affiliation (zero value when none).
func kingdomDetail(k db.GetKingdomsInViewportRow, guild guildInfo, isOwn bool, initialSelectedID int) Node {
	selected := k.ID == initialSelectedID
	msgHref := routes.KingdomMessagesComposePath + "?to=" + url.QueryEscape(k.Name)
	attackHref := routes.KingdomArmyPath + "?target=" + url.QueryEscape(k.Name) + "&action=attack"
	defendHref := routes.KingdomArmyPath + "?target=" + url.QueryEscape(k.Name) + "&action=defend"

	return Div(
		Class("kd-panel"),
		If(!selected, Style("display:none")),
		ds.Show(fmt.Sprintf("$selected_kingdom_id === %d", k.ID)),
		Div(Class("kd-head"),
			Div(Classes{"kd-crest": true, "is-self": isOwn}, Icon("crown", 28, false)),
			Div(
				P(Class("kd-name"), Text(k.Name)),
				P(Class("kd-sub"), Text(fmt.Sprintf("%d, %d", k.X, k.Y))),
			),
			Button(
				Type("button"),
				Class("kd-close"),
				Aria("label", "Deselect"),
				ds.On("click", "$selected_kingdom_id = 0"),
				Text("×"),
			),
		),
		guildLine(guild),
		If(!isOwn, Div(Class("kd-actions"),
			A(Class("btn kd-action-btn"), Href(msgHref), Text("Message")),
			A(Class("btn kd-action-btn btn--danger"), Href(attackHref), Text("Attack")),
			A(Class("btn kd-action-btn btn--accent"), Href(defendHref), Text("Defend")),
		)),
		Iff(isOwn, func() Node {
			return P(Class("kd-self-note"), Text("This is your home tile."))
		}),
	)
}

// guildLine renders the affiliation row beneath the kingdom profile. A linked
// guild name when affiliated, a muted note otherwise.
func guildLine(g guildInfo) Node {
	if g.Name == "" {
		return P(Class("kd-guild kd-guild--none"), Text("No guild affiliation"))
	}
	href := strings.ReplaceAll(routes.GuildViewPath, "{slug}", g.Slug)
	return P(Class("kd-guild"),
		Span(Class("kd-guild-label"), Text("Guild")),
		A(Class("kd-guild-name"), Href(href), Text(g.Name)),
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
	glyphs := map[string]string{
		"N": "▲", "S": "▼", "W": "◀", "E": "▶",
	}
	names := map[string]string{
		"N": "North", "S": "South", "W": "West", "E": "East",
	}
	label := glyphs[direction]
	name := names[direction]
	classes := Classes{
		"map-nav-btn":               true,
		"map-nav-btn--" + direction: true,
		"map-nav-btn--disabled":     !enabled,
	}
	if !enabled {
		return Span(classes, Role("button"), Aria("label", name), Aria("disabled", "true"), Text(label))
	}
	return A(Href(tileURL(targetTileX, targetTileY)), classes, Aria("label", name), Text(label))
}

// miniMap renders a compact PageCount×PageCount grid showing all world
// pages. The current page is highlighted; each other page is a navigation
// link. Top-left origin (ADR 0004): row 0 is py=0 (north), rendered at the
// top. Each cell's colour is the region's MainBiome — so the minimap always
// matches the dominant biome a player sees when they open that page.
func miniMap(pageX, pageY int) Node {
	cells := make([]Node, 0, game.PageCount*game.PageCount)
	for py := range game.PageCount {
		for px := range game.PageCount {
			region := game.RegionDefs[py][px]
			tile := biomeFill(region.MainBiome)
			tileStyle := Style("--tile:" + tile)
			label := fmt.Sprintf("%s (Region %d, %d)", region.Name, px, py)
			if px == pageX && py == pageY {
				cells = append(cells, Div(Class("map-minimap-cell map-minimap-cell--current"), Aria("label", label), Aria("current", "true"), tileStyle))
			} else {
				cells = append(cells, A(Href(tileURL(px*game.PageSize, py*game.PageSize)), Class("map-minimap-cell"), Aria("label", label), tileStyle))
			}
		}
	}
	return Div(Class("map-minimap"), Group(cells))
}

// findBar renders the kingdom search form in the command panel as a single
// attached row: a leading magnifier glyph, the name input, and an attached
// "Find" submit button sharing the field's edges.
func findBar() Node {
	return Div(Class("cmd-search"),
		Form(Class("cmd-search-row"),
			ds.On("submit", datastar.PostSSE(routes.KingdomMapFindPath)),
			Div(Class("cmd-search-field"),
				Input(
					Type("text"),
					Placeholder("Find a kingdom…"),
					Class("field"),
					Aria("label", "Kingdom name"),
					ds.Bind("find_name"),
				),
			),
			Button(
				Class("btn cmd-search-btn"),
				Type("submit"),
				ds.Indicator("find_fetching"),
				ds.Attr("disabled", "$find_fetching || $find_name === ''"),
				Text("Find"),
			),
		),
		findAlert(nil),
	)
}

// findAlert is the SSE patch target for find errors.
func findAlert(inner Node) Node {
	return AlertContainer("map-find-alert", inner)
}
