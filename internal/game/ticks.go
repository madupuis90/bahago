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
		r := ComputeRates(k, allBuildings[k.ID])

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
