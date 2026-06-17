package game

import "fmt"

const (
	ColGap = 162
	RowGap = 144
	NodeW  = 136
	NodeH  = 96
	PadX   = 16
	PadY   = 10
)

// Point is a 2D pixel coordinate used for SVG path generation.
type Point struct{ X, Y int }

// PlacedNode is a BuildingDef enriched with computed pixel geometry.
type PlacedNode struct {
	BuildingDef
	Row, Col   float64
	Left, Top  int
	CX, Bottom int
}

// PlacedTree is the output of PlaceNodes: positioned nodes and canvas dimensions.
type PlacedTree struct {
	Nodes    []PlacedNode
	NodeByID map[string]PlacedNode
	Width    int
	Height   int
}

func buildingByID(defs []BuildingDef, id string) BuildingDef {
	for _, d := range defs {
		if d.ID == id {
			return d
		}
	}
	return BuildingDef{}
}

// BuildingByID returns the BuildingDef with the given ID from defs, or a zero value if not found.
func BuildingByID(defs []BuildingDef, id string) BuildingDef {
	return buildingByID(defs, id)
}

// PlaceNodes computes pixel positions for each building in defs based on
// prerequisite depth (row) and lane hint (col), then returns the full placed tree.
func PlaceNodes(defs []BuildingDef) PlacedTree {
	rowOf := map[string]int{}
	var row func(id string) int
	row = func(id string) int {
		if v, ok := rowOf[id]; ok {
			return v
		}
		b := buildingByID(defs, id)
		r := 0
		for _, p := range b.Prereqs {
			if pr := row(p.Type) + 1; pr > r {
				r = pr
			}
		}
		rowOf[id] = r
		return r
	}

	colOf := map[string]float64{}
	var col func(id string) float64
	col = func(id string) float64 {
		if v, ok := colOf[id]; ok {
			return v
		}
		b := buildingByID(defs, id)
		var c float64
		if b.Lane != 0 || len(b.Prereqs) == 0 {
			c = float64(b.Lane)
		} else {
			sum := 0.0
			for _, p := range b.Prereqs {
				sum += col(p.Type)
			}
			c = sum / float64(len(b.Prereqs))
		}
		colOf[id] = c
		return c
	}

	for _, d := range defs {
		row(d.ID)
		col(d.ID)
	}

	maxCol, maxRow := 0.0, 0
	for _, d := range defs {
		if c := col(d.ID); c > maxCol {
			maxCol = c
		}
		if r := row(d.ID); r > maxRow {
			maxRow = r
		}
	}

	nodes := make([]PlacedNode, len(defs))
	byID := map[string]PlacedNode{}
	for i, d := range defs {
		cx := PadX + NodeW/2 + int(col(d.ID)*float64(ColGap))
		top := PadY + row(d.ID)*RowGap
		n := PlacedNode{
			BuildingDef: d,
			Row:         float64(row(d.ID)),
			Col:         col(d.ID),
			Left:        cx - NodeW/2,
			Top:         top,
			CX:          cx,
			Bottom:      top + NodeH,
		}
		nodes[i] = n
		byID[d.ID] = n
	}

	w := PadX*2 + NodeW + int(maxCol)*ColGap
	h := PadY*2 + NodeH + maxRow*RowGap
	return PlacedTree{Nodes: nodes, NodeByID: byID, Width: w, Height: h}
}

// ElbowPath returns an SVG path string for an orthogonal elbow connector
// from point a (top) to point b (bottom) with 14px rounded corners.
func ElbowPath(a, b Point) string {
	mid := (a.Y + b.Y) / 2
	dx := b.X - a.X
	if dx < 0 {
		dx = -dx
	}
	r := 14
	if dx/2 < r {
		r = dx / 2
	}
	if dx < 1 {
		return fmt.Sprintf("M%d,%d L%d,%d", a.X, a.Y, b.X, b.Y)
	}
	dir := 1
	if b.X < a.X {
		dir = -1
	}
	return fmt.Sprintf(
		"M%d,%d L%d,%d Q%d,%d %d,%d L%d,%d Q%d,%d %d,%d L%d,%d",
		a.X, a.Y,
		a.X, mid-r,
		a.X, mid, a.X+dir*r, mid,
		b.X-dir*r, mid,
		b.X, mid, b.X, mid+r,
		b.X, b.Y,
	)
}

// PrereqMet reports whether all prerequisites for b are satisfied by counts.
func PrereqMet(b BuildingDef, counts map[string]int) bool {
	for _, p := range b.Prereqs {
		if counts[p.Type] < p.Min {
			return false
		}
	}
	return true
}

// CanAfford reports whether resources cover the cost of one construction of b.
// ResourceValues is a struct — fields are checked explicitly.
func CanAfford(b BuildingDef, resources map[string]int) bool {
	return resources["wood"] >= b.Cost.Wood &&
		resources["stone"] >= b.Cost.Stone &&
		resources["food"] >= b.Cost.Food &&
		resources["mana"] >= b.Cost.Mana &&
		resources["devotion"] >= b.Cost.Devotion &&
		resources["knowledge"] >= b.Cost.Knowledge
}
