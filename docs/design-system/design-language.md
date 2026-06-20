# Bahago Design Language

**Single source for the visual direction.** The coding agent reads this when
implementing a design handoff. If a handoff or implementation diverges from
this, this wins (same rule as handoff READMEs vs. existing code — the written
direction wins).

## What Bahago is

A browser-based multiplayer medieval kingdom strategy game. The art needs to
serve a game UI that re-renders fragments over SSE, shows lots of small repeated
marks at small sizes (rosters, tick meters, map markers), and reads clearly on
parchment-toned surfaces. The direction here is intentionally light: it fixes the
look and feel, not the specific components.

## The direction, in plain terms

**Bande dessinée** (Franco-Belgian comic), with two named references:

- **Astérix** (Uderzo's inking) — confident, chunky black outlines; flat fills
  separated by thick black lines; slightly exaggerated, hand-drawn cartoon
  quality. Not hairline engraving, not geometric modernism.
- **Dofus** (Ankama) — bold inked fantasy illustration; flat colour regions with
  hard black borders; cartoonish but readable.

Genre is **fantasy**: swords, shields, magic, prayer, medieval/folk buildings,
resources. The subject matter is not in question — only the rendering style.

There is no internal term for the look beyond "BD / Asterix / Dofus" — that is
the canonical reference set. Do not relabel it with a new internal term.

## Art direction

- **Bold ink outlines**, not thin technical pen work. Think 2–3px-equivalent
  strokes at icon scale.
- **Flat fills** inside the ink outline, with a thick black separating line
  between colour regions where regions meet.
- **Cartoon/hand-drawn quality**, not geometric. Slightly exaggerated
  proportions are good; perfect symmetry and mechanical precision are not the
  goal.
- **Hard black borders over soft shadows** where a border/shadow choice arises.
  The token layer defines `--shadow-ink` / `-sm` / `-xs` / `-lg` as hard offset
  shadows (`4px 4px 0 var(--ink)`) — prefer those over blurred drop shadows.
- **Fantasy subject matter stays.** A sword is a sword, a prayer scroll is a
  prayer scroll. What changes is how it's drawn, not what's drawn.

This is a *direction*, not a closed spec. When a handoff needs something the
points above don't cover, follow the spirit (bold ink, flat fills, cartoon,
hard shadows) and record the choice in the handoff. Thin flat-modern icon
libraries (Lucide/Tabler/Phosphor) fight this look; if one is used, its paths
must be re-stroked to the bold-ink weight, or it reads as a third visual system.

## Palette

Meaning colours are defined in `web/css/01-tokens.css` and are the authority.
They are flat, used as **fills**:

- `--red` — realm / danger / destroy
- `--blue` — defence / steel / accent
- `--green` — growth / commit
- `--yellow` — time / emphasis / warn
- `--purple` — arcane / summon
- `--ink` — the outline colour and hard-shadow colour (the comic signature)
- `--paper` / `--paper-2` / `--card` / `--card-2` — surfaces (parchment-toned)

Resource gems (`--gem-tree`, `--gem-mountain`, `--gem-wheat`, `--gem-flame`,
`--gem-sun`, `--gem-star`) are the established resource-colour mapping.

## Typography

From `01-tokens.css`:

- `--font-display` (Lilita One) — titles, labels, numerals, buttons.
- `--font-body` / `--font-heading` / `--font-ui` (Nunito) — body and prose.

Lilita One's heavy, rounded, poster quality is consistent with the BD
direction; do not propose thin serif faces for headings.

## Relationship to the icon system

Icons are rendered via a sprite + `<use>` mechanism. The **mechanism** (sprite,
`<use>`, `currentColor`, embed) is a stable plumbing choice; the **art** inside
the sprite is what this direction governs. The icon spec lives in
`docs/design-system/icons.md`.
