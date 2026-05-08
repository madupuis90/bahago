package game

import (
	"context"
	"fmt"
	"log"
	"time"

	"bahago/internal/database/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ProcessTick runs one full game tick across all kingdoms.
// It fetches all kingdoms, computes fresh rates and starvation in Go, then
// bulk-updates all stockpiles in a single query.
// It also decrements active constructions and completes any that have finished.
// After a successful DB write it calls notify for each updated kingdom.
func ProcessTick(ctx context.Context, pool *pgxpool.Pool, notify func(db.Kingdom)) error {
	q := db.New(pool)

	// Record this tick in the database first so every event in this tick can
	// reference it. The returned ID is the authoritative tick number.
	tickID, err := q.InsertTick(ctx)
	if err != nil {
		return fmt.Errorf("tick: insert tick: %w", err)
	}
	log.Printf("tick #%d", tickID)

	kingdoms, err := q.ListAllKingdoms(ctx)
	if err != nil {
		return fmt.Errorf("tick: list kingdoms: %w", err)
	}
	if len(kingdoms) == 0 {
		return nil
	}

	// Fetch buildings and compute resources first, using the state at the START of
	// this tick — before any construction completes. This ensures the rates shown
	// in the UI (which reflect current buildings) match what is actually applied.
	allBuildings, err := fetchAllKingdomBuildings(ctx, q, kingdoms)
	if err != nil {
		return fmt.Errorf("tick: fetch buildings: %w", err)
	}

	prayersByCaster, prayersByTarget, err := fetchAllKingdomPrayers(ctx, q)
	if err != nil {
		return fmt.Errorf("tick: fetch prayers: %w", err)
	}

	// Cancel prayers for any caster that can no longer afford their devotion upkeep.
	// This must happen before the main rate loop so that target kingdoms don't receive
	// bonuses from prayers that are about to be deleted.
	if err := cancelUnsustainedPrayers(ctx, q, kingdoms, allBuildings, prayersByCaster, prayersByTarget); err != nil {
		return fmt.Errorf("tick: %w", err)
	}

	params := db.BulkTickKingdomsParams{
		Ids:        make([]int, len(kingdoms)),
		Wood:       make([]int, len(kingdoms)),
		Stone:      make([]int, len(kingdoms)),
		Food:       make([]int, len(kingdoms)),
		Mana:       make([]int, len(kingdoms)),
		Devotion:   make([]int, len(kingdoms)),
		Knowledge:  make([]int, len(kingdoms)),
		Population: make([]int, len(kingdoms)),
	}

	for i, k := range kingdoms {
		r := ComputeRates(k, allBuildings[k.ID], prayersByTarget[k.ID], prayersByCaster[k.ID])

		params.Ids[i] = k.ID
		params.Wood[i] = max(0, k.Wood+r.WoodProduction-r.WoodUpkeep)
		params.Stone[i] = max(0, k.Stone+r.StoneProduction-r.StoneUpkeep)
		params.Food[i] = max(0, k.Food+r.FoodProduction-r.FoodUpkeep)
		params.Mana[i] = max(0, k.Mana+r.ManaProduction-r.ManaUpkeep)
		params.Devotion[i] = max(0, k.Devotion+r.DevotionProduction-r.DevotionUpkeep)
		params.Knowledge[i] = max(0, k.Knowledge+r.KnowledgeProduction-r.KnowledgeUpkeep)
		params.Population[i] = max(100, k.Population+r.PopulationProduction-r.PopulationUpkeep)
	}

	if err := q.BulkTickKingdoms(ctx, params); err != nil {
		return fmt.Errorf("tick: bulk update: %w", err)
	}

	// Complete constructions after resource allocation — buildings that finish
	// this tick contribute their bonus starting next tick.
	completed, err := q.DecrementAndListConstructionAtZero(ctx)
	if err != nil {
		return fmt.Errorf("tick: decrement constructions: %w", err)
	}
	for _, c := range completed {
		if err := completeConstruction(ctx, pool, c); err != nil {
			return fmt.Errorf("tick: complete construction for kingdom %d: %w", c.KingdomID, err)
		}
	}

	// Complete training batches — units arrive at the end of the tick they finish.
	completedTraining, err := q.DecrementAndListTrainingAtZero(ctx)
	if err != nil {
		return fmt.Errorf("tick: decrement training: %w", err)
	}
	for _, t := range completedTraining {
		if err := completeTraining(ctx, pool, t); err != nil {
			return fmt.Errorf("tick: complete training for kingdom %d: %w", t.KingdomID, err)
		}
	}

	// Advance campaign states (movement, status transitions).
	if err := AdvanceCampaigns(ctx, q); err != nil {
		return fmt.Errorf("tick: advance campaigns: %w", err)
	}

	// Expire pending guilds that have not reached 5 supporters within 7 days.
	// Single bulk DELETE — no per-row work needed.
	if err := q.ExpirePendingGuilds(ctx); err != nil {
		return fmt.Errorf("tick: expire pending guilds: %w", err)
	}

	// Resolve combat for all active campaigns on a 4-tick boundary.
	// AdvanceCampaigns runs first, so freshly activated campaigns (ticks_remaining = action_ticks)
	// fire their first combat round on the same tick they arrive. This is intentional:
	// arrival and first strike are simultaneous.
	if err := ResolveCombat(ctx, pool, q, tickID); err != nil {
		return fmt.Errorf("tick: resolve combat: %w", err)
	}

	// Expire prayers whose countdown has reached zero.
	// The CTE in DecrementAndListPrayersAtZero decrements and deletes in one round trip.
	if _, err := q.DecrementAndListPrayersAtZero(ctx); err != nil {
		return fmt.Errorf("tick: decrement prayers: %w", err)
	}

	// Single authoritative notify — reads post-everything state from DB so
	// values reflect resources, completed constructions/training, and combat.
	finalKingdoms, err := q.ListAllKingdoms(ctx)
	if err != nil {
		return fmt.Errorf("tick: final notify fetch: %w", err)
	}
	for _, k := range finalKingdoms {
		notify(k)
	}
	return nil
}

// completeConstruction atomically increments the building count and removes the
// construction row. Using a transaction prevents a double-increment if the process
// dies between the two writes — the row would otherwise remain at ticks_remaining=0
// and be picked up again on the next tick.
func completeConstruction(ctx context.Context, pool *pgxpool.Pool, c db.DecrementAndListConstructionAtZeroRow) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	txq := db.New(tx)
	params := db.IncrementKingdomBuildingParams{
		KingdomID:    c.KingdomID,
		BuildingType: c.BuildingType,
	}
	if err := txq.IncrementKingdomBuilding(ctx, params); err != nil {
		return fmt.Errorf("increment building %s: %w", c.BuildingType, err)
	}
	if err := txq.DeleteConstruction(ctx, c.KingdomID); err != nil {
		return fmt.Errorf("delete construction: %w", err)
	}
	return tx.Commit(ctx)
}

// completeTraining atomically adds the trained units and removes the training row.
func completeTraining(ctx context.Context, pool *pgxpool.Pool, t db.DecrementAndListTrainingAtZeroRow) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	txq := db.New(tx)
	if err := txq.UpsertKingdomUnits(ctx, db.UpsertKingdomUnitsParams{
		KingdomID: t.KingdomID,
		UnitType:  t.UnitType,
		Count:     t.Count,
	}); err != nil {
		return fmt.Errorf("upsert units %s: %w", t.UnitType, err)
	}
	if err := txq.DeleteTraining(ctx, t.KingdomID); err != nil {
		return fmt.Errorf("delete training: %w", err)
	}
	return tx.Commit(ctx)
}

// cancelUnsustainedPrayers cancels prayers for any caster that cannot sustain their total
// devotion upkeep this tick. A caster is considered able to sustain if their current devotion
// stockpile plus their devotion production this tick covers the total upkeep — i.e. they would
// not go negative even starting from zero stock. This means a kingdom with 0 devotion but
// sufficient production keeps its prayers active.
// Both maps are modified in place (Go maps are reference types).
func cancelUnsustainedPrayers(
	ctx context.Context,
	q db.Querier,
	kingdoms []db.Kingdom,
	allBuildings map[int][]db.KingdomBuilding,
	prayersByCaster, prayersByTarget map[int][]db.KingdomPrayer,
) error {
	var cancelIDs []int
	for _, k := range kingdoms {
		castPrayers := prayersByCaster[k.ID]
		if len(castPrayers) == 0 {
			continue
		}
		bonus := ComputeBonuses(allBuildings[k.ID], prayersByTarget[k.ID])
		devProd := devotionProduction(k.Population, k.DevotionPct, bonus.Devotion)
		if k.Devotion+devProd < devotionUpkeep(castPrayers) {
			cancelIDs = append(cancelIDs, k.ID)
			delete(prayersByCaster, k.ID)
			for _, p := range castPrayers {
				targeted := prayersByTarget[p.TargetKingdomID]
				var kept []db.KingdomPrayer
				for _, tp := range targeted {
					if tp.KingdomID == k.ID {
						continue
					}
					kept = append(kept, tp)
				}
				if len(kept) == 0 {
					delete(prayersByTarget, p.TargetKingdomID)
				} else {
					prayersByTarget[p.TargetKingdomID] = kept
				}
			}
		}
	}
	if len(cancelIDs) == 0 {
		return nil
	}
	if err := q.DeleteKingdomPrayers(ctx, cancelIDs); err != nil {
		return fmt.Errorf("cancel prayers on devotion failure: %w", err)
	}
	return nil
}

// fetchAllKingdomBuildings returns a map from kingdom ID to its buildings.
// Uses a single query for all kingdoms to avoid N+1 queries during the tick.
func fetchAllKingdomBuildings(ctx context.Context, q db.Querier, kingdoms []db.Kingdom) (map[int][]db.KingdomBuilding, error) {
	all, err := q.GetAllKingdomBuildings(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[int][]db.KingdomBuilding, len(kingdoms))
	for _, b := range all {
		m[b.KingdomID] = append(m[b.KingdomID], b)
	}
	return m, nil
}

// fetchAllKingdomPrayers returns two maps built from a single query:
//   - byCaster: keyed by kingdom_id (the praying kingdom) — used for devotion upkeep
//   - byTarget: keyed by target_kingdom_id (the receiving kingdom) — used for resource bonuses
//
// Keeping them separate ensures that when caster ≠ target the costs and bonuses land on the
// correct kingdoms. For self-targeted prayers (the current default) both maps hold the same rows.
func fetchAllKingdomPrayers(ctx context.Context, q db.Querier) (byCaster, byTarget map[int][]db.KingdomPrayer, err error) {
	all, err := q.GetAllKingdomPrayers(ctx)
	if err != nil {
		return nil, nil, err
	}
	byCaster = make(map[int][]db.KingdomPrayer)
	byTarget = make(map[int][]db.KingdomPrayer)
	for _, p := range all {
		byCaster[p.KingdomID] = append(byCaster[p.KingdomID], p)
		byTarget[p.TargetKingdomID] = append(byTarget[p.TargetKingdomID], p)
	}
	return byCaster, byTarget, nil
}

// StartTicker starts the game tick loop. It blocks until ctx is cancelled.
// Call as a goroutine from main.
func StartTicker(ctx context.Context, pool *pgxpool.Pool, notify func(db.Kingdom), interval time.Duration) {
	q := db.New(pool)

	// Restore tick counter from DB so logging is continuous across restarts.
	latestTickID, err := q.GetLatestTickID(ctx)
	if err != nil {
		// No rows means the game is starting fresh — begin at tick 0.
		latestTickID = 0
	}
	log.Printf("game ticker started at tick %d (interval: %s)", latestTickID, interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := ProcessTick(ctx, pool, notify); err != nil {
				log.Printf("tick error: %v", err)
			}
		case <-ctx.Done():
			log.Println("game ticker stopped")
			return
		}
	}
}
