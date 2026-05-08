package game

import (
	"testing"

	"bahago/internal/database/db"
)

func TestComputeRates_WoodProduction(t *testing.T) {
	tests := []struct {
		name       string
		population int
		woodPct    int
		buildings  []db.KingdomBuilding
		wantWood   int
	}{
		{
			name:       "zero population",
			population: 0,
			woodPct:    100,
			wantWood:   0,
		},
		{
			name:       "zero allocation",
			population: 3000,
			woodPct:    0,
			wantWood:   0,
		},
		{
			name:       "full allocation no buildings",
			population: 1000,
			woodPct:    100,
			wantWood:   100, // 1000 * 100 * 100 / (10*100*100) = 10_000_000/100_000 = 100
		},
		{
			name:       "half allocation no buildings",
			population: 1000,
			woodPct:    50,
			wantWood:   50, // 1000 * 50 * 100 / 100_000 = 50
		},
		{
			name:       "5 mills give 50pct bonus",
			population: 1000,
			woodPct:    50,
			buildings: []db.KingdomBuilding{
				{BuildingType: BuildingMill, Count: 5},
			},
			wantWood: 75, // 1000 * 50 * 150 / 100_000 = 75
		},
		{
			name:       "mill plus factory stacks bonuses",
			population: 1000,
			woodPct:    100,
			buildings: []db.KingdomBuilding{
				{BuildingType: BuildingMill, Count: 5},    // +50% wood
				{BuildingType: BuildingFactory, Count: 2}, // +50% wood
			},
			wantWood: 200, // 1000 * 100 * 200 / 100_000 = 200
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := db.Kingdom{Population: tt.population, WoodPct: tt.woodPct, FoodPct: 30}
			// FoodPct: 30 keeps the kingdom at break-even so starvation doesn't affect pop.
			got := ComputeRates(k, tt.buildings, nil, nil)
			if got.WoodProduction != tt.wantWood {
				t.Errorf("WoodProduction = %d, want %d", got.WoodProduction, tt.wantWood)
			}
		})
	}
}

func TestComputeRates_Starvation(t *testing.T) {
	tests := []struct {
		name       string
		population int
		food       int
		foodPct    int
		wantLoss   int
		wantPop    int // PopulationProduction
	}{
		{
			name:       "break-even suppresses starvation",
			population: 3000, // upkeep = 3000/30 = 100; prod = 3000*30/900 = 100
			food:       0,
			foodPct:    30,
			wantLoss:   0,
			wantPop:    0, // idlePct=0 so growth is always 0
		},
		{
			name:       "full starvation loses population/100",
			population: 3000, // upkeep = 100, prod = 0 → loss = int(3000*100/(100*100)) = 30
			food:       0,
			foodPct:    0,
			wantLoss:   30,
			wantPop:    0, // suppressed by starvation
		},
		{
			name:       "half starvation halves the loss",
			population: 1200, // upkeep = 40, prod = 20 (foodPct=15) → deficit=20, loss = int(1200*20/(40*100)) = 6
			food:       0,
			foodPct:    15,
			wantLoss:   6,
			wantPop:    0,
		},
		{
			name:       "food stockpile covers deficit, no loss",
			population: 3000, // upkeep = 100, prod = 0, food = 150 → deficit = 100-(150+0) < 0 → 0
			food:       150,
			foodPct:    0,
			wantLoss:   0,
			wantPop:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := db.Kingdom{
				Population: tt.population,
				Food:       tt.food,
				FoodPct:    tt.foodPct,
			}
			got := ComputeRates(k, nil, nil, nil)
			if got.PopulationUpkeep != tt.wantLoss {
				t.Errorf("PopulationUpkeep (starvation loss) = %d, want %d", got.PopulationUpkeep, tt.wantLoss)
			}
			if got.PopulationProduction != tt.wantPop {
				t.Errorf("PopulationProduction = %d, want %d", got.PopulationProduction, tt.wantPop)
			}
		})
	}
}

func TestComputeRates_PopulationGrowthSuppressedWhenStarving(t *testing.T) {
	// Kingdom with idle workers (should produce population) but no food → growth must be 0.
	k := db.Kingdom{
		Population: 3000,
		Food:       0,
		FoodPct:    0,
		IdlePct:    10, // would produce 3000*10/2500=12 normally
	}
	got := ComputeRates(k, nil, nil, nil)
	if got.PopulationProduction != 0 {
		t.Errorf("PopulationProduction = %d, want 0 while starving", got.PopulationProduction)
	}
	if got.PopulationUpkeep == 0 {
		t.Error("PopulationUpkeep = 0, expected starvation loss > 0")
	}
}

func TestComputeRates_PopulationGrowsWithIdleWorkers(t *testing.T) {
	// Enough food, idle workers present → population grows.
	k := db.Kingdom{
		Population: 3000,
		Food:       0,
		FoodPct:    30, // break-even
		IdlePct:    10, // 3000*10/2500 = 12
	}
	got := ComputeRates(k, nil, nil, nil)
	if got.PopulationUpkeep != 0 {
		t.Errorf("expected no starvation, got PopulationUpkeep = %d", got.PopulationUpkeep)
	}
	if got.PopulationProduction != 12 {
		t.Errorf("PopulationProduction = %d, want 12", got.PopulationProduction)
	}
}

func TestComputeRates_FoodUpkeep(t *testing.T) {
	k := db.Kingdom{Population: 3000, FoodPct: 50}
	got := ComputeRates(k, nil, nil, nil)
	// foodUpkeep = 3000/30 = 100
	if got.FoodUpkeep != 100 {
		t.Errorf("FoodUpkeep = %d, want 100", got.FoodUpkeep)
	}
}
