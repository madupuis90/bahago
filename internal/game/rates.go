package game

import (
	"bahago/internal/database/db"
)

// ResourceRates holds the computed production and upkeep values for all resources.
type ResourceRates struct {
	WoodProduction       int
	WoodUpkeep           int
	StoneProduction      int
	StoneUpkeep          int
	FoodProduction       int
	FoodUpkeep           int
	ManaProduction       int
	ManaUpkeep           int
	DevotionProduction   int
	DevotionUpkeep       int
	KnowledgeProduction  int
	KnowledgeUpkeep      int
	PopulationProduction int
	PopulationUpkeep     int
}

// Divisors for resource production: output = population * pct / divisor.
// Using a single combined divisor avoids double integer truncation when population is small.
// The human-readable interpretation is: divisor/100 workers are required to produce 1 unit per tick.
// Food upkeep divisor is direct: 1 food consumed per foodUpkDivisor population per tick.
// Food break-even at 30% allocation requires foodProdDivisor = 30 * foodUpkDivisor.
//
// starvationDivisor controls the maximum population loss per tick when fully starving:
// loss = population / starvationDivisor. A value of 100 means at most 1% per tick,
// halving a fully starving population in roughly 69 hours (69 ticks at 1h intervals).
// Partial shortages cause proportionally less loss, scaling linearly with the deficit ratio.
const (
	starvationDivisor = 100
	woodDivisor       = 10 * 100 // 10 workers → 1 wood/tick
	stoneDivisor      = 20 * 100 // 20 workers → 1 stone/tick
	foodProdDivisor   = 9 * 100  // 9 workers → 1 food/tick
	foodUpkDivisor    = 30       // 1 food consumed per 30 population/tick (break-even at 30% food)
	manaDivisor       = 15 * 100 // 15 workers → 1 mana/tick
	devotionDivisor   = 15 * 100 // 15 workers → 1 devotion/tick
	knowDivisor       = 20 * 100 // 20 workers → 1 knowledge/tick
	popIdleDivisor    = 25 * 100 // 25 idle -> 1 Pop/tick
)

// woodProduction returns wood produced per tick for the given population, allocation percentage,
// and building bonus percentage. The bonus is folded into the numerator before the single
// integer division to avoid double truncation.
func woodProduction(population, pct, bonusPct int) int {
	return population * pct * (100 + bonusPct) / (woodDivisor * 100)
}

// stoneProduction returns stone produced per tick.
func stoneProduction(population, pct, bonusPct int) int {
	return population * pct * (100 + bonusPct) / (stoneDivisor * 100)
}

// foodProduction returns food produced per tick.
func foodProduction(population, pct, bonusPct int) int {
	return population * pct * (100 + bonusPct) / (foodProdDivisor * 100)
}

func foodUpkeep(population int) int {
	return population / foodUpkDivisor
}

func manaProduction(population, pct, bonusPct int) int {
	return population * pct * (100 + bonusPct) / (manaDivisor * 100)
}

func devotionProduction(population, pct, bonusPct int) int {
	return population * pct * (100 + bonusPct) / (devotionDivisor * 100)
}

func knowledgeProduction(population, pct, bonusPct int) int {
	return population * pct * (100 + bonusPct) / (knowDivisor * 100)
}

func populationProduction(population, pct int) int {
	return population * pct / popIdleDivisor
}

// devotionUpkeep returns the sum of devotion upkeep across all given prayers.
// Unknown prayer types are silently skipped (they have no upkeep to charge).
func devotionUpkeep(prayers []db.KingdomPrayer) int {
	var total int
	for _, p := range prayers {
		if def, ok := PrayerDefs[p.PrayerType]; ok {
			total += def.DevotionUpkeep
		}
	}
	return total
}

// ComputeBonuses returns the merged production bonus percentages for a kingdom,
// combining building bonuses with resource bonuses from prayers targeting it.
func ComputeBonuses(buildings []db.KingdomBuilding, targetedPrayers []db.KingdomPrayer) ProductionBonus {
	return BuildingBonusPct(BuildingCountMap(buildings)).Add(PrayerBonusPct(targetedPrayers))
}

// ComputeRates calculates production and upkeep for all resources based on kingdom state.
// This is a pure function with no side effects; it is safe to call in tests.
// Building bonus percentages are folded into the single integer division inside each production
// function to avoid double truncation from two separate integer divisions.
//
// targetedPrayers are prayers whose target_kingdom_id is this kingdom — their resource bonuses
// apply here. castPrayers are prayers whose kingdom_id is this kingdom — their devotion upkeep
// is charged here. For self-targeted prayers both slices contain the same rows.
func ComputeRates(k db.Kingdom, buildings []db.KingdomBuilding, targetedPrayers, castPrayers []db.KingdomPrayer) ResourceRates {
	bonus := ComputeBonuses(buildings, targetedPrayers)

	prayerDevotionUpkeep := devotionUpkeep(castPrayers)

	fp := foodProduction(k.Population, k.FoodPct, bonus.Food)
	fu := foodUpkeep(k.Population)
	popLoss := starvationLoss(k.Population, k.Food, fp, fu)
	// Population does not grow while starving — a food deficit suppresses births entirely.
	popProd := populationProduction(k.Population, k.IdlePct)
	if popLoss > 0 {
		popProd = 0
	}
	return ResourceRates{
		WoodProduction:       woodProduction(k.Population, k.WoodPct, bonus.Wood),
		WoodUpkeep:           0,
		StoneProduction:      stoneProduction(k.Population, k.StonePct, bonus.Stone),
		StoneUpkeep:          0,
		FoodProduction:       fp,
		FoodUpkeep:           fu,
		ManaProduction:       manaProduction(k.Population, k.ManaPct, bonus.Mana),
		ManaUpkeep:           0,
		DevotionProduction:   devotionProduction(k.Population, k.DevotionPct, bonus.Devotion),
		DevotionUpkeep:       prayerDevotionUpkeep,
		KnowledgeProduction:  knowledgeProduction(k.Population, k.KnowledgePct, bonus.Knowledge),
		KnowledgeUpkeep:      0,
		PopulationProduction: popProd,
		PopulationUpkeep:     popLoss,
	}
}

// starvationLoss returns the population lost to starvation this tick.
// Loss scales linearly with the deficit ratio: 0 when fully fed, population/starvationDivisor
// when there is no food at all. Returns 0 if foodUpkeep is zero (impossible in practice).
func starvationLoss(population, food, foodProduction, foodUpkeep int) int {
	if foodUpkeep <= 0 {
		return 0
	}
	deficit := foodUpkeep - (food + foodProduction)
	if deficit <= 0 {
		return 0
	}
	// Use float64 to avoid int64 overflow when population and deficit are both large.
	// At ~47B population and ~1.6B upkeep, population*deficit ≈ 7.4×10¹⁹ which exceeds
	// int64 max (9.2×10¹⁸). float64 precision is sufficient for game purposes.
	return int(float64(population) * float64(deficit) / (float64(foodUpkeep) * float64(starvationDivisor)))
}
