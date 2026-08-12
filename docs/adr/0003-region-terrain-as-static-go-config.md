# Region terrain stored as static Go config, not a DB table

The world's per-Region terrain (name, dominant Biome, per-tile Biome layout) is
authored as a static Go table in `internal/game/region_defs.go`, directly
parallel to `unit_defs.go` and `building_defs.go` — not as a `regions`/`tiles`
database table. Biome is cosmetic today: no query, tick, or combat mechanic reads
it. This keeps authoring reviewable in a diff, matches the existing
static-config precedent, and avoids a migration for content no mechanic depends
on.

The trade-off versus a DB table is queryability: SQL cannot ask "how many
Kingdoms sit in Mountain regions" because the data lives in Go, not Postgres.
We accept that loss now and promote to a DB table later — mechanically, by
swapping the `BiomeAt(x,y)` lookup for a query — if and when a gameplay
mechanic (movement cost, yield, combat bonus) needs server-side access to
biome.