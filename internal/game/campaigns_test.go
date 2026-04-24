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
	if err := q.UpsertKingdomUnits(ctx, db.UpsertKingdomUnitsParams{
		KingdomID: attacker.ID,
		UnitType:  UnitRecruit,
		Count:     10,
	}); err != nil {
		t.Fatalf("upsert units: %v", err)
	}

	// Create campaign with ticks_remaining=1, action_ticks=1, travel_ticks=1
	// so each Advance call triggers exactly one transition.
	campaign, err := q.CreateCampaignIfAvailable(ctx, db.CreateCampaignIfAvailableParams{
		KingdomID:       attacker.ID,
		TargetKingdomID: target.ID,
		UnitType:        UnitRecruit,
		SendCount:       5,
		Action:          "attack",
		TicksRemaining:  1,
		ActionTicks:     1,
		TravelTicks:     1,
	})
	if err != nil {
		t.Fatalf("CreateCampaignIfAvailable: %v", err)
	}
	if campaign.Status != "en_route" {
		t.Fatalf("initial status = %q, want en_route", campaign.Status)
	}

	findCampaign := func(t *testing.T) (db.KingdomCampaign, bool) {
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
		return db.KingdomCampaign{}, false
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

	// Advance 3: returning → deleted
	if err := AdvanceCampaigns(ctx, q); err != nil {
		t.Fatalf("AdvanceCampaigns (3): %v", err)
	}
	if _, ok := findCampaign(t); ok {
		t.Error("campaign still present after advance 3, expected it to be deleted")
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
			wantTicks: 12,
		},
		{
			name: "adjacent tile enforces minimum",
			x1:   0, y1: 0, x2: 1, y2: 0,
			wantTicks: 12,
		},
		{
			name: "diagonal 1 step enforces minimum",
			x1:   0, y1: 0, x2: 1, y2: 1,
			wantTicks: 12,
		},
		{
			name: "exactly minimum distance",
			x1:   0, y1: 0, x2: 12, y2: 0,
			wantTicks: 12,
		},
		{
			name: "one beyond minimum",
			x1:   0, y1: 0, x2: 13, y2: 0,
			wantTicks: 13,
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
