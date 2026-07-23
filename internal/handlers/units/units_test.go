package units

import (
	"context"
	"errors"
	"testing"

	"bahago/internal/database/db"
	"bahago/internal/game"
)

// ── Stub querier ──────────────────────────────────────────────────────────────

type stubQuerier struct {
	db.Querier
}

// ── validateTrainInput ────────────────────────────────────────────────────────

func TestValidateTrainInput(t *testing.T) {
	base := func() *trainInput {
		return &trainInput{UnitType: game.UnitRecruit, Count: 5}
	}
	tests := []struct {
		name     string
		mutate   func(*trainInput)
		wantErrs []error
	}{
		{"valid", func(_ *trainInput) {}, nil},
		{"unknown_type", func(in *trainInput) { in.UnitType = "wizard" }, []error{ErrUnknownUnitType}},
		{"count_zero", func(in *trainInput) { in.Count = 0 }, []error{ErrInvalidCount}},
		{"count_negative", func(in *trainInput) { in.Count = -1 }, []error{ErrInvalidCount}},
		{"count_too_large", func(in *trainInput) { in.Count = game.MaxUnitInput + 1 }, []error{ErrCountTooLarge}},
		{
			name:     "both_invalid",
			mutate:   func(in *trainInput) { in.UnitType = "wizard"; in.Count = 0 },
			wantErrs: []error{ErrUnknownUnitType, ErrInvalidCount},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := base()
			tc.mutate(in)
			got := validateTrainInput(in)
			if len(got) != len(tc.wantErrs) {
				t.Fatalf("got %d errs (%v), want %d (%v)", len(got), got, len(tc.wantErrs), tc.wantErrs)
			}
			for i, want := range tc.wantErrs {
				if !errors.Is(got[i], want) {
					t.Errorf("errs[%d] = %v, want %v", i, got[i], want)
				}
			}
		})
	}
}

// ── trainUnits (pre-tx paths only) ────────────────────────────────────────────
//
// Reachable without a real pool:
//   - Summons-unlock fail → ErrSummonsNotUnlocked
//   - CanTrain fail → ErrUnitNotAvailable
//
// Buildings are fetched by the handler before calling trainUnits, so DB error
// paths for GetKingdomBuildings are exercised at the handler level.
// Tx-level branches (insufficient resources, serialization failure) live in
// internal/database/db integration tests.

func TestTrainUnits_SummonsNotUnlocked(t *testing.T) {
	// Pick a summon unit; a fresh kingdom with no mana production cannot
	// unlock summoning.
	var summonName string
	for name, def := range game.UnitDefs {
		if def.Category == game.CategorySummon {
			summonName = name
			break
		}
	}
	if summonName == "" {
		t.Skip("no summon units in game.UnitDefs")
	}

	h := &handler{queries: &stubQuerier{}}
	err := h.trainUnits(context.Background(), &db.Kingdom{ID: 1}, &trainInput{UnitType: summonName, Count: 1}, nil)
	if !errors.Is(err, ErrSummonsNotUnlocked) {
		t.Fatalf("err = %v, want ErrSummonsNotUnlocked", err)
	}
}

func TestTrainUnits_UnitNotAvailable(t *testing.T) {
	// Find a unit that requires a building prerequisite — empty building list
	// then yields ErrUnitNotAvailable.
	var gated string
	for name := range game.UnitDefs {
		if !game.CanTrain(name, map[string]int{}) {
			gated = name
			break
		}
	}
	if gated == "" {
		t.Skip("every unit is trainable without buildings")
	}

	// Skip summons since they'd hit the unlock guard first.
	if game.UnitDefs[gated].Category == game.CategorySummon {
		for name, def := range game.UnitDefs {
			if def.Category != game.CategorySummon && !game.CanTrain(name, map[string]int{}) {
				gated = name
				break
			}
		}
	}

	h := &handler{queries: &stubQuerier{}}
	err := h.trainUnits(context.Background(), &db.Kingdom{ID: 1}, &trainInput{UnitType: gated, Count: 1}, nil)
	if !errors.Is(err, ErrUnitNotAvailable) {
		t.Fatalf("err = %v, want ErrUnitNotAvailable", err)
	}
}
