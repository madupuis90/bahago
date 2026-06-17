package game

import "bahago/internal/database/db"

// Building type identifiers. These are stored as-is in the database.
const (
	BuildingMill       = "mill"
	BuildingFactory    = "factory"
	BuildingQuarry     = "quarry"
	BuildingBlacksmith = "blacksmith"
	BuildingFarm       = "farm"
	BuildingGrainerie  = "grainerie"
	BuildingArmory     = "armory"
)

// Prerequisite requires that a given building type has at least Min instances built.
type Prerequisite struct {
	Type string
	Min  int
}

// BuildingDef describes a building type. All values are static config — nothing is stored in the DB.
type BuildingDef struct {
	// ID is the canonical string key (matches Building* constants).
	ID string

	// Name is the human-readable display name.
	Name string

	// Max is the maximum number of this building a kingdom can construct.
	Max int

	// Lane hints at the column position in the skill-tree layout (0 = auto).
	// For root buildings, Lane specifies the column explicitly.
	Lane int

	// Resource is the gem glyph ID used for the node icon ("tree", "mountain", "wheat", etc.).
	// Empty for buildings with no resource affinity (e.g. Armory).
	Resource string

	// Icon is the icon ID for buildings without a resource gem (e.g. "swords" for Armory).
	// Empty when Resource is set.
	Icon string

	// Flavour is the one-line flavour text shown in the detail panel.
	Flavour string

	// Cost is the resource cost per construction (same for every instance).
	Cost ResourceValues

	// Ticks is the number of game ticks required to complete one construction.
	Ticks int

	// BonusPctPer holds the % production bonus per instance for each resource.
	// Total bonus = count * BonusPctPer.{Resource}.
	BonusPctPer ProductionBonus

	// Prereqs lists buildings that must have at least Min instances before this
	// building becomes available. All must be satisfied (AND).
	Prereqs []Prerequisite
}

// BuildingDefs is the authoritative static configuration for all building types.
var BuildingDefs = map[string]BuildingDef{
	BuildingMill: {
		ID:          BuildingMill,
		Name:        "Mill",
		Max:         5,
		Lane:        0,
		Resource:    "tree",
		Flavour:     "Saws sing dawn to dusk — each mill swells the timber that comes off your woodcutters.",
		Cost:        ResourceValues{Wood: 30},
		Ticks:       2,
		BonusPctPer: ProductionBonus{Wood: 10},
	},
	BuildingQuarry: {
		ID:          BuildingQuarry,
		Name:        "Quarry",
		Max:         5,
		Lane:        1,
		Resource:    "mountain",
		Flavour:     "Hewn steps cut deeper into the rock; every quarry lifts the stone your miners draw.",
		Cost:        ResourceValues{Wood: 20, Stone: 10},
		Ticks:       2,
		BonusPctPer: ProductionBonus{Stone: 10},
	},
	BuildingFarm: {
		ID:          BuildingFarm,
		Name:        "Farm",
		Max:         5,
		Lane:        2,
		Resource:    "wheat",
		Flavour:     "Tilled rows and turned soil — a farm makes every farmer's harvest go further.",
		Cost:        ResourceValues{Wood: 20, Stone: 10},
		Ticks:       2,
		BonusPctPer: ProductionBonus{Food: 10},
	},
	BuildingFactory: {
		ID:          BuildingFactory,
		Name:        "Factory",
		Max:         2,
		Resource:    "tree",
		Flavour:     "Mass-cut lumber on a grand scale — what the mills begin, the factory multiplies.",
		Cost:        ResourceValues{Wood: 100, Stone: 20},
		Ticks:       5,
		BonusPctPer: ProductionBonus{Wood: 25},
		Prereqs:     []Prerequisite{{Type: BuildingMill, Min: 1}},
	},
	BuildingBlacksmith: {
		ID:          BuildingBlacksmith,
		Name:        "Blacksmith",
		Max:         2,
		Resource:    "mountain",
		Flavour:     "Hammer and forge dress raw stone into worked masonry — a far richer yield.",
		Cost:        ResourceValues{Wood: 40, Stone: 60},
		Ticks:       5,
		BonusPctPer: ProductionBonus{Stone: 25},
		Prereqs:     []Prerequisite{{Type: BuildingQuarry, Min: 1}},
	},
	BuildingGrainerie: {
		ID:          BuildingGrainerie,
		Name:        "Grainerie",
		Max:         2,
		Resource:    "wheat",
		Flavour:     "Granaries hold the surplus and feed it back — the harvest compounds, season on season.",
		Cost:        ResourceValues{Wood: 60, Stone: 40},
		Ticks:       5,
		BonusPctPer: ProductionBonus{Food: 25},
		Prereqs:     []Prerequisite{{Type: BuildingFarm, Min: 1}},
	},
	BuildingArmory: {
		ID:      BuildingArmory,
		Name:    "Armory",
		Max:     1,
		Icon:    "swords",
		Flavour: "Where timber and stone become war. The crown of your works — it asks both forge and factory first.",
		Cost:    ResourceValues{Wood: 80, Stone: 80},
		Ticks:   10,
		Prereqs: []Prerequisite{
			{Type: BuildingFactory, Min: 1},
			{Type: BuildingBlacksmith, Min: 1},
		},
	},
}

// BuildingDefList is the ordered list of building definitions for tree rendering.
// Order: dependency-safe (root buildings first, dependents after).
var BuildingDefList = []BuildingDef{
	BuildingDefs[BuildingMill],
	BuildingDefs[BuildingQuarry],
	BuildingDefs[BuildingFarm],
	BuildingDefs[BuildingFactory],
	BuildingDefs[BuildingBlacksmith],
	BuildingDefs[BuildingGrainerie],
	BuildingDefs[BuildingArmory],
}

// BuildingName returns the display name for the given building type ID.
func BuildingName(id string) string {
	return BuildingDefs[id].Name
}

// BuildingCountMap converts a slice of kingdom buildings into a map from
// building type to count for O(1) lookups during prerequisite checks.
func BuildingCountMap(buildings []db.KingdomBuilding) map[string]int {
	m := make(map[string]int, len(buildings))
	for _, b := range buildings {
		m[b.BuildingType] = b.Count
	}
	return m
}

// CanBuild reports whether a kingdom may begin construction of the given building type.
// It returns false if the kingdom has already reached Max or if any prerequisite
// is not yet satisfied.
func CanBuild(btype string, counts map[string]int) bool {
	def, ok := BuildingDefs[btype]
	if !ok {
		return false
	}
	if counts[btype] >= def.Max {
		return false
	}
	for _, p := range def.Prereqs {
		if counts[p.Type] < p.Min {
			return false
		}
	}
	return true
}

// BuildingBonusPct returns the total production bonus percentage per resource
// contributed by all buildings in the provided count map.
func BuildingBonusPct(counts map[string]int) ProductionBonus {
	var totals ProductionBonus
	for btype, count := range counts {
		def, ok := BuildingDefs[btype]
		if !ok || count == 0 {
			continue
		}
		totals.Wood += count * def.BonusPctPer.Wood
		totals.Stone += count * def.BonusPctPer.Stone
		totals.Food += count * def.BonusPctPer.Food
		totals.Mana += count * def.BonusPctPer.Mana
		totals.Devotion += count * def.BonusPctPer.Devotion
		totals.Knowledge += count * def.BonusPctPer.Knowledge
	}
	return totals
}
