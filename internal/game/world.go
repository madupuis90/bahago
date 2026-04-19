package game

// World map constants. The world is a WorldSize×WorldSize grid of tiles.
// Each WorldMap page shows PageSize×PageSize tiles.
// WorldSize must be a multiple of PageSize — there are (WorldSize/PageSize) pages per axis.
const (
	WorldSize = 64                   // total tiles per axis (8×8 pages)
	PageSize  = 8                    // tiles per page axis
	PageCount = WorldSize / PageSize // 8 pages per axis
)

// Coord is a tile coordinate in the world grid.
type Coord struct {
	X, Y int
}
