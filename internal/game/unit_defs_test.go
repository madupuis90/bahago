package game

import (
	"testing"

	"bahago/internal/database/db"
)

func TestCanTrain(t *testing.T) {
	tests := []struct {
		name           string
		utype          string
		buildingCounts map[string]int
		want           bool
	}{
		{
			name:  "unknown unit type",
			utype: "dragon",
			want:  false,
		},
		{
			name:           "recruit has no prerequisites",
			utype:          UnitRecruit,
			buildingCounts: map[string]int{},
			want:           true,
		},
		{
			name:           "archer has no prerequisites",
			utype:          UnitArcher,
			buildingCounts: map[string]int{},
			want:           true,
		},
		{
			name:           "raider requires mill, not present",
			utype:          UnitRaider,
			buildingCounts: map[string]int{},
			want:           false,
		},
		{
			name:           "raider requires mill, present",
			utype:          UnitRaider,
			buildingCounts: map[string]int{BuildingMill: 1},
			want:           true,
		},
		{
			name:           "knight requires armory, not present",
			utype:          UnitKnight,
			buildingCounts: map[string]int{},
			want:           false,
		},
		{
			name:           "knight requires armory, present",
			utype:          UnitKnight,
			buildingCounts: map[string]int{BuildingArmory: 1},
			want:           true,
		},
		{
			name:           "shade is a summon unit with no building prerequisites",
			utype:          UnitShade,
			buildingCounts: map[string]int{},
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTrain(tt.utype, tt.buildingCounts)
			if got != tt.want {
				t.Errorf("CanTrain(%q, %v) = %v, want %v", tt.utype, tt.buildingCounts, got, tt.want)
			}
		})
	}
}

func TestCanTrainSummons(t *testing.T) {
	t.Run("mana allocation zero blocks summons", func(t *testing.T) {
		k := db.Kingdom{ManaPct: 0}
		if CanTrainSummons(k) {
			t.Error("expected false when ManaPct=0")
		}
	})

	t.Run("mana allocation positive allows summons", func(t *testing.T) {
		k := db.Kingdom{ManaPct: 10}
		if !CanTrainSummons(k) {
			t.Error("expected true when ManaPct=10")
		}
	})
}

func TestUnitCountMap(t *testing.T) {
	t.Run("empty slice", func(t *testing.T) {
		m := UnitCountMap(nil)
		if len(m) != 0 {
			t.Errorf("expected empty map, got %v", m)
		}
	})

	t.Run("single unit type", func(t *testing.T) {
		m := UnitCountMap([]db.KingdomUnit{
			{UnitType: UnitRecruit, Count: 50},
		})
		if m[UnitRecruit] != 50 {
			t.Errorf("recruit count = %d, want 50", m[UnitRecruit])
		}
	})

	t.Run("multiple unit types", func(t *testing.T) {
		m := UnitCountMap([]db.KingdomUnit{
			{UnitType: UnitRecruit, Count: 100},
			{UnitType: UnitKnight, Count: 10},
		})
		if m[UnitRecruit] != 100 {
			t.Errorf("recruit count = %d, want 100", m[UnitRecruit])
		}
		if m[UnitKnight] != 10 {
			t.Errorf("knight count = %d, want 10", m[UnitKnight])
		}
	})
}
