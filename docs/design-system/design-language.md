# Bahago Design Language

**Single source for the visual direction.** The coding agent reads this when
implementing a design handoff. If a handoff or implementation diverges from
this, this wins (same rule as handoff READMEs vs. existing code — the written
direction wins).

## What Bahago is

A browser-based multiplayer medieval kingdom strategy game. The art needs to
serve a game UI that re-renders fragments over SSE, shows lots of small repeated
marks at small sizes (rosters, tick meters, map markers), and reads clearly on
parchment-toned surfaces that sit inside a dark, leather-and-metal frame. The
direction here is intentionally light: it fixes the look and feel, not the
specific components.

## The direction, in plain terms

**Dark-fantasy codex** — an illuminated, leather-bound CRPG interface. The
canonical reference is **Pathfinder: Kingmaker** (Owlcat), with two supporting
registers:

- **Pathfinder: Kingmaker** — the primary reference. Dark wood/leather frames
  with brass-riveted bevels; recessed parchment panels floated on a dark
  surface; serif inscription caps for titles; soft recessed/raised shadows, not
  hard offset shadows; muted, aged heraldic colours over bright cartoon ones.
  This is the look we want on every chrome edge and content panel.
- **Fantasy CRPG / "old-school online RPG"** (the genre feeling) — readable
  data tables, gold trim on active states, glowing magical accents, and a
  parchment-and-iron material vocabulary. The subject matter reads as a
  kingdom-management console, not a casual mobile cartoon.

This replaced an earlier **bande dessinée** (Astérix / Dofus) direction. That
direction is retired; do not reintroduce bold ink outlines, flat comic fills, or
hard offset ink shadows. Where old code or handoffs still assume BD, this doc
wins.

There is no internal term for the look beyond "the Codex direction" / "PK
inspired." Do not relabel it with a new internal term.

## Art direction

- **Dark frame, light panels.** The chrome is dark leather/wood with brass
  trim; content sits on recessed parchment. The contrast is the signature.
- **Brass bevels, not flat ink lines.** Edges are layered: a light top inner
  highlight and a dark bottom inner shadow read as a raised/recessed metal
  frame. A single flat `Npx solid` border is the exception, not the rule.
- **Muted, aged heraldic colours.** Realm red is a deep crimson, not a comic
  red; steel blue is pewter, not bright; gold is the brass trim colour. The
  palette is `01-tokens.css` and is the authority.
- **Soft shadows over hard offset shadows.** The old `4px 4px 0 var(--ink)`
  comic shadow is retired. The `--shadow-*` tokens are now soft, layered
  drop/inset shadows. Prefer beveled insets (`--shadow-recessed`,
  `--shadow-raised`) for the material read.
- **Serif display, sans body.** Cinzel (Roman-inscription caps) for titles,
  labels, numerals, and buttons; Nunito for body, tables, and prose.
  Readability beats flourish: numbers and rosters stay Nunito at small sizes.
- **Gold glow for active/magical states.** Active nav, alerts, and arcane
  accents get a soft warm glow, not a bright flat fill swap.
- **Fantasy subject matter stays.** A sword is a sword, a prayer scroll is a
  prayer scroll. What changes is how it's framed, not what's drawn.

This is a *direction*, not a closed spec. When a handoff needs something the
points above don't cover, follow the spirit (dark frame, parchment panels, brass
bevels, soft shadows, Cinzel/Nunito, muted heraldry) and record the choice in the
handoff. Thin flat-modern icon libraries (Lucide/Tabler/Phosphor) fight this
look; if one is used, its paths must be re-stroked to the sprite's `stroke-width`
and `currentColor` weight, or it reads as a third visual system.

## Materials vocabulary

- **Leather** (`--leather`, `--leather-2`, `--leather-edge`) — the dark frame.
  Used for the topbar, nav bands, and the page background wash.
- **Parchment** (`--paper`, `--paper-2`, `--card`, `--card-2`) — the light
  surface for content panels and recessed wells.
- **Brass** (`--brass`, `--brass-light`, `--brass-dark`) — trim, rivets, frame
  bevels, and the gold accent for active states.
- **Ink** (`--ink`) — dark brown, the text and outline colour on parchment. Not
  pure black; warm to match the leather family.
- **Heraldic fills** (`--red`, `--blue`, `--green`, `--yellow`, `--purple`) —
  muted, aged. Used sparingly for meaning, not as comic flats.

## Palette

Meaning colours are defined in `web/css/01-tokens.css` and are the authority:

- `--red` — realm / danger / destroy (deep crimson)
- `--blue` — defence / steel / accent (pewter)
- `--green` — growth / commit (forest)
- `--yellow` — time / emphasis / warn (aged gold; pairs with `--brass`)
- `--purple` — arcane / summon (muted violet)
- `--ink` — text/outline colour on parchment, and the warm shadow colour
- `--brass` / `--brass-light` / `--brass-dark` — trim, bevels, active gold
- `--leather` / `--leather-2` / `--leather-edge` — the dark frame
- `--paper` / `--paper-2` / `--card` / `--card-2` — parchment surfaces

Resource gems (`--gem-tree`, `--gem-mountain`, `--gem-wheat`, `--gem-flame`,
`--gem-sun`, `--gem-star`) keep their resource-colour mapping, now muted to read
as aged metal medallions rather than bright comic discs.

## Typography

From `01-tokens.css`:

- `--font-display` / `--font-cinzel` (Cinzel) — titles, labels, numerals,
  buttons. Use weights 600–800 for the inscription look; Cinzel 400 is too thin
  for headings. Apply wide letter-spacing for the caps register.
- `--font-body` / `--font-heading` / `--font-ui` (Nunito) — body, tables, prose.

Do not propose rounded poster faces (Lilita One etc.) for headings — that was
the retired BD look.

## Relationship to the icon system

Icons are rendered via a sprite + `<use>` mechanism. The **mechanism** (sprite,
`<use>`, `currentColor`, embed) is a stable plumbing choice; the **art** inside
the sprite is what this direction governs. The icon spec lives in
`docs/design-system/icons.md`.

The current sprite art is bold-line inked glyphs drawn for the BD direction. It
is **not** blocked by the chrome handoff: the few glyphs the chrome itself needs
(crest, gem, sandglass, leave) are retinted via CSS in this pass; a full sprite
re-skin toward finer, metal-stamped line art is a follow-up that will be tracked
in the icon spec.