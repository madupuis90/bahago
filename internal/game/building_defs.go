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

// Prerequisite requires that a given building type has at least MinCount instances built.
type Prerequisite struct {
	Type     string
	MinCount int
}

// BuildingDef describes a building type. All values are static config — nothing is stored in the DB.
type BuildingDef struct {
	// Name is the human-readable display name.
	Name string

	// MaxCount is the maximum number of this building a kingdom can construct.
	MaxCount int

	// Cost is the resource cost per construction (same for every instance).
	Cost ResourceValues

	// Ticks is the number of game ticks required to complete one construction.
	Ticks int

	// BonusPctPer holds the % production bonus per instance for each resource.
	// Total bonus = count * BonusPctPer.{Resource}.
	BonusPctPer ProductionBonus

	// Prerequisites lists buildings that must have at least MinCount instances
	// before this building becomes available. All must be satisfied (AND).
	Prerequisites []Prerequisite
}

// BuildingDefs is the authoritative static configuration for all building types.
var BuildingDefs = map[string]BuildingDef{
	BuildingMill: {
		Name:        "Mill",
		MaxCount:    5,
		Cost:        ResourceValues{Wood: 30},
		Ticks:       2,
		BonusPctPer: ProductionBonus{Wood: 10},
	},
	BuildingFactory: {
		Name:          "Factory",
		MaxCount:      2,
		Cost:          ResourceValues{Wood: 100, Stone: 20},
		Ticks:         5,
		BonusPctPer:   ProductionBonus{Wood: 25},
		Prerequisites: []Prerequisite{{Type: BuildingMill, MinCount: 1}},
	},
	BuildingQuarry: {
		Name:        "Quarry",
		MaxCount:    5,
		Cost:        ResourceValues{Wood: 20, Stone: 10},
		Ticks:       2,
		BonusPctPer: ProductionBonus{Stone: 10},
	},
	BuildingBlacksmith: {
		Name:          "Blacksmith",
		MaxCount:      2,
		Cost:          ResourceValues{Wood: 40, Stone: 60},
		Ticks:         5,
		BonusPctPer:   ProductionBonus{Stone: 25},
		Prerequisites: []Prerequisite{{Type: BuildingQuarry, MinCount: 1}},
	},
	BuildingFarm: {
		Name:        "Farm",
		MaxCount:    5,
		Cost:        ResourceValues{Wood: 20, Stone: 10},
		Ticks:       2,
		BonusPctPer: ProductionBonus{Food: 10},
	},
	BuildingGrainerie: {
		Name:          "Grainerie",
		MaxCount:      2,
		Cost:          ResourceValues{Wood: 60, Stone: 40},
		Ticks:         5,
		BonusPctPer:   ProductionBonus{Food: 25},
		Prerequisites: []Prerequisite{{Type: BuildingFarm, MinCount: 1}},
	},
	BuildingArmory: {
		Name:     "Armory",
		MaxCount: 1,
		Cost:     ResourceValues{Wood: 80, Stone: 80},
		Ticks:    10,
		Prerequisites: []Prerequisite{
			{Type: BuildingFactory, MinCount: 1},
			{Type: BuildingBlacksmith, MinCount: 1},
		},
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
		if counts[p.Type] < p.MinCount {
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
