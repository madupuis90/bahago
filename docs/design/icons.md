# Icon System

Spec for the sprite artwork and the icon rendering mechanism. Read this before
touching `web/static/sprite.svg` or the icon helpers in `internal/ui/icons.go`.

## Mechanism: sprite + `<use>` + `currentColor`

Icons are small `<svg>` stubs referencing shared `<symbol>` definitions in a
single `web/static/sprite.svg` via `<use href="/static/sprite.svg#id">`. The
sprite is embedded into the binary at build time (`web.Static` `embed.FS`) and
served with long-cache headers. All theming flows through `currentColor`, so a
colour variant (`.icon--active`, `.gem .icon { color:#fff }`) is one CSS rule —
no per-variant files or markup forks.

This fits the rendering model (Go via gomponents, no JS component runtime,
fragments re-rendered over SSE): one small file fetched once and cached, every
icon instance a ~50-byte stub, composition a second `<use>` in the same `<svg>`.

Two constraints worth remembering:

- **Composition requires a shared coordinate space.** A frame `<symbol>` and
  the glyphs composed over it must share the same viewBox so a second `<use>`
  overlays the first with no offset math.
- **`<use>` cross-document reference needs the sprite served from the same
  origin** (it is — a static asset). It does not work from a `data:` URI or a
  different origin without CORS considerations.

## Helpers (`internal/ui/icons.go`)

`Icon(id, sizePx, active)` renders a sprite glyph by id. The package parses
every `<symbol id="…">` and its `viewBox` from the embedded sprite once at
init, so the height is derived from the symbol's real aspect ratio. Adding a
new symbol with a new aspect ratio only needs its `viewBox` set — no Go change.
The target is square 24×24 everywhere, which lets `Icon()` treat every symbol
as square.

- `Icon(id, sizePx, active)` — a bare glyph. `active` toggles the
  `.icon--active` accent modifier.
- `Crest(id, sizePx, extraClass)` — a standalone coin (disc with a thick ink
  ring); the `extraClass` (`crest-lg`, `id-crest`, `home-chrome-crest`, …)
  selects ring and fill colour via `.crest-*` tokens. The inner symbol is
  optional (empty `id` renders the coin without a glyph).
- `ResourceGem(gemID, sizePx)` — the filled resource gem: the coloured disc
  (`.gem-<gemID>`) with the white resource glyph on top. `gemID` is the gem
  colour id (tree/mountain/wheat/flame/sun/star), mapped to the resource
  symbol key (wood/stone/food/mana/devotion/knowledge).

## Geometry

Every glyph in the sprite targets one visual weight on one coordinate space:

| Property | Value | Why |
|---|---|---|
| viewBox | `0 0 24 24` for every glyph | One aspect ratio across the sprite; `Icon(id, size)` derives a square height for all of them; no per-category sizing surprises. |
| Stroke width | `2` | Matches the existing resource art so the sprite is one weight. |
| Stroke caps | `round` | Matches the existing resource art; keeps small glyphs soft at the render sizes used. |
| Stroke joins | `round` | Same. |
| Fill | `none` on the glyph itself | Glyphs stay monochrome; colour is applied around them, not baked in (see below). |
| Inset | keep strokes inside the 24×24 box (account for the 1px either side a 2px round stroke needs) | Prevents clipping when the `<use>` is scaled down. |

If a genuinely non-square symbol is ever needed, the `Icon` helper derives
height from the viewBox — but the target is square-everywhere.

## Colour & theming

Separate **glyph** from **chip**:

- **The glyph is monochrome line art** (`fill: none`, `stroke: currentColor`).
  It inherits colour from CSS. This is what makes `.icon--active { color:
  var(--red) }`, `.gem .icon { color: #fff }`, and dim states each a one-line
  CSS rule. Do not bake flat fills into glyph paths.
- **A flat colour is delivered by the chip behind the glyph** — the established
  `.gem` pattern: a flat `--gem-*` disc with a bold ink ring and a white glyph
  on top. Extend it, don't replace it.
- **State colour** (active/dim/meaning-colour) is applied via `currentColor` on
  the glyph's container, never via per-variant artwork.

So a "filled bold crown" is a yellow chip with an ink ring and a white crown
glyph on top — not a yellow-filled crown path. This keeps one piece of artwork
per glyph and lets the same crown render active (red), on a gem (white), or
plain (ink) from CSS alone.

## Meaning-colour → chip mapping

When an icon sits on a categorical chip, use the established palette (from
`01-tokens.css`):

| Meaning | Token | Used for |
|---|---|---|
| realm / danger / destroy | `--red` | attack actions, destroy, the active state |
| defence / steel / accent | `--blue` | defend, defence attributes |
| growth / commit | `--green` | growth, commit, success |
| time / emphasis / warn | `--yellow` | time, emphasis, the nav `is-on` state |
| arcane / summon | `--purple` | arcane/summon units and chips |
| ink (outline + shadow) | `--ink` | the outline colour, hard offset shadows, drawback |

Resource gems keep their `--gem-*` mapping (tree/mountain/wheat/flame/sun/star)
— that is the resource-colour system and is not duplicated here.

## Framing: bare glyphs are the default; chips are the framing

There is no shield frame. `Shield()` / `ShieldFrame()` / `#shield-frame` were
retired as general-purpose icon framing. Bare glyphs are the default surface
treatment — monoline line drawings, no containing shape.

Where a framed treatment is wanted (categorical colour, identity, a clickable
medallion), use a **flat colour chip with an ink ring** — the `.gem` pattern,
generalised. Not a heraldic shield outline.

> **Note on the current sprite art.** The glyphs in `sprite.svg` are bold
> monoline. The chrome treats them as struck-on-metal marks (chip with ink
> ring, currentColor glyph on top — see `docs/design/ui-design.md` for the
> material direction). If the sprite is ever re-skinned, do it in a single
> pass so the sprite stays one visual system; until then, keep new glyphs
> consistent with the existing weight (`stroke-width="2"`, round caps/joins,
> `currentColor`, 24×24).

## Glyph inventory

The sprite keeps **twelve symbols**, all bare ids in 24×24 space:

- Resources (6): `res-wood`, `res-stone`, `res-food`, `res-mana`,
  `res-devotion`, `res-knowledge`.
- Standalone (6): `arrow-down` / `arrow-up` (direction markers for
  per-tick rate cost pills — upkeep down, production up), `crown` (realm),
  `envelope` (auth recovery-sent state), `idle` (sleeping zzz, renamed from
  `zzz`), `sandglass` (tick timer).

Slots the sprite used to hold (heraldic glyphs like `sword`, `swords`,
`helmet`, `spear`, `soldiers`, `flag`, `flame`, `cross`, `chevron`,
`house`, `person`, `globe`, etc., plus `#shield-frame`) are deleted. Those
slots are now text labels, coloured medallions with initials/counts, or plain
dots. No `shield-` prefix; no `g-` prefix.

The `/dev/icons` page (mounted by `internal/handlers/iconpreview/`) is the
review surface for the sprite — open it when checking glyphs at render size.
The rules above apply to any future glyph: a library path pasted into a
`<symbol>` must be re-stroked to 2px round-cap `currentColor` or it will read
as a third visual system.