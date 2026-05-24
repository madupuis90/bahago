package buildings

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
	onGetKingdomBuildings func(ctx context.Context, kingdomID int) ([]db.KingdomBuilding, error)
}

func (s *stubQuerier) GetKingdomBuildings(ctx context.Context, kingdomID int) ([]db.KingdomBuilding, error) {
	if s.onGetKingdomBuildings != nil {
		return s.onGetKingdomBuildings(ctx, kingdomID)
	}
	panic("stubQuerier: unexpected call to GetKingdomBuildings")
}

// ── validateBuildingType ──────────────────────────────────────────────────────

func TestValidateBuildingType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{"known_mill", game.BuildingMill, nil},
		{"known_quarry", game.BuildingQuarry, nil},
		{"unknown", "tower", ErrUnknownBuildingType},
		{"empty", "", ErrUnknownBuildingType},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBuildingType(tc.input)
			if tc.wantErr == nil {
				if err != nil {
					t.Errorf("unexpected err: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// ── startConstruction (pre-tx paths only) ─────────────────────────────────────
//
// The orchestrator reaches a real *pgxpool.Pool via BeginTx after the prereq
// check, so the only branches reachable with a nil pool are:
//   - GetKingdomBuildings error → wrapped
//   - CanBuild prerequisite fail → ErrBuildingNotAvailable
//
// Tx-level branches (insufficient resources, serialization failure) live in
// the internal/database/db integration tests.

func TestStartConstruction_GetBuildingsError(t *testing.T) {
	boom := errors.New("connection refused")
	q := &stubQuerier{
		onGetKingdomBuildings: func(_ context.Context, _ int) ([]db.KingdomBuilding, error) {
			return nil, boom
		},
	}
	h := &handler{queries: q}
	err := h.startConstruction(context.Background(), 1, game.BuildingMill)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrapped %v", err, boom)
	}
	if isStartConstructionUserError(err) {
		t.Errorf("unexpected user-error classification: %v", err)
	}
}

func TestStartConstruction_PrereqFails(t *testing.T) {
	// Find a building that has prerequisites; CanBuild on an empty-count map
	// must return false for it.
	var withPrereq string
	for name, def := range game.BuildingDefs {
		if len(def.Prerequisites) > 0 {
			withPrereq = name
			break
		}
	}
	if withPrereq == "" {
		t.Skip("no building with prerequisites in game.BuildingDefs")
	}

	q := &stubQuerier{
		onGetKingdomBuildings: func(_ context.Context, _ int) ([]db.KingdomBuilding, error) {
			return nil, nil // empty — no buildings yet, so prereqs unsatisfiable
		},
	}
	h := &handler{queries: q}
	err := h.startConstruction(context.Background(), 1, withPrereq)
	if !errors.Is(err, ErrBuildingNotAvailable) {
		t.Fatalf("err = %v, want ErrBuildingNotAvailable", err)
	}
}
