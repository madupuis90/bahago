package game

import "bahago/internal/database/db"

// Attribute is a conditional modifier applied to a unit during combat or upkeep calculation.
type Attribute string

// Unit attributes. Each constant maps to a display label and combat rule.
const (
	// AttributeWorshipper — power is boosted by devotion.
	AttributeWorshipper Attribute = "Worshipper"

	// AttributeSummon — unit does not consume food upkeep.
	AttributeSummon Attribute = "Summon"

	// AttributePacifism — unit cannot attack.
	AttributePacifism Attribute = "Pacifism"

	// AttributeRaiders — unit cannot block incoming attacks.
	AttributeRaiders Attribute = "Raiders"

	// AttributeFlying — bonus power against melee units.
	AttributeFlying Attribute = "Flying"

	// AttributeArcher — 20% bonus damage against flying units.
	AttributeArcher Attribute = "Archer"

	// AttributeMelee — no special bonus; standard ground combatant.
	AttributeMelee Attribute = "Melee"

	// AttributeSiegeEngine — survives until end of the combat round before dying.
	AttributeSiegeEngine Attribute = "Siege Engine"

	// AttributeUndead — takes 30% less damage from non-worshipper units.
	AttributeUndead Attribute = "Undead"

	// AttributeDeathtouch — deals 50% more damage to non-summon units.
	AttributeDeathtouch Attribute = "Deathtouch"

	// AttributeEnrage — gains 30% more power when outnumbered.
	AttributeEnrage Attribute = "Enrage"

	// AttributeShields — takes 40% less damage from archer units.
	AttributeShields Attribute = "Shields"

	// AttributeGluttony — consumes 30% more food upkeep.
	AttributeGluttony Attribute = "Gluttony"
)

// Unit type identifiers. These are stored as-is in the database.
const (
	// Regular units
	UnitRecruit  = "recruit"
	UnitArcher   = "archer"
	UnitKnight   = "knight"
	UnitRaider   = "raider"
	UnitCatapult = "catapult"

	// Summon units
	UnitShade       = "shade"
	UnitDreadKnight = "dread_knight"
)

// UnitDef describes a unit type. All values are static config — nothing is stored in the DB.
type UnitDef struct {
	// Name is the human-readable display name.
	Name string

	// Power is the base combat value of one unit.
	Power int

	// Cost is the resource cost to train one unit.
	// Regular units use Wood/Stone; summons use Mana.
	Cost ResourceCost

	// FoodUpkeep is food consumed per unit per tick.
	// Zero for units with the Summon attribute.
	FoodUpkeep int

	// ManaUpkeep is mana consumed per unit per tick.
	// Only non-zero for summon units.
	ManaUpkeep int

	// Ticks is the number of game ticks required to complete one training batch,
	// regardless of how many units are in the batch.
	Ticks int

	// IsSummon marks units that belong to the summon table.
	// Summons cost mana, have mana upkeep, and require the summon feature to be unlocked.
	IsSummon bool

	// Prerequisites lists buildings that must have at least MinCount instances
	// before this unit becomes available to train. All must be satisfied (AND).
	Prerequisites []Prerequisite

	// Attributes lists up to 3 conditional modifiers for this unit.
	Attributes []Attribute
}

// UnitDefs is the authoritative static configuration for all unit types.
var UnitDefs = map[string]UnitDef{
	UnitRecruit: {
		Name:       "Recruit",
		Power:      1,
		Ticks:      12, // 3h
		Cost:       ResourceCost{Wood: 10},
		FoodUpkeep: 1,
		Attributes: []Attribute{AttributeMelee},
	},
	UnitArcher: {
		Name:       "Archer",
		Power:      1,
		Ticks:      16, // 4h
		Cost:       ResourceCost{Wood: 15},
		FoodUpkeep: 1,
		Attributes: []Attribute{AttributeArcher},
	},
	UnitRaider: {
		Name:          "Raider",
		Power:         2,
		Ticks:         16, // 4h
		Cost:          ResourceCost{Wood: 15, Stone: 5},
		FoodUpkeep:    1,
		Prerequisites: []Prerequisite{{Type: BuildingMill, MinCount: 1}},
		Attributes:    []Attribute{AttributeRaiders, AttributeMelee},
	},
	UnitKnight: {
		Name:          "Knight",
		Power:         3,
		Ticks:         24, // 6h
		Cost:          ResourceCost{Wood: 20, Stone: 20},
		FoodUpkeep:    2,
		Prerequisites: []Prerequisite{{Type: BuildingArmory, MinCount: 1}},
		Attributes:    []Attribute{AttributeMelee, AttributeShields},
	},
	UnitCatapult: {
		Name:          "Catapult",
		Power:         5,
		Ticks:         32, // 8h
		Cost:          ResourceCost{Stone: 40},
		FoodUpkeep:    2,
		Prerequisites: []Prerequisite{{Type: BuildingArmory, MinCount: 1}},
		Attributes:    []Attribute{AttributeSiegeEngine},
	},
	// Summon units
	UnitShade: {
		Name:       "Shade",
		Power:      2,
		Ticks:      20, // 5h
		Cost:       ResourceCost{Mana: 30},
		ManaUpkeep: 1,
		IsSummon:   true,
		Attributes: []Attribute{AttributeSummon},
	},
	UnitDreadKnight: {
		Name:       "Dread Knight",
		Power:      5,
		Ticks:      32, // 8h
		Cost:       ResourceCost{Mana: 60},
		ManaUpkeep: 2,
		IsSummon:   true,
		Attributes: []Attribute{AttributeSummon, AttributeDeathtouch, AttributeUndead},
	},
}

// UnitOrder defines the display order for the regular unit table.
var UnitOrder = []string{
	UnitRecruit,
	UnitArcher,
	UnitRaider,
	UnitKnight,
	UnitCatapult,
}

// SummonOrder defines the display order for the summon unit table.
var SummonOrder = []string{
	UnitShade,
	UnitDreadKnight,
}

// CanTrain reports whether a kingdom may train the given unit type based on building prerequisites.
func CanTrain(utype string, buildingCounts map[string]int) bool {
	def, ok := UnitDefs[utype]
	if !ok {
		return false
	}
	for _, p := range def.Prerequisites {
		if buildingCounts[p.Type] < p.MinCount {
			return false
		}
	}
	return true
}

// UnitCountMap converts a slice of KingdomUnit rows into a map of unit type → count.
func UnitCountMap(units []db.KingdomUnit) map[string]int {
	m := make(map[string]int, len(units))
	for _, u := range units {
		m[u.UnitType] = u.Count
	}
	return m
}

// CanTrainSummons returns true if the kingdom has unlocked the summon feature.
// Currently gates on mana production being active; update this condition as
// the unlock mechanism is formalised.
func CanTrainSummons(kingdom db.Kingdom) bool {
	return kingdom.ManaPct > 0
}
