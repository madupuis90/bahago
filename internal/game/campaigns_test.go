package game

import (
	"context"
	"testing"

	"bahago/internal/database/db"
	"bahago/internal/testhelper"
)

// TestAdvanceCampaigns_StateMachine walks a campaign through all three transitions:
// en_route → active → returning → deleted.
// Requires TEST_DATABASE_URL; skipped automatically when it is unset.
func TestAdvanceCampaigns_StateMachine(t *testing.T) {
	pool := testhelper.SetupDB(t)
	q := testhelper.WithRollback(t, pool)
	ctx := context.Background()

	atkUserID, err := q.CreateUser(ctx, db.CreateUserParams{Email: "smAtk@example.com", PwHash: "h"})
	if err != nil {
		t.Fatalf("create attacker user: %v", err)
	}
	tgtUserID, err := q.CreateUser(ctx, db.CreateUserParams{Email: "smTgt@example.com", PwHash: "h"})
	if err != nil {
		t.Fatalf("create target user: %v", err)
	}
	attacker, err := q.CreateKingdom(ctx, db.CreateKingdomParams{UserID: atkUserID, Name: "SMAtk", X: 0, Y: 0})
	if err != nil {
		t.Fatalf("create attacker kingdom: %v", err)
	}
	target, err := q.CreateKingdom(ctx, db.CreateKingdomParams{UserID: tgtUserID, Name: "SMTgt", X: 5, Y: 0})
	if err != nil {
		t.Fatalf("create target kingdom: %v", err)
	}
	legion, err := q.CreateLegion(ctx, db.CreateLegionParams{
		KingdomID: attacker.ID,
		Cap:       MaxLegionsPerKingdom,
	})
	if err != nil {
		t.Fatalf("create legion: %v", err)
	}
	if err := q.UpsertLegionUnit(ctx, db.UpsertLegionUnitParams{
		LegionID: legion.ID,
		UnitType: UnitRecruit,
		Count:    10,
	}); err != nil {
		t.Fatalf("upsert legion unit: %v", err)
	}

	// Create campaign with ticks_remaining=1, action_ticks=1, travel_ticks=3
	// (3 is the schema minimum). The en_route→active and active→returning
	// transitions still take one advance each; the returning leg requires
	// travel_ticks=3 decrements before deletion.
	campaign, err := q.CreateCampaign(ctx, db.CreateCampaignParams{
		KingdomID:       attacker.ID,
		TargetKingdomID: target.ID,
		LegionID:        legion.ID,
		Action:          "attack",
		TicksRemaining:  1,
		ActionTicks:     1,
		TravelTicks:     3,
	})
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	if err := q.SnapshotLegionUnitsIntoCampaign(ctx, db.SnapshotLegionUnitsIntoCampaignParams{
		CampaignID: campaign.ID,
		LegionID:   legion.ID,
	}); err != nil {
		t.Fatalf("SnapshotLegionUnitsIntoCampaign: %v", err)
	}
	if err := q.ClearLegionUnits(ctx, legion.ID); err != nil {
		t.Fatalf("ClearLegionUnits: %v", err)
	}
	if campaign.Status != "en_route" {
		t.Fatalf("initial status = %q, want en_route", campaign.Status)
	}

	findCampaign := func(t *testing.T) (db.GetCampaignsForKingdomRow, bool) {
		t.Helper()
		campaigns, err := q.GetCampaignsForKingdom(ctx, attacker.ID)
		if err != nil {
			t.Fatalf("GetCampaignsForKingdom: %v", err)
		}
		for _, c := range campaigns {
			if c.ID == campaign.ID {
				return c, true
			}
		}
		return db.GetCampaignsForKingdomRow{}, false
	}

	// Advance 1: en_route → active
	if err := AdvanceCampaigns(ctx, q); err != nil {
		t.Fatalf("AdvanceCampaigns (1): %v", err)
	}
	if c, ok := findCampaign(t); !ok {
		t.Fatal("campaign missing after advance 1")
	} else if c.Status != "active" {
		t.Errorf("after advance 1: status = %q, want active", c.Status)
	}

	// Advance 2: active → returning
	if err := AdvanceCampaigns(ctx, q); err != nil {
		t.Fatalf("AdvanceCampaigns (2): %v", err)
	}
	if c, ok := findCampaign(t); !ok {
		t.Fatal("campaign missing after advance 2")
	} else if c.Status != "returning" {
		t.Errorf("after advance 2: status = %q, want returning", c.Status)
	}

	// Advances 3-4: returning, ticks_remaining decrements but stays > 0.
	for i := 3; i <= 4; i++ {
		if err := AdvanceCampaigns(ctx, q); err != nil {
			t.Fatalf("AdvanceCampaigns (%d): %v", i, err)
		}
		if c, ok := findCampaign(t); !ok {
			t.Fatalf("campaign missing after advance %d", i)
		} else if c.Status != "returning" {
			t.Errorf("after advance %d: status = %q, want returning", i, c.Status)
		}
	}

	// Advance 5: returning → deleted (ticks_remaining hits 0).
	if err := AdvanceCampaigns(ctx, q); err != nil {
		t.Fatalf("AdvanceCampaigns (5): %v", err)
	}
	if _, ok := findCampaign(t); ok {
		t.Error("campaign still present after advance 5, expected it to be deleted")
	}
}

func TestTravelTicks(t *testing.T) {
	tests := []struct {
		name      string
		x1, y1    int
		x2, y2    int
		wantTicks int
	}{
		{
			name: "same tile enforces minimum",
			x1:   5, y1: 5, x2: 5, y2: 5,
			wantTicks: 3,
		},
		{
			name: "adjacent tile enforces minimum",
			x1:   0, y1: 0, x2: 1, y2: 0,
			wantTicks: 3,
		},
		{
			name: "diagonal 1 step enforces minimum",
			x1:   0, y1: 0, x2: 1, y2: 1,
			wantTicks: 3,
		},
		{
			name: "exactly at minimum boundary",
			x1:   0, y1: 0, x2: 3, y2: 0,
			wantTicks: 3,
		},
		{
			name: "one beyond minimum",
			x1:   0, y1: 0, x2: 4, y2: 0,
			wantTicks: 4,
		},
		{
			name: "non-square uses max of dx dy",
			x1:   0, y1: 0, x2: 5, y2: 20,
			wantTicks: 20,
		},
		{
			name: "negative direction works correctly",
			x1:   20, y1: 20, x2: 5, y2: 5,
			wantTicks: 15,
		},
		{
			name: "corner to corner of 64x64 world",
			x1:   0, y1: 0, x2: 63, y2: 63,
			wantTicks: 63,
		},
		{
			name: "asymmetric diagonal uses larger axis",
			x1:   10, y1: 10, x2: 40, y2: 25,
			wantTicks: 30, // dx=30, dy=15 → max=30
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TravelTicks(tt.x1, tt.y1, tt.x2, tt.y2)
			if got != tt.wantTicks {
				t.Errorf("TravelTicks(%d,%d,%d,%d) = %d, want %d",
					tt.x1, tt.y1, tt.x2, tt.y2, got, tt.wantTicks)
			}
		})
	}
}
