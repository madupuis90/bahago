package game

import (
	"testing"

	"bahago/internal/database/db"
)

func TestBuildingCountMap(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		m := BuildingCountMap(nil)
		if len(m) != 0 {
			t.Errorf("expected empty map, got %v", m)
		}
	})

	t.Run("single entry", func(t *testing.T) {
		m := BuildingCountMap([]db.KingdomBuilding{
			{BuildingType: BuildingMill, Count: 3},
		})
		if m[BuildingMill] != 3 {
			t.Errorf("mill count = %d, want 3", m[BuildingMill])
		}
	})

	t.Run("multiple entries", func(t *testing.T) {
		m := BuildingCountMap([]db.KingdomBuilding{
			{BuildingType: BuildingMill, Count: 5},
			{BuildingType: BuildingQuarry, Count: 2},
		})
		if m[BuildingMill] != 5 {
			t.Errorf("mill count = %d, want 5", m[BuildingMill])
		}
		if m[BuildingQuarry] != 2 {
			t.Errorf("quarry count = %d, want 2", m[BuildingQuarry])
		}
	})
}

func TestCanBuild(t *testing.T) {
	tests := []struct {
		name   string
		btype  string
		counts map[string]int
		want   bool
	}{
		{
			name:  "unknown building type",
			btype: "dragon_lair",
			want:  false,
		},
		{
			name:   "mill with no existing buildings",
			btype:  BuildingMill,
			counts: map[string]int{},
			want:   true,
		},
		{
			name:   "mill at max count",
			btype:  BuildingMill,
			counts: map[string]int{BuildingMill: 5},
			want:   false,
		},
		{
			name:   "factory without prereq mill",
			btype:  BuildingFactory,
			counts: map[string]int{},
			want:   false,
		},
		{
			name:   "factory with prereq mill",
			btype:  BuildingFactory,
			counts: map[string]int{BuildingMill: 1},
			want:   true,
		},
		{
			name:   "factory at max count",
			btype:  BuildingFactory,
			counts: map[string]int{BuildingMill: 1, BuildingFactory: 2},
			want:   false,
		},
		{
			name:  "armory requires both factory and blacksmith",
			btype: BuildingArmory,
			counts: map[string]int{
				BuildingMill:    1,
				BuildingFactory: 1,
				// missing blacksmith
			},
			want: false,
		},
		{
			name:  "armory with all prerequisites met",
			btype: BuildingArmory,
			counts: map[string]int{
				BuildingMill:       1,
				BuildingFactory:    1,
				BuildingQuarry:     1,
				BuildingBlacksmith: 1,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanBuild(tt.btype, tt.counts)
			if got != tt.want {
				t.Errorf("CanBuild(%q, %v) = %v, want %v", tt.btype, tt.counts, got, tt.want)
			}
		})
	}
}

func TestBuildingBonusPct(t *testing.T) {
	t.Run("empty counts", func(t *testing.T) {
		bonus := BuildingBonusPct(map[string]int{})
		if bonus != (ProductionBonus{}) {
			t.Errorf("expected zero BonusMap, got %v", bonus)
		}
	})

	t.Run("single mill gives wood bonus", func(t *testing.T) {
		bonus := BuildingBonusPct(map[string]int{BuildingMill: 1})
		if bonus.Wood != 10 {
			t.Errorf("wood bonus = %d, want 10", bonus.Wood)
		}
	})

	t.Run("five mills stack to 50pct wood bonus", func(t *testing.T) {
		bonus := BuildingBonusPct(map[string]int{BuildingMill: 5})
		if bonus.Wood != 50 {
			t.Errorf("wood bonus = %d, want 50", bonus.Wood)
		}
	})

	t.Run("mill and factory stack wood bonuses", func(t *testing.T) {
		bonus := BuildingBonusPct(map[string]int{
			BuildingMill:    5, // +50% wood
			BuildingFactory: 2, // +50% wood
		})
		if bonus.Wood != 100 {
			t.Errorf("wood bonus = %d, want 100", bonus.Wood)
		}
	})

	t.Run("different buildings contribute to different resources", func(t *testing.T) {
		bonus := BuildingBonusPct(map[string]int{
			BuildingMill:   3, // +30% wood
			BuildingQuarry: 2, // +20% stone
		})
		if bonus.Wood != 30 {
			t.Errorf("wood bonus = %d, want 30", bonus.Wood)
		}
		if bonus.Stone != 20 {
			t.Errorf("stone bonus = %d, want 20", bonus.Stone)
		}
		if bonus.Food != 0 {
			t.Errorf("food bonus = %d, want 0", bonus.Food)
		}
	})

	t.Run("zero count building contributes no bonus", func(t *testing.T) {
		bonus := BuildingBonusPct(map[string]int{BuildingMill: 0})
		if bonus.Wood != 0 {
			t.Errorf("wood bonus = %d, want 0 for zero-count building", bonus.Wood)
		}
	})
}
