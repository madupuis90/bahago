package game

import "testing"

// TestRegionMainBiomeDominant guards the minimap/board coherence contract: a
// Region's stored MainBiome (which drives the minimap colour) must be the most
// frequent biome in its Tiles (which drive the board's per-tile fills).
// Without this, a page could be themed "Mountain" on the minimap while its
// board showed mostly Forest tiles. The per-tile layout rule keeps MainBiome
// the single most frequent biome (roughly 50%+), enough for the minimap to
// read correctly while letting each page show organic transitions.
func TestRegionMainBiomeDominant(t *testing.T) {
	for py := range PageCount {
		for px := range PageCount {
			r := RegionDefs[py][px]
			counts := make(map[Biome]int, 5)
			for _, b := range r.Tiles {
				counts[b]++
			}
			var dominant Biome
			dominantCount := -1
			for b, c := range counts {
				if c > dominantCount || (c == dominantCount && string(b) < string(dominant)) {
					dominant = b
					dominantCount = c
				}
			}
			if dominant != r.MainBiome {
				t.Errorf("region [%d][%d] %q: MainBiome=%s but dominant tile biome is %s (counts=%v)",
					py, px, r.Name, r.MainBiome, dominant, counts)
			}
		}
	}
}

// TestRegionTilesFull ensures every RegionDef authorits exactly 64 tiles.
func TestRegionTilesFull(t *testing.T) {
	for py := range PageCount {
		for px := range PageCount {
			r := RegionDefs[py][px]
			if r.Name == "" {
				t.Errorf("region [%d][%d] has empty Name", py, px)
			}
			for i, b := range r.Tiles {
				switch b {
				case BiomePlains, BiomeForest, BiomeWater, BiomeMountain, BiomeMarsh:
				default:
					t.Errorf("region [%d][%d] %q tile %d has unknown biome %q", py, px, r.Name, i, b)
				}
			}
		}
	}
}

// TestBiomeAtConsistency verifies BiomeAt agrees with the RegionDefs literal
// arrays for every world tile.
func TestBiomeAtConsistency(t *testing.T) {
	for y := 0; y < WorldSize; y++ {
		for x := 0; x < WorldSize; x++ {
			got := BiomeAt(x, y)
			want := RegionDefs[y/PageSize][x/PageSize].Tiles[(y%PageSize)*PageSize+x%PageSize]
			if got != want {
				t.Errorf("BiomeAt(%d,%d)=%s want %s", x, y, got, want)
			}
		}
	}
}

// TestRegionAt verifies RegionAt resolves to the page containing the tile.
func TestRegionAt(t *testing.T) {
	for py := range PageCount {
		for px := range PageCount {
			want := RegionDefs[py][px]
			// sample the page's north-west corner tile
			got := RegionAt(px*PageSize, py*PageSize)
			if got.Name != want.Name {
				t.Errorf("RegionAt(%d,%d)=%q want %q", px*PageSize, py*PageSize, got.Name, want.Name)
			}
		}
	}
}
