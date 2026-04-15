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

// ResourceCost defines the resource price for a single construction of a building.
type ResourceCost struct {
	Wood      int
	Stone     int
	Food      int
	Mana      int
	Devotion  int
	Knowledge int
}

// Prerequisite requires that a given building type has at least MinCount instances built.
type Prerequisite struct {
	BuildingType string
	MinCount     int
}

// BuildingDef describes a building type. All values are static config — nothing is stored in the DB.
type BuildingDef struct {
	// Name is the human-readable display name.
	Name string

	// MaxCount is the maximum number of this building a kingdom can construct.
	MaxCount int

	// Cost is the resource cost per construction (same for every instance).
	Cost ResourceCost

	// Ticks is the number of game ticks required to complete one construction.
	Ticks int

	// BonusPctPer maps resource keys (e.g. "wood") to the % production bonus
	// added per instance. Total bonus = count * BonusPctPer[resource].
	BonusPctPer map[string]int

	// Prerequisites lists buildings that must have at least MinCount instances
	// before this building becomes available. All must be satisfied (AND).
	Prerequisites []Prerequisite

	// UnlocksUnits lists unit types that become available once count >= 1.
	UnlocksUnits []string
}

// BuildingDefs is the authoritative static configuration for all building types.
var BuildingDefs = map[string]BuildingDef{
	BuildingMill: {
		Name:        "Mill",
		MaxCount:    5,
		Cost:        ResourceCost{Wood: 30},
		Ticks:       2,
		BonusPctPer: map[string]int{"wood": 10},
	},
	BuildingFactory: {
		Name:          "Factory",
		MaxCount:      2,
		Cost:          ResourceCost{Wood: 100, Stone: 20},
		Ticks:         5,
		BonusPctPer:   map[string]int{"wood": 25},
		Prerequisites: []Prerequisite{{BuildingType: BuildingMill, MinCount: 1}},
	},
	BuildingQuarry: {
		Name:        "Quarry",
		MaxCount:    5,
		Cost:        ResourceCost{Wood: 20, Stone: 10},
		Ticks:       2,
		BonusPctPer: map[string]int{"stone": 10},
	},
	BuildingBlacksmith: {
		Name:          "Blacksmith",
		MaxCount:      2,
		Cost:          ResourceCost{Wood: 40, Stone: 60},
		Ticks:         5,
		BonusPctPer:   map[string]int{"stone": 25},
		Prerequisites: []Prerequisite{{BuildingType: BuildingQuarry, MinCount: 1}},
	},
	BuildingFarm: {
		Name:        "Farm",
		MaxCount:    5,
		Cost:        ResourceCost{Wood: 20, Stone: 10},
		Ticks:       2,
		BonusPctPer: map[string]int{"food": 10},
	},
	BuildingGrainerie: {
		Name:          "Grainerie",
		MaxCount:      2,
		Cost:          ResourceCost{Wood: 60, Stone: 40},
		Ticks:         5,
		BonusPctPer:   map[string]int{"food": 25},
		Prerequisites: []Prerequisite{{BuildingType: BuildingFarm, MinCount: 1}},
	},
	BuildingArmory: {
		Name:     "Armory",
		MaxCount: 1,
		Cost:     ResourceCost{Wood: 80, Stone: 80},
		Ticks:    10,
		Prerequisites: []Prerequisite{
			{BuildingType: BuildingFactory, MinCount: 1},
			{BuildingType: BuildingBlacksmith, MinCount: 1},
		},
		UnlocksUnits: []string{"footman"},
	},
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
// It returns false if the kingdom has already reached MaxCount or if any prerequisite
// is not yet satisfied.
func CanBuild(btype string, counts map[string]int) bool {
	def, ok := BuildingDefs[btype]
	if !ok {
		return false
	}
	if counts[btype] >= def.MaxCount {
		return false
	}
	for _, p := range def.Prerequisites {
		if counts[p.BuildingType] < p.MinCount {
			return false
		}
	}
	return true
}

// BuildingBonusPct returns the total production bonus percentage per resource
// contributed by all buildings in the provided count map.
// The returned map keys are resource names (e.g. "wood", "stone").
func BuildingBonusPct(counts map[string]int) map[string]int {
	totals := make(map[string]int)
	for btype, count := range counts {
		def, ok := BuildingDefs[btype]
		if !ok || count == 0 {
			continue
		}
		for resource, pctPer := range def.BonusPctPer {
			totals[resource] += count * pctPer
		}
	}
	return totals
}
