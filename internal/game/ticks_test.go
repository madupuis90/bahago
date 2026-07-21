package game

import (
	"context"
	"testing"
	"time"

	"bahago/internal/database/db"
	"bahago/internal/testhelper"
)

func TestNextAlignedBoundary(t *testing.T) {
	t.Parallel()

	loc := time.UTC

	cases := []struct {
		name     string
		now      time.Time
		interval time.Duration
		want     time.Time
	}{
		{
			name:     "mid-hour with 1h interval",
			now:      time.Date(2026, 1, 1, 9, 43, 0, 0, loc),
			interval: time.Hour,
			want:     time.Date(2026, 1, 1, 10, 0, 0, 0, loc),
		},
		{
			name:     "exactly on boundary with 1h interval",
			now:      time.Date(2026, 1, 1, 10, 0, 0, 0, loc),
			interval: time.Hour,
			want:     time.Date(2026, 1, 1, 11, 0, 0, 0, loc),
		},
		{
			name:     "sub-second past boundary with 1h interval",
			now:      time.Date(2026, 1, 1, 10, 0, 0, 1, loc),
			interval: time.Hour,
			want:     time.Date(2026, 1, 1, 11, 0, 0, 0, loc),
		},
		{
			name:     "9:43 with 30m interval",
			now:      time.Date(2026, 1, 1, 9, 43, 0, 0, loc),
			interval: 30 * time.Minute,
			want:     time.Date(2026, 1, 1, 10, 0, 0, 0, loc),
		},
		{
			name:     "9:15 with 30m interval",
			now:      time.Date(2026, 1, 1, 9, 15, 0, 0, loc),
			interval: 30 * time.Minute,
			want:     time.Date(2026, 1, 1, 9, 30, 0, 0, loc),
		},
		{
			name:     "9:47 with 15m interval",
			now:      time.Date(2026, 1, 1, 9, 47, 0, 0, loc),
			interval: 15 * time.Minute,
			want:     time.Date(2026, 1, 1, 10, 0, 0, 0, loc),
		},
		{
			name:     "midnight rollover",
			now:      time.Date(2026, 1, 1, 23, 30, 0, 0, loc),
			interval: time.Hour,
			want:     time.Date(2026, 1, 2, 0, 0, 0, 0, loc),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := alignedTick(tc.now, tc.interval)
			if !got.Equal(tc.want) {
				t.Errorf("nextAlignedBoundary(%v, %v) = %v, want %v", tc.now, tc.interval, got, tc.want)
			}
		})
	}
}

// TestCompletePhases exercises the construction and training completion phases
// the way ProcessTick runs them: decrement + completion inside a single
// transaction. A row reaching zero must grant its output and be removed;
// a row still in progress must only be decremented.
// Requires TEST_DATABASE_URL; skipped automatically when it is unset.
func TestCompletePhases(t *testing.T) {
	pool := testhelper.SetupDB(t)
	q := testhelper.WithRollback(t, pool)
	ctx := context.Background()

	userID, err := q.CreateUser(ctx, db.CreateUserParams{Email: "tick@example.com", PwHash: "h"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	kingdom, err := q.CreateKingdom(ctx, db.CreateKingdomParams{UserID: userID, Name: "Tickton", X: 0, Y: 0})
	if err != nil {
		t.Fatalf("create kingdom: %v", err)
	}

	// Construction finishing this tick (1 → 0).
	if err := q.StartConstruction(ctx, db.StartConstructionParams{
		KingdomID:      kingdom.ID,
		BuildingType:   BuildingMill,
		TicksRemaining: 1,
	}); err != nil {
		t.Fatalf("start construction: %v", err)
	}
	// Training finishing this tick (1 → 0).
	if err := q.StartTraining(ctx, db.StartTrainingParams{
		KingdomID:      kingdom.ID,
		UnitType:       UnitRecruit,
		Count:          7,
		TicksRemaining: 1,
	}); err != nil {
		t.Fatalf("start training: %v", err)
	}

	if err := completeConstructions(ctx, q); err != nil {
		t.Fatalf("completeConstructions: %v", err)
	}
	if err := completeTrainings(ctx, q); err != nil {
		t.Fatalf("completeTrainings: %v", err)
	}

	// Building granted, construction row removed.
	buildings, err := q.GetKingdomBuildings(ctx, kingdom.ID)
	if err != nil {
		t.Fatalf("get buildings: %v", err)
	}
	if len(buildings) != 1 || buildings[0].BuildingType != BuildingMill || buildings[0].Count != 1 {
		t.Fatalf("buildings = %+v; want one mill with count 1", buildings)
	}
	if _, err := q.GetKingdomConstruction(ctx, kingdom.ID); err == nil {
		t.Fatal("construction row still present after completion")
	}

	// Units granted, training row removed.
	units, err := q.GetKingdomUnits(ctx, kingdom.ID)
	if err != nil {
		t.Fatalf("get units: %v", err)
	}
	if len(units) != 1 || units[0].UnitType != UnitRecruit || units[0].Count != 7 {
		t.Fatalf("units = %+v; want 7 recruits", units)
	}
	if _, err := q.GetKingdomTraining(ctx, kingdom.ID); err == nil {
		t.Fatal("training row still present after completion")
	}
}

// TestCompletePhases_InProgress verifies that rows with ticks_remaining > 1 are
// only decremented, not completed.
func TestCompletePhases_InProgress(t *testing.T) {
	pool := testhelper.SetupDB(t)
	q := testhelper.WithRollback(t, pool)
	ctx := context.Background()

	userID, err := q.CreateUser(ctx, db.CreateUserParams{Email: "tick2@example.com", PwHash: "h"})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	kingdom, err := q.CreateKingdom(ctx, db.CreateKingdomParams{UserID: userID, Name: "Tickton2", X: 0, Y: 0})
	if err != nil {
		t.Fatalf("create kingdom: %v", err)
	}
	if err := q.StartConstruction(ctx, db.StartConstructionParams{
		KingdomID:      kingdom.ID,
		BuildingType:   BuildingMill,
		TicksRemaining: 2,
	}); err != nil {
		t.Fatalf("start construction: %v", err)
	}

	if err := completeConstructions(ctx, q); err != nil {
		t.Fatalf("completeConstructions: %v", err)
	}

	c, err := q.GetKingdomConstruction(ctx, kingdom.ID)
	if err != nil {
		t.Fatalf("get construction: %v", err)
	}
	if c.TicksRemaining != 1 {
		t.Fatalf("ticks_remaining = %d; want 1", c.TicksRemaining)
	}
	buildings, err := q.GetKingdomBuildings(ctx, kingdom.ID)
	if err != nil {
		t.Fatalf("get buildings: %v", err)
	}
	if len(buildings) != 0 {
		t.Fatalf("buildings = %+v; want none yet", buildings)
	}
}
