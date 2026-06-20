# Icon Language

The detailed spec for sprite artwork. Governs the sprite redraw (and any future icon). Must conform to `docs/design-system/design-language.md` (bande dessinée, bold ink, flat fills, fantasy subject) and `docs/adr/0003-sprite-icon-system.md` (sprite + `<use>` + `currentColor` mechanism, which does not change).

Read this before touching the sprite or icon helpers.

## The core decision: standardize on what the resource art already does

The six `res-*` symbols (`res-wood`, `res-stone`, `res-food`, `res-mana`, `res-devotion`, `res-knowledge`) are already drawn in the target language: **24×24 viewBox, `stroke-width="2"`, round caps/joins, `currentColor`, no fill**. They are the reference. The heraldic glyphs (the bare `crown`, `sword`, etc., formerly `shield-*`) lag at 0.65–1.4px in a 20×23 space — that is the engraving/ligne-claire weight this spec retires.

**Every glyph in the sprite targets the resource art's language.** No two visual weights, no two coordinate spaces.

## Geometry

| Property | Value | Why |
|---|---|---|
| viewBox | `0 0 24 24` for every glyph | One aspect ratio across the sprite; `Icon(id, size)` derives a square height for all of them; no per-category sizing surprises. The old 20×23 shield space is retired. |
| Stroke width | `2` (the `res-*` weight) | Bold enough to read as BD ink at 16–48px render sizes; matches the existing resource art so the sprite is one weight. |
| Stroke caps | `round` | Cartoon-friendly; the BD default. The old glyphs mixed `square` and `round` — pick one, and round is it. |
| Stroke joins | `round` | Same reasoning. |
| Fill | `none` on the glyph itself | See "Colour & theming" below — glyphs stay monochrome; colour is applied around them, not baked in. |
| Inset | keep strokes inside the 24×24 box (account for the 1px either side that a 2px round stroke needs) | Prevents clipping when the `<use>` is scaled down. The `res-*` art already respects this. |

The `sandglass` is the one existing non-24×24 symbol (10×14). Redraw it at 24×24 too, so `Icon(id, sizePx)` can treat every symbol as square and drop the per-symbol aspect-ratio lookup. (If a genuinely non-square symbol is ever needed later, the `Icon` helper already derives height from the viewBox — but the target is square-everywhere.)

## Colour & theming (resolves the BD-fill vs currentColor tension)

BD art is flat fills inside a bold black outline. The sprite's theming mechanism is `currentColor`. These seem to conflict — they don't, once you separate **glyph** from **chip**:

- **The glyph is monochrome line art** (`fill: none`, `stroke: currentColor`). It inherits colour from CSS. This is what makes `.icon--active { color: var(--red) }`, `.gem .icon { color: #fff }`, and dim states each a one-line CSS rule. Do not bake flat fills into glyph paths.
- **Flat BD colour is delivered by the chip behind the glyph** — the established `.gem` pattern: a flat `--gem-*` disc with a bold ink ring and a white glyph on top. This is already the BD model in the codebase. Extend it, don't replace it.
- **State colour** (active/dim/meaning-colour) is applied via `currentColor` on the glyph's container, never via per-variant artwork.

So: a "filled bold crown" in BD style is a yellow chip with an ink ring and a white crown glyph on top — not a yellow-filled crown path. This keeps one piece of artwork per glyph and lets the same crown render active (red), on a gem (white), or plain (ink) from CSS alone.

## Meaning-colour → chip mapping

When an icon sits on a categorical chip, use the established palette (from `01-tokens.css`):

| Meaning | Token | Used for |
|---|---|---|
| realm / danger / destroy | `--red` | attack actions, destroy, the active state |
| defence / steel / accent | `--blue` | defend, defence attributes |
| growth / commit | `--green` | growth, commit, success |
| time / emphasis / warn | `--yellow` | time, emphasis, the nav `is-on` state |
| arcane / summon | `--purple` | arcane/summon units and chips |
| ink (outline + shadow) | `--ink` | the outline colour, hard offset shadows, drawback |

Resource gems keep their `--gem-*` mapping (tree/mountain/wheat/flame/sun/star) — that is the resource-colour system and is not duplicated here.

## Framing: bare glyphs are the default; chips are the framing

**There is no shield frame anymore.** The `#shield-frame` symbol, the `Shield()` composition, and `ShieldFrame()` are retired as general-purpose icon framing. Bare bold glyphs are the default surface treatment — this is the authentic BD/Asterix icon look (bold line drawings, no containing shape).

Where a framed treatment is wanted (categorical colour, identity, a clickable medallion), use a **flat colour chip with an ink ring** — the `.gem` pattern, generalised. Not a heraldic shield outline.

Implications for the redraw:
- The nav is **text-only labels** (no icons) — the "navigation stones" heraldic concept is dropped entirely.
- Army action tags and unit medallions move to **text tags** and **coloured medallions with initials/counts** (no glyphs); world-map markers move to **plain coloured dots**.
- `Shield()` and `ShieldFrame()` are deleted from `internal/ui/icons.go` (done — no callers remain). `Crest()` is the one framed treatment that stays — see below.

## The identity mark — The Coin (step 4 settled)

`Crest()` renders the kingdom/brand identity mark: **The Coin**, a bold BD medallion — a flat disc with a thick ink ring and flat fill. The shield outline is retired. Per-skin classes (`.id-crest`, `.home-chrome-crest`, `.crest-lg`) override the `--crest-ring` / `--crest-fill` tokens to set ring and fill colour.

The inner symbol is currently **empty** (a placeholder). The frame reads cleanly at all sizes without it; when a symbol is chosen it will be added as a child of the coin without changing the frame. World-map markers use the same coin, degraded gracefully to a bold 14px dot with the same ring/fill tokens.

The coin is a **standalone frame**, not composed from the sprite. Its art is CSS (border + background), not a `<symbol>` path. This is intentional: the coin is a single UI element, not a reusable glyph, and does not belong in the shared sprite.

## Glyph inventory and naming

The slim sprite keeps **nine symbols**, all bare ids in 24×24 space:

- Resources: `res-wood`, `res-stone`, `res-food`, `res-mana`, `res-devotion`, `res-knowledge`.
- Standalone: `crown` (identity mark), `idle` (sleeping zzz, renamed from `zzz`), `sandglass` (tick timer).

Everything else the sprite ever held (the heraldic glyphs `sword`, `swords`, `helmet`, `spear`, `soldiers`, `flag`, `flame`, `star`, `cross`, `chevron`, `house`, `person`, `globe`, `envelope`, `sliders`, `eye`, `book`, `scroll`, `quill`, `lantern`, `moon`, `sun`, `tree`, `mountain`, `wheat`, `scales`, plus `#shield-frame`) is **deleted**. The slots those glyphs filled are now text labels, coloured medallions with initials/counts, or plain dots — to be revisited later. No `shield-` prefix; no `g-` prefix.

The `/dev/icons` page remains the review surface for the kept set. The art-language rules above apply to any future glyph: a library path pasted into a `<symbol>` must be re-stroked to 2px round-cap `currentColor` or it will read as a third visual system.

## What this spec rules out

- Thin (sub-2px) strokes, square caps, engraving/ligne-claire rendering for game glyphs.
- Baked-in flat fills on glyph paths (breaks `currentColor` theming; use a chip instead).
- The 20×23 shield coordinate space and the `#shield-frame` composition for general icons.
- A thin modern flat library (Lucide/Tabler/Phosphor) imported without re-skinning to 2px round-cap.
- Two stroke weights in the same sprite.

## Confirmed decisions

1. **Bare-glyph default + chip framing** (no shield frame) — confirmed and realized; `Shield()`/`ShieldFrame()`/`#shield-frame` are deleted.
2. **24×24 / 2px / round** as the single weight — confirmed; every kept symbol is square 24×24.
3. **`sandglass` redrawn to 24×24** — confirmed; the whole sprite is square, so `Icon()` treats every symbol as square.
4. **Identity mark** — settled as **The Coin** (bold BD medallion: thick ink ring, flat fill, empty centre). The frame is standalone CSS, not a sprite symbol. The inner symbol is still TBD.
