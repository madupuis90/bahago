package game

// Biome is a terrain class assigned to each tile of the World Map. Five exist.
// A Region's dominant Biome drives its minimap colour; per-tile Biomes drive
// the board's tile fills. Biome is currently cosmetic — no gameplay mechanic
// reads it. See ADR 0003.
type Biome string

// Biome constants. Singleton nouns, parallel to UnitRecruit / BuildingFarm.
const (
	BiomePlains   Biome = "plains"
	BiomeForest   Biome = "forest"
	BiomeWater    Biome = "water"
	BiomeMountain Biome = "mountain"
	BiomeMarsh    Biome = "marsh"
)

// RegionDef describes one of the 64 pages of the World Map. It is static
// configuration in the spirit of UnitDefs / BuildingDefs — nothing is stored
// in the DB.
//
// Coordinates use a top-left origin (ADR 0004): (0,0) is the north-west
// corner; X increases east, Y increases south. RegionDefs is indexed
// [py][px], so RegionDefs[0][0] is the north-west region and RegionDefs[7][7]
// is the south-east.
//
// Tiles is row-major, 8×8, top-left within the region: index
// (ty%PageSize)*PageSize + (tx%PageSize). The per-tile layout is derived from
// the MainBiome grid via a transition rule, so the world reads as coherent
// terrain rather than uniform blocks — with enough variance that adjacent
// regions don't look identical:
//   - Each region's interior is its MainBiome.
//   - For each edge whose neighbour's MainBiome differs, a tapered WEDGE of
//     that neighbour's biome enters the region (a triangle, wide along the
//     edge, narrowing to a point a few tiles inward). Wedge depth, width and
//     base centre are randomised per (region, edge) so regions vary.
//   - For each corner where the diagonal neighbour's MainBiome differs, a small
//     CONE (triangle apex at the corner) intrudes inward.
//   - Water regions carry a couple of interior marsh flecks for wetland feel.
//   - The 2×2 Mountain cluster in the north-east gets an injected Forest wedge
//     per region, since it has no Forest neighbour to trigger the edge rule.
//
// MainBiome need not be a strict majority — roughly 50%+ is enough and keeps
// the minimap reading the same biome while letting each page show organic
// transitions. The dominance test in region_defs_test.go guards that
// MainBiome stays the most frequent biome of each region's tiles.
type RegionDef struct {
	// Name is the human-readable name shown in the board header and minimap tooltip.
	Name string

	// MainBiome is the dominant biome of the region. It drives the minimap
	// colour and is expected to be the most frequent value in Tiles; the test
	// in region_defs_test.go guards against drift.
	MainBiome Biome

	// Tiles is the per-tile biome layout for the region's 64 tiles, row-major
	// top-left.
	Tiles [PageSize * PageSize]Biome
}

// Biome shorthand for the tile literals below: pl, fo, wa, mt, ma.
var (
	pl = BiomePlains
	fo = BiomeForest
	wa = BiomeWater
	mt = BiomeMountain
	ma = BiomeMarsh
)

// RegionDefs is indexed [py][px]. See the package doc and ADRs 0003/0004.
//
// Macro layout (MainBiome grid) — a C-shaped water course opens EAST, enclosing
// a Plains basin at the centre (statistical spawn target); Forest lies to the
// west; Mountain bands frame the north and south edges; a 2×2 Mountain region
// cluster stands alone in the north-east interior.
//
//	px:   0  1  2  3  4  5  6  7
//	py=0  M  M  M  M  M  M  M  M      north edge: Mountains
//	py=1  M  F  W  W  P  P  M  M      top of C arc (px=2,3); mountain fringe
//	py=2  F  F  W  P  P  P  P  P      Forest (west) | C left arm | Plains basin
//	py=3  F  F  W  P  P  P  P  P      Plains basin (spawn centre)
//	py=4  F  F  W  P  P  P  M  M      Plains | 2×2 Mountain (north half)
//	py=5  F  F  W  P  P  P  M  M      Plains | 2×2 Mountain (south half)
//	py=6  M  F  W  W  P  P  P  M      bottom of C arc (px=2,3); south fringe
//	py=7  M  M  M  M  P  P  M  M      south edge: Mountains
var RegionDefs = [PageCount][PageCount]RegionDef{

	// ── py=0 ─────────────────────────────────────────────────────
	{
		{ // px=0
			Name: "Frostpeak Reach", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, fo,
				mt, mt, mt, mt, mt, mt, fo, fo,
			},
		},
		{ // px=1
			Name: "Greypeak Marches", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, fo, wa,
				mt, mt, mt, mt, fo, fo, fo, fo,
			},
		},
		{ // px=2
			Name: "Stormhold Heights", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, wa, mt, mt, mt, mt, mt, mt,
				wa, wa, wa, wa, mt, mt, mt, wa,
				wa, wa, wa, wa, wa, mt, wa, wa,
			},
		},
		{ // px=3
			Name: "Irongate Climb", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, wa, mt, mt, mt, mt,
				mt, mt, mt, wa, mt, mt, mt, mt,
				mt, mt, wa, wa, wa, mt, mt, mt,
				wa, wa, wa, wa, wa, wa, mt, pl,
				wa, wa, wa, wa, wa, wa, pl, pl,
			},
		},
		{ // px=4
			Name: "North Vale Pass", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, pl, mt, mt, mt, mt, mt, mt,
				pl, pl, pl, mt, mt, mt, mt, pl,
				pl, pl, pl, pl, mt, mt, pl, pl,
			},
		},
		{ // px=5
			Name: "Highwatch Moor", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, pl, mt, mt,
				pl, mt, mt, mt, pl, pl, pl, mt,
				pl, pl, mt, pl, pl, pl, pl, pl,
			},
		},
		{ // px=6
			Name: "Blackspine Ridge", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, fo, mt, mt, mt, mt,
				pl, mt, mt, fo, mt, mt, mt, mt,
				pl, pl, fo, fo, fo, mt, mt, mt,
			},
		},
		{ // px=7
			Name: "Wraithhold Crag", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, fo, mt,
				mt, mt, mt, mt, fo, fo, fo, fo,
			},
		},
	},
	// ── py=1 ─────────────────────────────────────────────────────
	{
		{ // px=0
			Name: "Stonefell Marches", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, fo,
				mt, mt, mt, mt, mt, mt, fo, fo,
				mt, mt, mt, mt, mt, fo, fo, fo,
				mt, mt, mt, mt, mt, fo, fo, fo,
				mt, mt, mt, mt, fo, fo, fo, fo,
			},
		},
		{ // px=1
			Name: "Pinewatch Fringe", MainBiome: BiomeForest,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, fo, mt, wa,
				mt, fo, mt, fo, fo, fo, wa, wa,
				mt, fo, fo, fo, fo, fo, wa, wa,
				mt, mt, fo, fo, fo, wa, wa, wa,
				mt, fo, fo, fo, fo, fo, wa, wa,
				mt, fo, fo, fo, fo, fo, wa, wa,
				fo, fo, fo, fo, fo, fo, fo, wa,
				fo, fo, fo, fo, fo, fo, wa, wa,
			},
		},
		{ // px=2
			Name: "Mistvein Headwaters", MainBiome: BiomeWater,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, mt, mt, mt,
				fo, wa, mt, mt, mt, wa, wa, mt,
				fo, fo, wa, ma, wa, wa, wa, wa,
				fo, wa, wa, wa, wa, wa, wa, wa,
				wa, wa, wa, wa, wa, wa, wa, wa,
				wa, wa, wa, wa, ma, wa, wa, wa,
				fo, wa, wa, wa, wa, wa, wa, pl,
				fo, fo, wa, wa, wa, wa, pl, pl,
			},
		},
		{ // px=3
			Name: "Silverfall Run", MainBiome: BiomeWater,
			Tiles: [64]Biome{
				mt, mt, wa, wa, mt, mt, mt, mt,
				mt, wa, wa, wa, wa, wa, mt, pl,
				wa, wa, wa, ma, wa, wa, pl, pl,
				wa, wa, wa, wa, wa, pl, pl, pl,
				wa, wa, wa, wa, wa, wa, pl, pl,
				wa, wa, wa, wa, ma, wa, wa, pl,
				wa, wa, wa, wa, pl, wa, wa, pl,
				wa, wa, wa, pl, pl, pl, pl, pl,
			},
		},
		{ // px=4
			Name: "Openfields", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, pl, mt, mt,
				mt, pl, mt, pl, pl, pl, pl, mt,
				wa, pl, pl, pl, pl, pl, pl, pl,
				wa, wa, wa, pl, pl, pl, pl, pl,
				wa, wa, wa, wa, pl, pl, pl, pl,
				wa, wa, wa, pl, pl, pl, pl, pl,
				wa, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
			},
		},
		{ // px=5
			Name: "Goldenfields", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, pl, mt, mt,
				mt, pl, pl, mt, pl, pl, pl, mt,
				pl, pl, pl, mt, pl, mt, mt, mt,
				pl, pl, pl, pl, pl, pl, pl, mt,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
			},
		},
		{ // px=6
			Name: "Dragonsback", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				fo, fo, fo, fo, fo, mt, mt, mt,
				pl, pl, fo, mt, mt, mt, mt, mt,
				pl, pl, pl, mt, mt, mt, mt, mt,
				pl, pl, mt, mt, mt, mt, mt, mt,
				pl, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				pl, mt, mt, mt, pl, mt, mt, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
			},
		},
		{ // px=7
			Name: "Raincrag", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, fo, fo, fo,
				mt, mt, mt, mt, mt, mt, fo, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, pl, mt, mt,
				pl, mt, mt, mt, mt, pl, mt, mt,
				pl, pl, mt, mt, pl, pl, pl, mt,
			},
		},
	},
	// ── py=2 ─────────────────────────────────────────────────────
	{
		{ // px=0
			Name: "Mosswood", MainBiome: BiomeForest,
			Tiles: [64]Biome{
				mt, mt, mt, fo, fo, fo, fo, fo,
				fo, mt, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
			},
		},
		{ // px=1
			Name: "Deepgreen Vale", MainBiome: BiomeForest,
			Tiles: [64]Biome{
				mt, mt, fo, fo, wa, wa, wa, wa,
				mt, fo, fo, wa, wa, wa, wa, wa,
				fo, fo, fo, fo, wa, wa, wa, wa,
				fo, fo, fo, fo, fo, wa, wa, wa,
				fo, fo, fo, fo, fo, fo, fo, wa,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, wa,
				fo, fo, fo, fo, fo, fo, wa, wa,
			},
		},
		{ // px=2
			Name: "Silvercurrent", MainBiome: BiomeWater,
			Tiles: [64]Biome{
				fo, fo, wa, wa, wa, wa, wa, pl,
				fo, wa, wa, wa, wa, wa, wa, pl,
				wa, wa, wa, ma, wa, wa, pl, pl,
				fo, wa, wa, wa, wa, wa, wa, pl,
				fo, fo, fo, wa, wa, wa, wa, pl,
				fo, wa, wa, wa, ma, wa, wa, wa,
				fo, wa, wa, wa, wa, wa, wa, pl,
				fo, fo, wa, wa, wa, wa, pl, pl,
			},
		},
		{ // px=3
			Name: "Sunmeadow", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				wa, wa, wa, wa, wa, wa, wa, wa,
				wa, pl, wa, wa, wa, wa, wa, pl,
				wa, wa, wa, pl, wa, pl, pl, pl,
				wa, wa, wa, wa, pl, pl, pl, pl,
				wa, wa, wa, pl, pl, pl, pl, pl,
				wa, pl, pl, pl, pl, pl, pl, pl,
				wa, pl, pl, pl, pl, pl, pl, pl,
				wa, wa, pl, pl, pl, pl, pl, pl,
			},
		},
		{ // px=4
			Name: "Haycross", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				wa, wa, pl, pl, pl, pl, pl, pl,
				wa, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
			},
		},
		{ // px=5
			Name: "Golderin Plain", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				pl, pl, pl, pl, pl, pl, mt, mt,
				pl, pl, pl, pl, pl, pl, pl, mt,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
			},
		},
		{ // px=6
			Name: "Eastfields", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				pl, pl, mt, mt, mt, mt, mt, mt,
				pl, pl, pl, pl, mt, pl, pl, mt,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
			},
		},
		{ // px=7
			Name: "Reedmere", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, mt, pl, pl,
				mt, pl, pl, mt, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
			},
		},
	},
	// ── py=3 ─────────────────────────────────────────────────────
	{
		{ // px=0
			Name: "Wolfwood", MainBiome: BiomeForest,
			Tiles: [64]Biome{
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
			},
		},
		{ // px=1
			Name: "Greenholme", MainBiome: BiomeForest,
			Tiles: [64]Biome{
				fo, fo, fo, fo, fo, fo, wa, wa,
				fo, fo, fo, fo, fo, fo, fo, wa,
				fo, fo, fo, fo, fo, fo, fo, wa,
				fo, fo, fo, fo, fo, fo, wa, wa,
				fo, fo, fo, fo, fo, wa, wa, wa,
				fo, fo, fo, fo, fo, fo, wa, wa,
				fo, fo, fo, fo, fo, fo, fo, wa,
				fo, fo, fo, fo, fo, fo, wa, wa,
			},
		},
		{ // px=2
			Name: "Glasswater", MainBiome: BiomeWater,
			Tiles: [64]Biome{
				fo, fo, wa, wa, wa, wa, pl, pl,
				fo, wa, wa, wa, wa, wa, wa, pl,
				wa, wa, wa, ma, wa, wa, pl, pl,
				fo, wa, wa, wa, wa, wa, wa, pl,
				fo, fo, fo, wa, wa, wa, wa, pl,
				fo, wa, wa, wa, ma, wa, wa, wa,
				fo, wa, wa, wa, wa, wa, wa, pl,
				fo, fo, wa, wa, wa, wa, pl, pl,
			},
		},
		{ // px=3
			Name: "Briarfields", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				wa, wa, pl, pl, pl, pl, pl, pl,
				wa, wa, wa, pl, pl, pl, pl, pl,
				wa, wa, wa, wa, wa, pl, pl, pl,
				wa, wa, wa, pl, pl, pl, pl, pl,
				wa, wa, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				wa, pl, pl, pl, pl, pl, pl, pl,
				wa, wa, pl, pl, pl, pl, pl, pl,
			},
		},
		{ // px=4
			Name: "Amberlea", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
			},
		},
		{ // px=5
			Name: "Eastreach", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, mt,
				pl, pl, pl, pl, pl, pl, mt, mt,
			},
		},
		{ // px=6
			Name: "Wheatgate", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, mt, pl, pl, pl, pl,
				pl, pl, pl, mt, pl, pl, pl, mt,
				pl, pl, mt, mt, mt, pl, mt, mt,
			},
		},
		{ // px=7
			Name: "Goldwell", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				mt, pl, mt, pl, pl, pl, pl, pl,
				mt, mt, mt, mt, mt, pl, pl, pl,
			},
		},
	},
	// ── py=4 ─────────────────────────────────────────────────────
	{
		{ // px=0
			Name: "Gloomwood", MainBiome: BiomeForest,
			Tiles: [64]Biome{
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
			},
		},
		{ // px=1
			Name: "Duskwood", MainBiome: BiomeForest,
			Tiles: [64]Biome{
				fo, fo, fo, fo, fo, fo, wa, wa,
				fo, fo, fo, fo, fo, fo, fo, wa,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, wa, wa,
				fo, fo, fo, fo, fo, wa, wa, wa,
				fo, fo, fo, wa, wa, wa, wa, wa,
				fo, fo, fo, fo, fo, wa, wa, wa,
				fo, fo, fo, fo, fo, fo, wa, wa,
			},
		},
		{ // px=2
			Name: "Stillwater", MainBiome: BiomeWater,
			Tiles: [64]Biome{
				fo, fo, wa, wa, wa, wa, pl, pl,
				fo, wa, wa, wa, wa, wa, wa, pl,
				fo, wa, wa, ma, wa, wa, wa, wa,
				fo, fo, wa, wa, wa, wa, wa, wa,
				fo, wa, wa, wa, wa, wa, wa, wa,
				fo, wa, wa, wa, ma, wa, wa, pl,
				fo, wa, wa, wa, wa, wa, pl, pl,
				fo, fo, wa, wa, wa, wa, pl, pl,
			},
		},
		{ // px=3
			Name: "Clearmeadow", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				wa, wa, pl, pl, pl, pl, pl, pl,
				wa, pl, pl, pl, pl, pl, pl, pl,
				wa, pl, pl, pl, pl, pl, pl, pl,
				wa, wa, wa, pl, pl, pl, pl, pl,
				wa, wa, wa, wa, pl, pl, pl, pl,
				wa, wa, wa, wa, wa, pl, pl, pl,
				wa, wa, wa, wa, pl, pl, pl, pl,
				wa, wa, wa, pl, pl, pl, pl, pl,
			},
		},
		{ // px=4
			Name: "Lyefields", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
			},
		},
		{ // px=5
			Name: "Goldenvale", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, mt,
				pl, pl, pl, pl, pl, pl, mt, mt,
				pl, pl, pl, pl, pl, mt, mt, mt,
				pl, pl, pl, pl, pl, pl, mt, mt,
				pl, pl, pl, pl, pl, pl, mt, mt,
			},
		},
		{ // px=6
			Name: "Ironpeak", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				fo, pl, mt, mt, mt, pl, pl, pl,
				fo, fo, mt, mt, mt, mt, pl, pl,
				fo, fo, fo, mt, mt, mt, pl, mt,
				fo, fo, pl, mt, mt, mt, mt, mt,
				fo, pl, mt, mt, mt, mt, mt, mt,
				pl, mt, mt, mt, mt, mt, mt, mt,
				pl, mt, mt, mt, mt, mt, mt, mt,
				pl, pl, mt, mt, mt, mt, mt, mt,
			},
		},
		{ // px=7
			Name: "Mount Vallis", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				pl, pl, mt, mt, pl, pl, pl, mt,
				pl, mt, mt, mt, mt, pl, mt, mt,
				fo, mt, mt, mt, mt, pl, mt, mt,
				fo, fo, mt, mt, mt, mt, mt, mt,
				fo, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
			},
		},
	},
	// ── py=5 ─────────────────────────────────────────────────────
	{
		{ // px=0
			Name: "Fernwood", MainBiome: BiomeForest,
			Tiles: [64]Biome{
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, fo, fo, fo, fo,
				fo, fo, fo, fo, mt, fo, fo, fo,
				fo, fo, fo, mt, mt, mt, fo, fo,
			},
		},
		{ // px=1
			Name: "Willowvale", MainBiome: BiomeForest,
			Tiles: [64]Biome{
				fo, fo, fo, fo, fo, fo, wa, wa,
				fo, fo, fo, fo, fo, fo, fo, wa,
				fo, fo, fo, fo, fo, fo, wa, wa,
				fo, fo, fo, fo, fo, fo, wa, wa,
				fo, fo, fo, fo, fo, wa, wa, wa,
				fo, fo, fo, fo, fo, fo, wa, wa,
				mt, fo, fo, fo, fo, fo, wa, wa,
				mt, mt, fo, fo, fo, fo, wa, wa,
			},
		},
		{ // px=2
			Name: "Reedwater", MainBiome: BiomeWater,
			Tiles: [64]Biome{
				fo, fo, wa, wa, wa, wa, pl, pl,
				fo, wa, wa, wa, wa, wa, pl, pl,
				wa, wa, wa, ma, wa, wa, wa, pl,
				wa, wa, wa, wa, wa, wa, wa, pl,
				fo, wa, wa, wa, wa, wa, wa, wa,
				fo, fo, fo, wa, ma, wa, wa, wa,
				fo, wa, wa, wa, wa, wa, wa, wa,
				fo, fo, wa, wa, wa, wa, wa, wa,
			},
		},
		{ // px=3
			Name: "Hollowmeadow", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				wa, wa, pl, pl, pl, pl, pl, pl,
				wa, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, wa, pl, pl, pl, pl,
				wa, pl, pl, wa, pl, pl, pl, pl,
				wa, wa, wa, wa, wa, pl, pl, pl,
				wa, wa, wa, wa, wa, wa, pl, pl,
				wa, wa, wa, wa, wa, wa, pl, pl,
			},
		},
		{ // px=4
			Name: "Farfields", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				wa, pl, pl, pl, pl, pl, pl, pl,
				wa, wa, pl, pl, pl, pl, pl, pl,
			},
		},
		{ // px=5
			Name: "Pleasant Reach", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				pl, pl, pl, pl, pl, pl, mt, mt,
				pl, pl, pl, pl, pl, pl, pl, mt,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, mt,
				pl, pl, pl, pl, pl, pl, mt, mt,
				pl, pl, pl, pl, pl, pl, pl, mt,
			},
		},
		{ // px=6
			Name: "Greyspire", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				fo, fo, mt, mt, mt, mt, mt, mt,
				fo, fo, fo, mt, mt, mt, mt, mt,
				fo, fo, mt, mt, mt, mt, mt, mt,
				fo, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				pl, mt, mt, mt, mt, pl, mt, mt,
				pl, pl, mt, mt, pl, pl, pl, mt,
			},
		},
		{ // px=7
			Name: "Cinderpeak", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				fo, mt, mt, mt, mt, mt, mt, mt,
				fo, fo, fo, mt, mt, mt, mt, mt,
				fo, mt, mt, mt, mt, mt, mt, mt,
				pl, mt, mt, mt, mt, mt, mt, mt,
				pl, pl, mt, mt, mt, mt, mt, mt,
			},
		},
	},
	// ── py=6 ─────────────────────────────────────────────────────
	{
		{ // px=0
			Name: "Southwall Ridge", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				mt, mt, fo, fo, fo, fo, fo, fo,
				mt, mt, mt, fo, fo, fo, fo, fo,
				mt, mt, mt, mt, fo, fo, fo, fo,
				mt, mt, mt, mt, mt, mt, fo, fo,
				mt, mt, mt, mt, mt, mt, mt, fo,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
			},
		},
		{ // px=1
			Name: "Fellwood Edge", MainBiome: BiomeForest,
			Tiles: [64]Biome{
				fo, fo, fo, fo, fo, fo, wa, wa,
				mt, fo, fo, fo, fo, wa, wa, wa,
				mt, mt, fo, wa, wa, wa, wa, wa,
				mt, fo, fo, fo, fo, wa, wa, wa,
				fo, fo, fo, fo, fo, fo, wa, wa,
				fo, fo, fo, fo, fo, fo, fo, fo,
				mt, fo, fo, fo, fo, mt, fo, mt,
				mt, mt, fo, fo, mt, mt, mt, mt,
			},
		},
		{ // px=2
			Name: "Southcurrent", MainBiome: BiomeWater,
			Tiles: [64]Biome{
				fo, fo, wa, wa, wa, wa, pl, pl,
				fo, wa, wa, wa, wa, wa, wa, pl,
				wa, wa, wa, ma, wa, wa, wa, wa,
				wa, wa, wa, wa, wa, wa, wa, wa,
				fo, wa, wa, wa, wa, wa, wa, wa,
				fo, wa, wa, wa, ma, wa, wa, wa,
				fo, fo, wa, wa, wa, wa, mt, mt,
				fo, mt, wa, wa, mt, mt, mt, mt,
			},
		},
		{ // px=3
			Name: "Tidemarsh", MainBiome: BiomeWater,
			Tiles: [64]Biome{
				wa, wa, wa, wa, wa, pl, pl, pl,
				wa, wa, wa, wa, wa, wa, pl, pl,
				wa, wa, wa, ma, wa, wa, pl, pl,
				wa, wa, wa, wa, wa, wa, wa, pl,
				wa, wa, wa, wa, wa, wa, wa, pl,
				wa, mt, wa, wa, ma, wa, wa, wa,
				mt, mt, mt, wa, wa, wa, wa, pl,
				mt, mt, mt, mt, wa, wa, pl, pl,
			},
		},
		{ // px=4
			Name: "Meadowlands", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				wa, wa, pl, pl, pl, pl, pl, pl,
				wa, wa, pl, pl, pl, pl, pl, pl,
				wa, wa, wa, pl, pl, pl, pl, pl,
				wa, wa, pl, pl, pl, pl, pl, pl,
				wa, wa, pl, pl, pl, pl, pl, pl,
				wa, pl, pl, pl, pl, pl, pl, pl,
				mt, pl, pl, pl, pl, pl, pl, pl,
				mt, mt, pl, pl, pl, pl, pl, pl,
			},
		},
		{ // px=5
			Name: "Greenfields", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				pl, pl, pl, pl, pl, pl, mt, mt,
				pl, pl, pl, pl, pl, pl, pl, mt,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, mt,
				pl, pl, pl, pl, pl, pl, mt, mt,
			},
		},
		{ // px=6
			Name: "Harvestfields", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				pl, pl, pl, pl, mt, mt, mt, mt,
				pl, pl, pl, pl, pl, mt, mt, mt,
				pl, pl, pl, pl, pl, pl, mt, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, mt, pl, pl, pl, mt,
				pl, pl, mt, mt, mt, pl, mt, mt,
				pl, mt, mt, mt, mt, mt, mt, mt,
			},
		},
		{ // px=7
			Name: "Aftward Crags", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, mt, mt, mt,
				pl, mt, mt, mt, mt, mt, mt, mt,
				pl, pl, mt, mt, mt, mt, mt, mt,
				pl, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
			},
		},
	},
	// ── py=7 ─────────────────────────────────────────────────────
	{
		{ // px=0
			Name: "Southcape Peaks", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				mt, mt, mt, mt, mt, mt, fo, fo,
				mt, mt, mt, mt, mt, mt, mt, fo,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
			},
		},
		{ // px=1
			Name: "Southplain Rise", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				mt, fo, fo, fo, mt, mt, wa, wa,
				mt, mt, fo, mt, mt, mt, mt, wa,
				mt, mt, fo, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
			},
		},
		{ // px=2
			Name: "Blackmoor Ridge", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				fo, fo, wa, wa, wa, wa, wa, wa,
				fo, mt, mt, wa, wa, wa, wa, wa,
				mt, mt, mt, mt, mt, wa, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
			},
		},
		{ // px=3
			Name: "Southgate Pass", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				wa, wa, wa, wa, mt, mt, pl, pl,
				wa, wa, wa, wa, mt, mt, mt, pl,
				wa, wa, wa, mt, mt, mt, mt, mt,
				mt, wa, mt, mt, mt, mt, mt, mt,
				mt, wa, mt, mt, mt, mt, mt, pl,
				mt, mt, mt, mt, mt, pl, pl, pl,
				mt, mt, mt, mt, mt, mt, mt, pl,
				mt, mt, mt, mt, mt, mt, mt, mt,
			},
		},
		{ // px=4
			Name: "Farplain", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				wa, wa, pl, pl, pl, pl, pl, pl,
				mt, pl, pl, pl, pl, pl, pl, pl,
				mt, pl, pl, pl, pl, pl, pl, pl,
				mt, mt, pl, pl, pl, pl, pl, pl,
				mt, pl, pl, pl, pl, pl, pl, pl,
				mt, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
			},
		},
		{ // px=5
			Name: "Lastfields", MainBiome: BiomePlains,
			Tiles: [64]Biome{
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, pl,
				pl, pl, pl, pl, pl, pl, pl, mt,
				pl, pl, pl, pl, pl, mt, mt, mt,
				pl, pl, pl, pl, pl, pl, pl, mt,
				pl, pl, pl, pl, pl, pl, pl, pl,
			},
		},
		{ // px=6
			Name: "Cinderhold", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				pl, pl, mt, mt, mt, pl, pl, pl,
				pl, pl, pl, mt, mt, mt, pl, mt,
				pl, pl, mt, mt, mt, mt, mt, mt,
				pl, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
			},
		},
		{ // px=7
			Name: "Drakeward Crag", MainBiome: BiomeMountain,
			Tiles: [64]Biome{
				pl, pl, mt, mt, mt, mt, mt, mt,
				pl, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
				mt, mt, mt, mt, mt, mt, mt, mt,
			},
		},
	},
}

// BiomeAt returns the tile biome at world coordinates (x, y). Coordinates use
// the top-left origin (ADR 0004). x,y must be in [0, WorldSize).
func BiomeAt(x, y int) Biome {
	return RegionDefs[y/PageSize][x/PageSize].
		Tiles[(y%PageSize)*PageSize+x%PageSize]
}

// RegionAt returns the RegionDef for the page containing world tile (x, y).
func RegionAt(x, y int) RegionDef {
	return RegionDefs[y/PageSize][x/PageSize]
}
