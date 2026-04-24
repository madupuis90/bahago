package db_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"bahago/internal/database/db"
	"bahago/internal/game"
	"bahago/internal/testhelper"
)

// seedCampaignFixture creates two kingdoms (attacker + target) with the attacker
// having the specified number of recruits. Returns both kingdoms.
func seedCampaignFixture(t *testing.T, q db.Querier, attackerUnits int) (attacker, target db.Kingdom) {
	t.Helper()
	ctx := context.Background()

	atkUserID, err := q.CreateUser(ctx, db.CreateUserParams{Email: "atk@example.com", PwHash: "h"})
	if err != nil {
		t.Fatalf("seedCampaignFixture: create attacker user: %v", err)
	}
	tgtUserID, err := q.CreateUser(ctx, db.CreateUserParams{Email: "tgt@example.com", PwHash: "h"})
	if err != nil {
		t.Fatalf("seedCampaignFixture: create target user: %v", err)
	}

	attacker, err = q.CreateKingdom(ctx, db.CreateKingdomParams{
		UserID: atkUserID, Name: "Attackia", X: 0, Y: 0,
	})
	if err != nil {
		t.Fatalf("seedCampaignFixture: create attacker kingdom: %v", err)
	}
	target, err = q.CreateKingdom(ctx, db.CreateKingdomParams{
		UserID: tgtUserID, Name: "Targetia", X: 20, Y: 20,
	})
	if err != nil {
		t.Fatalf("seedCampaignFixture: create target kingdom: %v", err)
	}

	if attackerUnits > 0 {
		if err := q.UpsertKingdomUnits(ctx, db.UpsertKingdomUnitsParams{
			KingdomID: attacker.ID,
			UnitType:  game.UnitRecruit,
			Count:     attackerUnits,
		}); err != nil {
			t.Fatalf("seedCampaignFixture: upsert units: %v", err)
		}
	}

	return attacker, target
}

func TestCreateCampaignIfAvailable_Success(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	attacker, target := seedCampaignFixture(t, q, 50)

	travelTicks := game.TravelTicks(attacker.X, attacker.Y, target.X, target.Y)
	campaign, err := q.CreateCampaignIfAvailable(context.Background(), db.CreateCampaignIfAvailableParams{
		KingdomID:       attacker.ID,
		TargetKingdomID: target.ID,
		UnitType:        game.UnitRecruit,
		SendCount:       20,
		Action:          "attack",
		TicksRemaining:  travelTicks,
		ActionTicks:     4,
		TravelTicks:     travelTicks,
	})
	if err != nil {
		t.Fatalf("CreateCampaignIfAvailable: %v", err)
	}
	if campaign.KingdomID != attacker.ID {
		t.Errorf("campaign.KingdomID = %d, want %d", campaign.KingdomID, attacker.ID)
	}
	if campaign.Count != 20 {
		t.Errorf("campaign.Count = %d, want 20", campaign.Count)
	}
	if campaign.Status != "en_route" {
		t.Errorf("campaign.Status = %q, want en_route", campaign.Status)
	}
}

func TestCreateCampaignIfAvailable_InsufficientUnits(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	attacker, target := seedCampaignFixture(t, q, 10) // only 10 units

	_, err := q.CreateCampaignIfAvailable(context.Background(), db.CreateCampaignIfAvailableParams{
		KingdomID:       attacker.ID,
		TargetKingdomID: target.ID,
		UnitType:        game.UnitRecruit,
		SendCount:       50, // more than available
		Action:          "attack",
		TicksRemaining:  12,
		ActionTicks:     4,
		TravelTicks:     12,
	})
	// Query returns no rows when available < requested.
	if err == nil {
		t.Fatal("expected no rows error when units insufficient, got nil")
	}
	if err != pgx.ErrNoRows {
		t.Errorf("error = %v, want pgx.ErrNoRows", err)
	}
}

func TestCreateCampaignIfAvailable_NoUnitsAtAll(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	attacker, target := seedCampaignFixture(t, q, 0) // no units seeded

	_, err := q.CreateCampaignIfAvailable(context.Background(), db.CreateCampaignIfAvailableParams{
		KingdomID:       attacker.ID,
		TargetKingdomID: target.ID,
		UnitType:        game.UnitRecruit,
		SendCount:       1,
		Action:          "attack",
		TicksRemaining:  12,
		ActionTicks:     4,
		TravelTicks:     12,
	})
	if err != pgx.ErrNoRows {
		t.Errorf("error = %v, want pgx.ErrNoRows", err)
	}
}

func TestCancelCampaign_Success(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()
	attacker, target := seedCampaignFixture(t, q, 30)

	travelTicks := game.TravelTicks(attacker.X, attacker.Y, target.X, target.Y)
	campaign, err := q.CreateCampaignIfAvailable(ctx, db.CreateCampaignIfAvailableParams{
		KingdomID:       attacker.ID,
		TargetKingdomID: target.ID,
		UnitType:        game.UnitRecruit,
		SendCount:       10,
		Action:          "attack",
		TicksRemaining:  travelTicks,
		ActionTicks:     4,
		TravelTicks:     travelTicks,
	})
	if err != nil {
		t.Fatalf("CreateCampaignIfAvailable: %v", err)
	}

	returnedID, err := q.CancelCampaign(ctx, db.CancelCampaignParams{
		ID:        campaign.ID,
		KingdomID: attacker.ID,
	})
	if err != nil {
		t.Fatalf("CancelCampaign: %v", err)
	}
	if returnedID != campaign.ID {
		t.Errorf("CancelCampaign returned ID %d, want %d", returnedID, campaign.ID)
	}
}

func TestCancelCampaign_WrongKingdom(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()
	attacker, target := seedCampaignFixture(t, q, 30)

	travelTicks := game.TravelTicks(attacker.X, attacker.Y, target.X, target.Y)
	campaign, err := q.CreateCampaignIfAvailable(ctx, db.CreateCampaignIfAvailableParams{
		KingdomID:       attacker.ID,
		TargetKingdomID: target.ID,
		UnitType:        game.UnitRecruit,
		SendCount:       10,
		Action:          "attack",
		TicksRemaining:  travelTicks,
		ActionTicks:     4,
		TravelTicks:     travelTicks,
	})
	if err != nil {
		t.Fatalf("CreateCampaignIfAvailable: %v", err)
	}

	_, err = q.CancelCampaign(ctx, db.CancelCampaignParams{
		ID:        campaign.ID,
		KingdomID: target.ID, // wrong kingdom
	})
	if err != pgx.ErrNoRows {
		t.Errorf("error = %v, want pgx.ErrNoRows for wrong kingdom", err)
	}
}

func TestCancelCampaign_AlreadyReturning(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()
	attacker, target := seedCampaignFixture(t, q, 30)

	travelTicks := game.TravelTicks(attacker.X, attacker.Y, target.X, target.Y)
	campaign, err := q.CreateCampaignIfAvailable(ctx, db.CreateCampaignIfAvailableParams{
		KingdomID:       attacker.ID,
		TargetKingdomID: target.ID,
		UnitType:        game.UnitRecruit,
		SendCount:       10,
		Action:          "attack",
		TicksRemaining:  travelTicks,
		ActionTicks:     4,
		TravelTicks:     travelTicks,
	})
	if err != nil {
		t.Fatalf("CreateCampaignIfAvailable: %v", err)
	}

	// First cancel succeeds.
	if _, err := q.CancelCampaign(ctx, db.CancelCampaignParams{
		ID: campaign.ID, KingdomID: attacker.ID,
	}); err != nil {
		t.Fatalf("first CancelCampaign: %v", err)
	}

	// Second cancel on the same campaign (now in returning status) returns no rows.
	_, err = q.CancelCampaign(ctx, db.CancelCampaignParams{
		ID: campaign.ID, KingdomID: attacker.ID,
	})
	if err != pgx.ErrNoRows {
		t.Errorf("error = %v, want pgx.ErrNoRows for already-returning campaign", err)
	}
}
