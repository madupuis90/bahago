# Bahago Design Language

**Single source for the visual direction.** The coding agent reads this when implementing a design handoff. If a handoff or implementation diverges from this, this wins (same rule as handoff READMEs vs. existing code — the written direction wins).

## The direction, in plain terms

**Bande dessinée** (Franco-Belgian comic), with two named references:

- **Astérix** (Uderzo's inking) — confident, chunky black outlines; flat fills separated by thick black lines; slightly exaggerated, hand-drawn cartoon quality. Not hairline engraving, not geometric modernism.
- **Dofus** (Ankama) — bold inked fantasy illustration; flat colour regions with hard black borders; cartoonish but readable.

Genre is **fantasy**: swords, shields, magic, prayer, medieval/folk buildings, resources. The subject matter is not in question — only the rendering style.

## Why this file exists

Earlier sessions invented labels ("heraldic", "Comic Lab") that were never the actual direction, and drifted the icon art and some class names toward a thin ligne-claire / engraving aesthetic that is the opposite of BD. There is no internal term for the look beyond "BD / Asterix / Dofus" — that is fine and is the canonical reference set. Do not relabel it.

## Concrete implications for artwork (icons, crests, illustrations)

- **Bold ink outlines**, not thin technical pen work. Think 2–3px-equivalent strokes at icon scale, not the 0.85px ligne-claire the old sprite used.
- **Flat fills** inside the ink outline, with a thick black separating line between colour regions where regions meet.
- **Cartoon/hand-drawn quality**, not geometric. Slightly exaggerated proportions are good; perfect symmetry and mechanical precision are not the goal.
- **Hard black borders over soft shadows** where a border/shadow choice arises. The token layer already defines `--shadow-ink` / `--shadow-ink-sm` / `--shadow-ink-xs` / `--shadow-ink-lg` as hard offset shadows (`4px 4px 0 var(--ink)`) — use those, not blurred drop shadows.
- **Fantasy subject matter stays.** A sword is a sword, a prayer scroll is a prayer scroll. What changes is how it's drawn, not what's drawn.

## What this rules out

- Thin flat-modern icon libraries (Lucide, Tabler, Phosphor) as-is — their stroke weight and geometric plainness fight the BD look. If a library icon is used, its path must be re-stroked to match the bold-ink language, or it will read as a third visual system alongside the custom art.
- Hairline / engraving / ligne-claire rendering for game art.
- Soft blurred drop shadows where a hard offset ink shadow is the established pattern.
- Relabelling the direction with a new internal term.

## Palette

Meaning colours are defined in `web/css/01-tokens.css` and are the authority. They are flat, used as **fills**, not as a heraldic blazon system:

- `--red` — realm / danger / destroy
- `--blue` — defence / steel / accent
- `--green` — growth / commit
- `--yellow` — time / emphasis / warn
- `--purple` — arcane / summon
- `--ink` — the outline colour and hard-shadow colour (the comic signature)
- `--paper` / `--paper-2` / `--card` / `--card-2` — surfaces (parchment-toned)

Resource gems (`--gem-tree`, `--gem-mountain`, `--gem-wheat`, `--gem-flame`, `--gem-sun`, `--gem-star`) are the established resource-colour mapping and stay.

## Typography

From `01-tokens.css`:

- `--font-display` (Lilita One) — titles, labels, numerals, buttons.
- `--font-body` / `--font-heading` / `--font-ui` (Nunito) — body and prose.

Lilita One's heavy, rounded, poster quality is consistent with the BD direction; do not propose thin serif faces for headings.

## Relationship to the icon system

Icons are rendered via the sprite + `<use>` mechanism (see `docs/adr/0003-sprite-icon-system.md`). The mechanism is fixed; the **art** inside the sprite is what this language governs. The detailed icon spec (stroke weights, fill rules, framed-vs-bare treatment now that shields are not the default motif) lives in `docs/design-system/icons.md` and must conform to this file.

## Relationship to the identity mark (settled)

The kingdom/brand identity mark is **The Coin**: a bold BD medallion — a flat disc with a thick ink ring and flat fill. The shield outline is retired; the coin is the one framed treatment that survives in the UI. The inner symbol is currently empty (a placeholder) and will be chosen in a future session; the frame reads cleanly at all sizes without it.

The coin appears in three sizes (auth hero ~54px, CommandBar ~40px, home brand ~32px) and degrades gracefully to a bold dot on the world map (~14px). All kingdoms share the same mark — per-kingdom differentiation is a future system, not part of this decision.
