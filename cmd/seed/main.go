// Seeder populates the database with fake users and kingdoms for local development.
// Run via: task seed
// Run via: task seed -- --count=500
//
// Each seeded user gets:
//   - email: seedN@dev.local (e.g. seed1@dev.local)
//   - password: "password" (bcrypt hashed)
//   - a verified, active account
//   - one kingdom named "KingdomN"
//
// Positions are drawn from the same normal distribution used in production
// (mean=WorldSize/2, sigma grows with sqrt(population)) with a collision set
// to guarantee no two kingdoms share a tile.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"os"

	"bahago/internal/database/db"
	"bahago/internal/game"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	count := flag.Int("count", 100, "number of kingdoms to seed")
	flag.Parse()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	queries := db.New(pool)

	// Pre-hash once — bcrypt is intentionally slow so we reuse the same hash
	// for all seed users (they all share the same dev password).
	pwHash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("bcrypt hash: %v", err)
	}

	existing, err := queries.GetKingdomsInViewport(ctx, db.GetKingdomsInViewportParams{
		X:   0,
		X_2: game.WorldSize - 1,
		Y:   0,
		Y_2: game.WorldSize - 1,
	})
	if err != nil {
		log.Fatalf("load existing kingdoms: %v", err)
	}
	occupied := make(map[game.Coord]struct{}, len(existing)+*count)
	for _, k := range existing {
		occupied[game.Coord{X: k.X, Y: k.Y}] = struct{}{}
	}
	log.Printf("found %d existing kingdoms, seeding %d more", len(existing), *count)

	for i := range *count {
		n := len(existing) + i + 1
		email := fmt.Sprintf("seed%d@dev.local", n)
		name := fmt.Sprintf("Kingdom%d", n)

		userID, err := queries.CreateUser(ctx, db.CreateUserParams{
			Email:  email,
			PwHash: string(pwHash),
		})
		if err != nil {
			log.Fatalf("create user %d (%s): %v", n, email, err)
		}

		if err := queries.VerifyUser(ctx, userID); err != nil {
			log.Fatalf("verify user %d: %v", n, err)
		}

		x, y := pickPosition(occupied)
		occupied[game.Coord{X: x, Y: y}] = struct{}{}

		if _, err := queries.CreateKingdom(ctx, db.CreateKingdomParams{
			UserID: userID,
			Name:   name,
			X:      x,
			Y:      y,
		}); err != nil {
			log.Fatalf("create kingdom %d: %v", n, err)
		}

		if (i+1)%10 == 0 {
			log.Printf("seeded %d/%d kingdoms", i+1, *count)
		}
	}

	log.Printf("done: seeded %d kingdoms", *count)
}

// pickPosition draws a tile coordinate from a normal distribution centred on
// the map, with sigma growing as more kingdoms are placed (mirrors production).
func pickPosition(occupied map[game.Coord]struct{}) (int, int) {
	const centre = float64(game.WorldSize) / 2

	sigma := math.Max(5.0, math.Sqrt(float64(len(occupied)))*0.6)

	clamp := func(v float64) int {
		n := int(math.Round(centre + sigma*v))
		if n < 0 {
			return 0
		}
		if n > game.WorldSize-1 {
			return game.WorldSize - 1
		}
		return n
	}

	for range 10_000 {
		x, y := clamp(rand.NormFloat64()), clamp(rand.NormFloat64())
		if _, ok := occupied[game.Coord{X: x, Y: y}]; !ok {
			return x, y
		}
	}

	// Fallback: scan sequentially for any free tile.
	for x := range game.WorldSize {
		for y := range game.WorldSize {
			if _, ok := occupied[game.Coord{X: x, Y: y}]; !ok {
				return x, y
			}
		}
	}

	log.Fatal("world map is full, cannot place more kingdoms")
	return 0, 0
}
