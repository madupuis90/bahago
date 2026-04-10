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
func ProcessTick(ctx context.Context, pool *pgxpool.Pool) error {
	q := db.New(pool)

	kingdoms, err := q.ListAllKingdoms(ctx)
	if err != nil {
		return fmt.Errorf("tick: list kingdoms: %w", err)
	}
	if len(kingdoms) == 0 {
		return nil
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
		r := ComputeRates(k)

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

	return nil
}

// StartTicker starts the game tick loop. It blocks until ctx is cancelled.
// Call as a goroutine from main.
func StartTicker(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("game ticker started (interval: %s)", interval)
	counter := 0
	for {
		select {
		case <-ticker.C:
			counter++
			log.Printf("tick #%d", counter)
			if err := ProcessTick(ctx, pool); err != nil {
				log.Printf("tick error: %v", err)
			}
		case <-ctx.Done():
			log.Println("game ticker stopped")
			return
		}
	}
}
