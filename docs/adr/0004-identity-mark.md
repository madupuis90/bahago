# Identity mark: The Coin (standalone CSS frame, empty centre)

## Status

Accepted

## Context

The kingdom/brand identity mark has been a placeholder since the icon slim pass. The old heraldic shield outline + inner glyph was retired; a flat colour chip (disc + 2.5px ink ring + crown glyph) stood in as `Crest()`. The handoff "identity-mark (step 4)" was created to settle the real form.

The open questions were:

1. **Form** — medallion, cartouche, banner, or something else?
2. **Shared vs. per-kingdom** — one mark for all kingdoms, or a differentiation system?
3. **Glyph vs. frame** — does the mark live in the sprite as a `<symbol>`, or is it standalone?
4. **World-map markers** — reuse the mark at 14px, or stay as plain dots?

## Decision

### Form: The Coin

A bold BD medallion — a flat disc with a thick ink ring and flat fill. It is the most Astérix/Dofus-aligned of the three options evaluated (Coin, Pennant, Totem) and reads cleanly at every size from 54px down to 14px.

The ring is **3px solid** (`var(--crest-ring)`), the fill is flat (`var(--crest-fill)`), and the shape is a perfect circle (`border-radius: 50%`). No lumpiness or irregularity is added — at the small sizes where the mark lives, geometric precision reads better than hand-drawn imperfection.

### Shared mark

All kingdoms share the same coin. Per-kingdom differentiation (charges, monograms, colours) is a future system; this decision does not block it — the frame stays constant and the inner symbol can vary later.

### Empty centre (for now)

The inner symbol is intentionally omitted. The frame reads well without it, and the product owner has not yet chosen a symbol. When one is chosen, it slots into the coin as a child node without changing the frame CSS or the `Crest()` helper.

### Standalone, not sprite

The coin is **CSS-only** (`border`, `background`, `border-radius`). It is not a `<symbol>` in `sprite.svg` and does not use `<use>`. This is the right boundary: the sprite is for reusable monochrome glyphs that theming via `currentColor`; the coin is a single chrome element with baked-in frame geometry.

### World-map markers

Map markers adopt the same coin, degraded to 14px — essentially a bold dot with a 2.5px ink ring. The `.marker-dot` and `.kd-crest-dot` classes use the same token system (`--crest-ring` / `--crest-fill` via inline overrides) so the visual family is consistent.

## Consequences

- `Crest()` in `internal/ui/icons.go` no longer requires an inner glyph id. Passing `""` renders the empty coin.
- The three skins (`.id-crest`, `.home-chrome-crest`, `.crest-lg`) continue to override `--crest-ring` and `--crest-fill`; `--crest-glyph` remains declared but has no effect until a symbol is added.
- The sprite stays at 9 symbols; no new `<symbol>` is added for the coin.
- Adding an inner symbol later is a small change: choose or draw the symbol, add it as a child of `Crest()` when `id != ""`, and verify at 32/40/54/14px.
- The `/dev/identity` scratch page (and the `/dev/icons` audit page) remain as dev tools; they can be deleted once the inner symbol is chosen and the preview is no longer needed.
