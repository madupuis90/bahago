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
// halving a fully starving population in roughly 17 hours (69 ticks at 15min intervals).
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

// woodProduction returns wood produced per tick for the given population and allocation percentage.
func woodProduction(population, pct int) int {
	return population * pct / woodDivisor
}

// stoneProduction returns stone produced per tick.
func stoneProduction(population, pct int) int {
	return population * pct / stoneDivisor
}

// foodProduction returns food produced per tick.
func foodProduction(population, pct int) int {
	return population * pct / foodProdDivisor
}

// foodUpkeep returns food consumed per tick (one unit per 30 population).
func foodUpkeep(population int) int {
	return population / foodUpkDivisor
}

// manaProduction returns mana produced per tick.
func manaProduction(population, pct int) int {
	return population * pct / manaDivisor
}

// devotionProduction returns devotion produced per tick.
func devotionProduction(population, pct int) int {
	return population * pct / devotionDivisor
}

// knowledgeProduction returns knowledge produced per tick.
func knowledgeProduction(population, pct int) int {
	return population * pct / knowDivisor
}

// populationProduction returns population growth per tick.
func populationProduction(population, pct int) int {
	return population * pct / popIdleDivisor
}

// ComputeRates calculates production and upkeep for all resources based on kingdom state.
// This is a pure function with no side effects; it is safe to call in tests.
// Building bonuses, skill modifiers, and other multipliers will be added as parameters later.
func ComputeRates(k db.Kingdom) ResourceRates {
	fp := foodProduction(k.Population, k.FoodPct)
	fu := foodUpkeep(k.Population)
	popLoss := starvationLoss(k.Population, k.Food, fp, fu)
	// Population does not grow while starving — a food deficit suppresses births entirely.
	popProd := populationProduction(k.Population, k.IdlePct)
	if popLoss > 0 {
		popProd = 0
	}
	return ResourceRates{
		WoodProduction:       woodProduction(k.Population, k.WoodPct),
		WoodUpkeep:           0,
		StoneProduction:      stoneProduction(k.Population, k.StonePct),
		StoneUpkeep:          0,
		FoodProduction:       fp,
		FoodUpkeep:           fu,
		ManaProduction:       manaProduction(k.Population, k.ManaPct),
		ManaUpkeep:           0,
		DevotionProduction:   devotionProduction(k.Population, k.DevotionPct),
		DevotionUpkeep:       0,
		KnowledgeProduction:  knowledgeProduction(k.Population, k.KnowledgePct),
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
	return population * deficit / (foodUpkeep * starvationDivisor)
}
