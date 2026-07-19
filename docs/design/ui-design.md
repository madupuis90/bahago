# UI Design

High-level visual direction for Bahago: theme, feel, inspiration, and
typography. This fixes the *look*, not specific components. When the code or a
proposed change diverges from this, this wins.

## What Bahago is

A browser-based multiplayer medieval kingdom strategy game. The UI re-renders
fragments over SSE, shows lots of small repeated marks at small sizes (rosters,
tick meters, map markers), and reads clearly on parchment-toned surfaces that
sit inside a dark, leather-and-metal frame.

## The direction, in plain terms

A dark leather-and-metal frame holding recessed parchment panels — an
illuminated, leather-bound kingdom-management console. The closest concrete
reference is **Pathfinder: Kingmaker** (Owlcat), and the points below describe
what to take from it:

- Dark wood/leather frames with brass-riveted bevels; recessed parchment panels
  floated on a dark surface; serif inscription caps for titles; soft
  recessed/raised shadows, not hard offset shadows; muted, aged heraldic
  colours over bright cartoon ones. This is the look for every chrome edge and
  content panel.
- The supporting genre feeling — "old-school online RPG": readable data tables,
  gold trim on active states, glowing magical accents, a parchment-and-iron
  material vocabulary. The subject matter reads as a kingdom-management
  console, not a casual mobile cartoon.

Do not give the look an internal brand name. Describe it by what it is —
materials, palette, typography, shadow treatment — as this file does.

Earlier work used bold ink outlines, flat comic fills, and hard offset ink
shadows. Those are not the direction; do not reintroduce them.

## Art direction

- **Dark frame, light panels.** The chrome is dark leather/wood with brass
  trim; content sits on recessed parchment. The contrast is the signature.
- **Brass bevels, not flat ink lines.** Edges are layered: a light top inner
  highlight and a dark bottom inner shadow read as a raised/recessed metal
  frame. A single flat `Npx solid` border is the exception, not the rule.
- **Muted, aged heraldic colours.** Realm red is a deep crimson, not a comic
  red; steel blue is pewter, not bright; gold is the brass trim colour. The
  palette lives in `web/css/01-tokens.css` and is the authority.
- **Soft shadows over hard offset shadows.** The `4px 4px 0 var(--ink)`
  hard offset shadow is not the direction. The `--shadow-*` tokens are soft,
  layered drop/inset shadows. Prefer beveled insets (`--shadow-recessed`,
  `--shadow-raised`) for the material read.
- **Serif display, sans body.** Cinzel (Roman-inscription caps) for titles,
  labels, numerals, and buttons; Nunito for body, tables, and prose.
  Readability beats flourish: numbers and rosters stay Nunito at small sizes.
- **Gold glow for active/magical states.** Active nav, alerts, and arcane
  accents get a soft warm glow, not a bright flat fill swap.
- **Fantasy subject matter stays.** A sword is a sword, a prayer scroll is a
  prayer scroll. What changes is how it's framed, not what's drawn.

This is a *direction*, not a closed spec. When a point above doesn't cover a
case, follow the spirit (dark frame, parchment panels, brass bevels, soft
shadows, Cinzel/Nunito, muted heraldry) and record the choice where the code
lives. Thin flat-modern icon libraries (Lucide/Tabler/Phosphor) fight this look;
if one is used, its paths must be re-stroked to the sprite's `stroke-width` and
`currentColor` weight, or it reads as a third visual system.

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
`--gem-sun`, `--gem-star`) keep their resource-colour mapping, muted to read as
aged metal medallions rather than bright comic discs.

## Typography

From `01-tokens.css`:

- `--font-display` / `--font-cinzel` (Cinzel) — titles, labels, numerals,
  buttons. Use weights 600–800 for the inscription look; Cinzel 400 is too thin
  for headings. Apply wide letter-spacing for the caps register.
- `--font-body` / `--font-heading` / `--font-ui` (Nunito) — body, tables, prose.

Do not propose rounded poster faces (Lilita One etc.) for headings — rounded
poster faces do not fit the inscription register.

## Related files

- `web/css/01-tokens.css` — the authoritative palette, fonts, shadows, spacing.
- `web/static/sprite.svg` + `internal/ui/icons.go` — the icon system. See
  `docs/design/icons.md` for the sprite mechanism.