package game

import "bahago/internal/database/db"

// Prayer type identifiers. These are stored as-is in the database.
const (
	PrayerManaPrayer = "mana_prayer"
	PrayerWoodPrayer = "wood_prayer"
)

// SkillPrerequisite names a skill that must be unlocked before a prayer becomes available.
// No skills system exists yet; all current prayers have an empty prerequisites slice.
type SkillPrerequisite struct {
	Skill string
}

// PrayerDef describes a prayer type. All values are static config — nothing is stored in the DB.
type PrayerDef struct {
	// Name is the human-readable display name.
	Name string

	// Description is a short flavour text shown on the prayer card.
	Description string

	// DevotionUpkeep is the devotion consumed per tick while the prayer is active.
	DevotionUpkeep int

	// SkillPrerequisites lists skills that must be unlocked before this prayer is available.
	SkillPrerequisites []SkillPrerequisite

	// ResourceBonusPct holds the percentage bonus to each resource's production
	// while the prayer is active.
	ResourceBonusPct ProductionBonus

	// CombatBonusPct maps combat stat names to a percentage bonus applied during combat.
	// Not yet consumed by the combat resolver; defined here for future use.
	CombatBonusPct map[string]int
}

// PrayerDefs is the authoritative static configuration for all prayer types.
var PrayerDefs = map[string]PrayerDef{
	PrayerManaPrayer: {
		Name:               "Mana Prayer",
		Description:        "Your clergy channel divine focus into the arcane, amplifying mana drawn from the ether.",
		DevotionUpkeep:     20,
		SkillPrerequisites: []SkillPrerequisite{},
		ResourceBonusPct:   ProductionBonus{Mana: 20},
		CombatBonusPct:     map[string]int{},
	},
	PrayerWoodPrayer: {
		Name:               "Wood Prayer",
		Description:        "Your clergy bless the forests, coaxing the ancient trees to yield their bounty more freely.",
		DevotionUpkeep:     20,
		SkillPrerequisites: []SkillPrerequisite{},
		ResourceBonusPct:   ProductionBonus{Wood: 20},
		CombatBonusPct:     map[string]int{},
	},
}

// PrayerBonusPct returns the merged resource bonus percentages from all given prayers.
// Prayers with unknown types are silently skipped.
func PrayerBonusPct(prayers []db.KingdomPrayer) ProductionBonus {
	var totals ProductionBonus
	for _, p := range prayers {
		if def, ok := PrayerDefs[p.PrayerType]; ok {
			totals = totals.Add(def.ResourceBonusPct)
		}
	}
	return totals
}
