package db_test

import (
	"context"
	"testing"

	"bahago/internal/database/db"
	"bahago/internal/game"
	"bahago/internal/testhelper"
)

// seedKingdomPair creates two users and two kingdoms for legion tests.
func seedKingdomPair(t *testing.T, q db.Querier) (k1, k2 db.Kingdom) {
	t.Helper()
	ctx := context.Background()

	u1, err := q.CreateUser(ctx, db.CreateUserParams{Email: "leg1@example.com", PwHash: "h"})
	if err != nil {
		t.Fatalf("seedKingdomPair: create user 1: %v", err)
	}
	u2, err := q.CreateUser(ctx, db.CreateUserParams{Email: "leg2@example.com", PwHash: "h"})
	if err != nil {
		t.Fatalf("seedKingdomPair: create user 2: %v", err)
	}
	k1, err = q.CreateKingdom(ctx, db.CreateKingdomParams{UserID: u1, Name: "LegK1", X: 0, Y: 0})
	if err != nil {
		t.Fatalf("seedKingdomPair: create kingdom 1: %v", err)
	}
	k2, err = q.CreateKingdom(ctx, db.CreateKingdomParams{UserID: u2, Name: "LegK2", X: 10, Y: 10})
	if err != nil {
		t.Fatalf("seedKingdomPair: create kingdom 2: %v", err)
	}
	return k1, k2
}

func TestCreateLegion_AutoNames(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()
	k, _ := seedKingdomPair(t, q)

	for i := range game.MaxLegionsPerKingdom {
		legion, err := q.CreateLegion(ctx, db.CreateLegionParams{
			KingdomID: k.ID,
			Cap:       game.MaxLegionsPerKingdom,
		})
		if err != nil {
			t.Fatalf("CreateLegion %d: %v", i+1, err)
		}
		want := "Legion " + string(rune('1'+i))
		if legion.Name != want {
			t.Errorf("legion %d name = %q, want %q", i+1, legion.Name, want)
		}
		if legion.Number != i+1 {
			t.Errorf("legion %d number = %d, want %d", i+1, legion.Number, i+1)
		}
	}
}

func TestCreateLegion_CapEnforced(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()
	k, _ := seedKingdomPair(t, q)

	for range game.MaxLegionsPerKingdom {
		if _, err := q.CreateLegion(ctx, db.CreateLegionParams{
			KingdomID: k.ID,
			Cap:       game.MaxLegionsPerKingdom,
		}); err != nil {
			t.Fatalf("CreateLegion (filling cap): %v", err)
		}
	}

	// One over the cap — query returns no rows.
	_, err := q.CreateLegion(ctx, db.CreateLegionParams{
		KingdomID: k.ID,
		Cap:       game.MaxLegionsPerKingdom,
	})
	if err == nil {
		t.Fatal("CreateLegion beyond cap: expected error, got nil")
	}
}

func TestCreateLegion_ReclaimsDeletedSlot(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()
	k, _ := seedKingdomPair(t, q)

	l1, err := q.CreateLegion(ctx, db.CreateLegionParams{KingdomID: k.ID, Cap: game.MaxLegionsPerKingdom})
	if err != nil {
		t.Fatalf("CreateLegion 1: %v", err)
	}
	l2, err := q.CreateLegion(ctx, db.CreateLegionParams{KingdomID: k.ID, Cap: game.MaxLegionsPerKingdom})
	if err != nil {
		t.Fatalf("CreateLegion 2: %v", err)
	}

	// Delete the first legion (number=1).
	if err := q.DeleteLegion(ctx, db.DeleteLegionParams{ID: l1.ID, KingdomID: k.ID}); err != nil {
		t.Fatalf("DeleteLegion 1: %v", err)
	}

	// Next creation should reclaim number=1, not use 3.
	l3, err := q.CreateLegion(ctx, db.CreateLegionParams{KingdomID: k.ID, Cap: game.MaxLegionsPerKingdom})
	if err != nil {
		t.Fatalf("CreateLegion after delete: %v", err)
	}
	if l3.Number != 1 {
		t.Errorf("reclaimed legion number = %d, want 1", l3.Number)
	}
	_ = l2
}

func TestUpsertLegionUnit_AddsAndAccumulates(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()
	k, _ := seedKingdomPair(t, q)

	legion, err := q.CreateLegion(ctx, db.CreateLegionParams{KingdomID: k.ID, Cap: game.MaxLegionsPerKingdom})
	if err != nil {
		t.Fatalf("CreateLegion: %v", err)
	}

	params := db.UpsertLegionUnitParams{LegionID: legion.ID, UnitType: game.UnitRecruit, Count: 10}
	if err := q.UpsertLegionUnit(ctx, params); err != nil {
		t.Fatalf("UpsertLegionUnit (first): %v", err)
	}
	// Upsert adds to the existing count.
	if err := q.UpsertLegionUnit(ctx, params); err != nil {
		t.Fatalf("UpsertLegionUnit (second): %v", err)
	}

	units, err := q.ListLegionUnits(ctx, legion.ID)
	if err != nil {
		t.Fatalf("ListLegionUnits: %v", err)
	}
	if len(units) != 1 {
		t.Fatalf("len(units) = %d, want 1", len(units))
	}
	if units[0].Count != 20 {
		t.Errorf("count = %d, want 20", units[0].Count)
	}
}

func TestDecrementLegionUnit_DeletesAtZero(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()
	k, _ := seedKingdomPair(t, q)

	legion, err := q.CreateLegion(ctx, db.CreateLegionParams{KingdomID: k.ID, Cap: game.MaxLegionsPerKingdom})
	if err != nil {
		t.Fatalf("CreateLegion: %v", err)
	}

	if err := q.UpsertLegionUnit(ctx, db.UpsertLegionUnitParams{
		LegionID: legion.ID, UnitType: game.UnitRecruit, Count: 5,
	}); err != nil {
		t.Fatalf("UpsertLegionUnit: %v", err)
	}

	// Decrement partially — row should survive.
	if err := q.DecrementLegionUnit(ctx, db.DecrementLegionUnitParams{
		Amount: 3, LegionID: legion.ID, UnitType: game.UnitRecruit,
	}); err != nil {
		t.Fatalf("DecrementLegionUnit (partial): %v", err)
	}
	units, err := q.ListLegionUnits(ctx, legion.ID)
	if err != nil {
		t.Fatalf("ListLegionUnits after partial: %v", err)
	}
	if len(units) != 1 || units[0].Count != 2 {
		t.Errorf("after partial decrement: units = %v, want [{count:2}]", units)
	}

	// Decrement to zero — row should be deleted.
	if err := q.DecrementLegionUnit(ctx, db.DecrementLegionUnitParams{
		Amount: 2, LegionID: legion.ID, UnitType: game.UnitRecruit,
	}); err != nil {
		t.Fatalf("DecrementLegionUnit (to zero): %v", err)
	}
	units, err = q.ListLegionUnits(ctx, legion.ID)
	if err != nil {
		t.Fatalf("ListLegionUnits after zero: %v", err)
	}
	if len(units) != 0 {
		t.Errorf("after decrement to zero: units = %v, want []", units)
	}
}

func TestClearLegionUnits(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()
	k, _ := seedKingdomPair(t, q)

	legion, err := q.CreateLegion(ctx, db.CreateLegionParams{KingdomID: k.ID, Cap: game.MaxLegionsPerKingdom})
	if err != nil {
		t.Fatalf("CreateLegion: %v", err)
	}

	for _, ut := range []string{game.UnitRecruit, game.UnitArcher} {
		if err := q.UpsertLegionUnit(ctx, db.UpsertLegionUnitParams{
			LegionID: legion.ID, UnitType: ut, Count: 10,
		}); err != nil {
			t.Fatalf("UpsertLegionUnit %s: %v", ut, err)
		}
	}

	if err := q.ClearLegionUnits(ctx, legion.ID); err != nil {
		t.Fatalf("ClearLegionUnits: %v", err)
	}

	units, err := q.ListLegionUnits(ctx, legion.ID)
	if err != nil {
		t.Fatalf("ListLegionUnits after clear: %v", err)
	}
	if len(units) != 0 {
		t.Errorf("after ClearLegionUnits: %d rows remain, want 0", len(units))
	}
}

func TestListLegionsForKingdom_ShowsCampaignStatus(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()
	k1, k2 := seedKingdomPair(t, q)

	legion, err := q.CreateLegion(ctx, db.CreateLegionParams{KingdomID: k1.ID, Cap: game.MaxLegionsPerKingdom})
	if err != nil {
		t.Fatalf("CreateLegion: %v", err)
	}
	if err := q.UpsertLegionUnit(ctx, db.UpsertLegionUnitParams{
		LegionID: legion.ID, UnitType: game.UnitRecruit, Count: 20,
	}); err != nil {
		t.Fatalf("UpsertLegionUnit: %v", err)
	}

	// Before campaign — CampaignStatus should be invalid (NULL).
	rows, err := q.ListLegionsForKingdom(ctx, k1.ID)
	if err != nil {
		t.Fatalf("ListLegionsForKingdom (before campaign): %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].CampaignStatus.Valid {
		t.Errorf("CampaignStatus.Valid = true before campaign, want false")
	}

	// Create a campaign for the legion.
	travelTicks := game.TravelTicks(k1.X, k1.Y, k2.X, k2.Y)
	campaign, err := q.CreateCampaign(ctx, db.CreateCampaignParams{
		KingdomID:       k1.ID,
		TargetKingdomID: k2.ID,
		LegionID:        legion.ID,
		Action:          "attack",
		TicksRemaining:  travelTicks,
		ActionTicks:     4,
		TravelTicks:     travelTicks,
	})
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	_ = campaign

	// After campaign — CampaignStatus should be "en_route".
	rows, err = q.ListLegionsForKingdom(ctx, k1.ID)
	if err != nil {
		t.Fatalf("ListLegionsForKingdom (after campaign): %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if !rows[0].CampaignStatus.Valid || rows[0].CampaignStatus.String != "en_route" {
		t.Errorf("CampaignStatus = %v, want {Valid:true String:en_route}", rows[0].CampaignStatus)
	}
}

func TestListAllLegionUnitsForKingdom(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()
	k, _ := seedKingdomPair(t, q)

	l1, err := q.CreateLegion(ctx, db.CreateLegionParams{KingdomID: k.ID, Cap: game.MaxLegionsPerKingdom})
	if err != nil {
		t.Fatalf("CreateLegion 1: %v", err)
	}
	l2, err := q.CreateLegion(ctx, db.CreateLegionParams{KingdomID: k.ID, Cap: game.MaxLegionsPerKingdom})
	if err != nil {
		t.Fatalf("CreateLegion 2: %v", err)
	}

	if err := q.UpsertLegionUnit(ctx, db.UpsertLegionUnitParams{LegionID: l1.ID, UnitType: game.UnitRecruit, Count: 5}); err != nil {
		t.Fatalf("UpsertLegionUnit l1 recruit: %v", err)
	}
	if err := q.UpsertLegionUnit(ctx, db.UpsertLegionUnitParams{LegionID: l2.ID, UnitType: game.UnitArcher, Count: 3}); err != nil {
		t.Fatalf("UpsertLegionUnit l2 archer: %v", err)
	}

	units, err := q.ListAllLegionUnitsForKingdom(ctx, k.ID)
	if err != nil {
		t.Fatalf("ListAllLegionUnitsForKingdom: %v", err)
	}
	if len(units) != 2 {
		t.Fatalf("len = %d, want 2", len(units))
	}
	// Results ordered by legion_id, unit_type — l1/recruit comes first.
	if units[0].LegionID != l1.ID || units[0].UnitType != game.UnitRecruit || units[0].Count != 5 {
		t.Errorf("units[0] = %+v, want {LegionID:%d UnitType:recruit Count:5}", units[0], l1.ID)
	}
	if units[1].LegionID != l2.ID || units[1].UnitType != game.UnitArcher || units[1].Count != 3 {
		t.Errorf("units[1] = %+v, want {LegionID:%d UnitType:archer Count:3}", units[1], l2.ID)
	}
}

func TestDeleteLegion_CascadesUnits(t *testing.T) {
	q := testhelper.WithRollback(t, testPool)
	ctx := context.Background()
	k, _ := seedKingdomPair(t, q)

	legion, err := q.CreateLegion(ctx, db.CreateLegionParams{KingdomID: k.ID, Cap: game.MaxLegionsPerKingdom})
	if err != nil {
		t.Fatalf("CreateLegion: %v", err)
	}
	if err := q.UpsertLegionUnit(ctx, db.UpsertLegionUnitParams{
		LegionID: legion.ID, UnitType: game.UnitRecruit, Count: 10,
	}); err != nil {
		t.Fatalf("UpsertLegionUnit: %v", err)
	}

	if err := q.DeleteLegion(ctx, db.DeleteLegionParams{ID: legion.ID, KingdomID: k.ID}); err != nil {
		t.Fatalf("DeleteLegion: %v", err)
	}

	units, err := q.ListLegionUnits(ctx, legion.ID)
	if err != nil {
		t.Fatalf("ListLegionUnits after delete: %v", err)
	}
	if len(units) != 0 {
		t.Errorf("units after DeleteLegion = %v, want [] (CASCADE expected)", units)
	}
}
