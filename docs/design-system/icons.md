# Icon Language

Spec for sprite artwork and the icon rendering mechanism. Read this before
touching `web/static/sprite.svg` or the icon helpers in `internal/ui/icons.go`.

This is a working spec, not a closed design record. It fixes the mechanism and
the art direction; where a handoff needs something it doesn't cover, follow the
spirit (bold ink, flat fills, square 24×24 glyphs, `currentColor`) and record
the choice in the handoff.

## Mechanism: sprite + `<use>` + `currentColor`

Icons are small `<svg>` stubs referencing shared `<symbol>` definitions in a
single `web/static/sprite.svg` via `<use href="/static/sprite.svg#id">`. The
sprite is embedded into the binary at build time (`web.Static` `embed.FS`) and
served with long-cache headers. All theming flows through `currentColor`, so
colour variants (`.icon--active`, `.gem .icon { color:#fff }`) are one CSS rule
each — no per-variant files or markup forks.

This mechanism is a stable plumbing choice. It fits the rendering model (Go via
gomponents, no JS component runtime, fragments re-rendered over SSE): one ~14KB
file fetched once and cached, every icon instance a ~50-byte stub, composition a
second `<use>` in the same `<svg>`. Two constraints worth remembering:

- **Composition requires a shared coordinate space.** A frame `<symbol>` and the
  glyphs composed over it must share the same viewBox so a second `<use>`
  overlays the first with no offset math.
- **`<use>` cross-document reference needs the sprite served from the same
  origin** (it is — a static asset). It does not work from a `data:` URI or a
  different origin without CORS considerations.

`internal/ui/icons.go` parses every `<symbol id="…">` and its `viewBox` from the
embedded sprite once at init, so `Icon(id, sizePx)` derives height from the real
aspect ratio. Adding a new symbol with a new aspect ratio only needs its
`viewBox` set; no Go change. The target is square 24×24 everywhere (see below),
which lets `Icon()` treat every symbol as square.

## Art direction: standardize on what the resource art already does

The six `res-*` symbols (`res-wood`, `res-stone`, `res-food`, `res-mana`,
`res-devotion`, `res-knowledge`) are already drawn in the target language:
**24×24 viewBox, `stroke-width="2"`, round caps/joins, `currentColor`,
no fill**. They are the reference. **Every glyph in the sprite targets this
language** — one visual weight, one coordinate space.

This conforms to `docs/design-system/design-language.md` (bande dessinée, bold
ink, flat fills, fantasy subject).

## Geometry

| Property | Value | Why |
|---|---|---|
| viewBox | `0 0 24 24` for every glyph | One aspect ratio across the sprite; `Icon(id, size)` derives a square height for all of them; no per-category sizing surprises. |
| Stroke width | `2` (the `res-*` weight) | Bold enough to read as BD ink at 16–48px render sizes; matches the existing resource art so the sprite is one weight. |
| Stroke caps | `round` | Cartoon-friendly; the BD default. |
| Stroke joins | `round` | Same reasoning. |
| Fill | `none` on the glyph itself | Glyphs stay monochrome; colour is applied around them, not baked in (see "Colour & theming"). |
| Inset | keep strokes inside the 24×24 box (account for the 1px either side that a 2px round stroke needs) | Prevents clipping when the `<use>` is scaled down. The `res-*` art already respects this. |

If a genuinely non-square symbol is ever needed, the `Icon` helper derives
height from the viewBox — but the target is square-everywhere.

## Colour & theming

BD art is flat fills inside a bold black outline. The sprite's theming mechanism
is `currentColor`. These coexist once you separate **glyph** from **chip**:

- **The glyph is monochrome line art** (`fill: none`, `stroke: currentColor`).
  It inherits colour from CSS. This is what makes `.icon--active { color:
  var(--red) }`, `.gem .icon { color: #fff }`, and dim states each a one-line CSS
  rule. Do not bake flat fills into glyph paths.
- **Flat BD colour is delivered by the chip behind the glyph** — the established
  `.gem` pattern: a flat `--gem-*` disc with a bold ink ring and a white glyph
  on top. Extend it, don't replace it.
- **State colour** (active/dim/meaning-colour) is applied via `currentColor` on
  the glyph's container, never via per-variant artwork.

So a "filled bold crown" in BD style is a yellow chip with an ink ring and a
white crown glyph on top — not a yellow-filled crown path. This keeps one piece
of artwork per glyph and lets the same crown render active (red), on a gem
(white), or plain (ink) from CSS alone.

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
retired as general-purpose icon framing. Bare bold glyphs are the default
surface treatment — the authentic BD/Asterix icon look (bold line drawings, no
containing shape).

Where a framed treatment is wanted (categorical colour, identity, a clickable
medallion), use a **flat colour chip with an ink ring** — the `.gem` pattern,
generalised. Not a heraldic shield outline.

## Glyph inventory and naming

The slim sprite keeps **nine symbols**, all bare ids in 24×24 space:

- Resources: `res-wood`, `res-stone`, `res-food`, `res-mana`, `res-devotion`,
  `res-knowledge`.
- Standalone: `crown`, `idle` (sleeping zzz, renamed from `zzz`), `sandglass`
  (tick timer).

Slots the sprite used to hold (heraldic glyphs like `sword`, `swords`,
`helmet`, `spear`, `soldiers`, `flag`, `flame`, `star`, `cross`, `chevron`,
`house`, `person`, `globe`, `envelope`, etc., plus `#shield-frame`) are deleted.
Those slots are now text labels, coloured medallions with initials/counts, or
plain dots. No `shield-` prefix; no `g-` prefix.

The `/dev/icons` page remains the review surface for the kept set. The
art-language rules above apply to any future glyph: a library path pasted into a
`<symbol>` must be re-stroked to 2px round-cap `currentColor` or it will read as
a third visual system.
