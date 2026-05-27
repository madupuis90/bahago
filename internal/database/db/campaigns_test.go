package db_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"bahago/internal/database/db"
	"bahago/internal/game"
	"bahago/internal/testhelper"
)

// seedCampaignFixture creates two kingdoms (attacker + target) with a legion
// for the attacker containing the specified number of recruits.
func seedCampaignFixture(t *testing.T, q db.Querier, attackerUnits int) (attacker, target db.Kingdom, legion db.KingdomLegion) {
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

	legion, err = q.CreateLegion(ctx, db.CreateLegionParams{
		KingdomID: attacker.ID,
		Cap:       game.MaxLegionsPerKingdom,
	})
	if err != nil {
		t.Fatalf("seedCampaignFixture: create legion: %v", err)
	}

	if attackerUnits > 0 {
		if err := q.UpsertLegionUnit(ctx, db.UpsertLegionUnitParams{
			LegionID: legion.ID,
			UnitType: game.UnitRecruit,
			Count:    attackerUnits,
		}); err != nil {
			t.Fatalf("seedCampaignFixture: upsert legion unit: %v", err)
		}
	}

	return attacker, target, legion
}

func TestCreateCampaign_Success(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()
	attacker, target, legion := seedCampaignFixture(t, q, 50)

	travelTicks := game.TravelTicks(attacker.X, attacker.Y, target.X, target.Y)
	params := db.CreateCampaignParams{
		KingdomID:       attacker.ID,
		TargetKingdomID: target.ID,
		LegionID:        legion.ID,
		Action:          "attack",
		TicksRemaining:  travelTicks,
		ActionTicks:     4,
		TravelTicks:     travelTicks,
	}
	campaign, err := q.CreateCampaign(ctx, params)
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	if campaign.KingdomID != attacker.ID {
		t.Errorf("campaign.KingdomID = %d, want %d", campaign.KingdomID, attacker.ID)
	}
	if campaign.LegionID != legion.ID {
		t.Errorf("campaign.LegionID = %d, want %d", campaign.LegionID, legion.ID)
	}
	if campaign.Status != "en_route" {
		t.Errorf("campaign.Status = %q, want en_route", campaign.Status)
	}

	snapParams := db.SnapshotLegionUnitsIntoCampaignParams{
		CampaignID: campaign.ID,
		LegionID:   legion.ID,
	}
	if err := q.SnapshotLegionUnitsIntoCampaign(ctx, snapParams); err != nil {
		t.Fatalf("SnapshotLegionUnitsIntoCampaign: %v", err)
	}
	if err := q.ClearLegionUnits(ctx, legion.ID); err != nil {
		t.Fatalf("ClearLegionUnits: %v", err)
	}
}

func TestCancelCampaign_Success(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()
	attacker, target, legion := seedCampaignFixture(t, q, 30)

	travelTicks := game.TravelTicks(attacker.X, attacker.Y, target.X, target.Y)
	params := db.CreateCampaignParams{
		KingdomID:       attacker.ID,
		TargetKingdomID: target.ID,
		LegionID:        legion.ID,
		Action:          "attack",
		TicksRemaining:  travelTicks,
		ActionTicks:     4,
		TravelTicks:     travelTicks,
	}
	campaign, err := q.CreateCampaign(ctx, params)
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	snapParams := db.SnapshotLegionUnitsIntoCampaignParams{
		CampaignID: campaign.ID,
		LegionID:   legion.ID,
	}
	if err := q.SnapshotLegionUnitsIntoCampaign(ctx, snapParams); err != nil {
		t.Fatalf("SnapshotLegionUnitsIntoCampaign: %v", err)
	}
	if err := q.ClearLegionUnits(ctx, legion.ID); err != nil {
		t.Fatalf("ClearLegionUnits: %v", err)
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
	attacker, target, legion := seedCampaignFixture(t, q, 30)

	travelTicks := game.TravelTicks(attacker.X, attacker.Y, target.X, target.Y)
	params := db.CreateCampaignParams{
		KingdomID:       attacker.ID,
		TargetKingdomID: target.ID,
		LegionID:        legion.ID,
		Action:          "attack",
		TicksRemaining:  travelTicks,
		ActionTicks:     4,
		TravelTicks:     travelTicks,
	}
	campaign, err := q.CreateCampaign(ctx, params)
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
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
	attacker, target, legion := seedCampaignFixture(t, q, 30)

	travelTicks := game.TravelTicks(attacker.X, attacker.Y, target.X, target.Y)
	params := db.CreateCampaignParams{
		KingdomID:       attacker.ID,
		TargetKingdomID: target.ID,
		LegionID:        legion.ID,
		Action:          "attack",
		TicksRemaining:  travelTicks,
		ActionTicks:     4,
		TravelTicks:     travelTicks,
	}
	campaign, err := q.CreateCampaign(ctx, params)
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}

	// First cancel succeeds — campaign moves to returning.
	if _, err := q.CancelCampaign(ctx, db.CancelCampaignParams{
		ID: campaign.ID, KingdomID: attacker.ID,
	}); err != nil {
		t.Fatalf("first CancelCampaign: %v", err)
	}

	// Second cancel on the same campaign (now returning) returns no rows.
	_, err = q.CancelCampaign(ctx, db.CancelCampaignParams{
		ID: campaign.ID, KingdomID: attacker.ID,
	})
	if err != pgx.ErrNoRows {
		t.Errorf("error = %v, want pgx.ErrNoRows for already-returning campaign", err)
	}
}
