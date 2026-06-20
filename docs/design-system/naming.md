# Naming Conventions: Design Reference → Codebase

Translation table from design-prototype names to codebase semantic names, plus the permanent naming rules for new components. The table exists to cover the transition from the old prefix system (`.kbtn`, `.folio`, etc.); the **naming rules below are permanent** — they codify a standing instruction from the project owner.

---

## Translation table

| Prototype / `reference.css` name | Codebase (semantic) name | Status | Note |
|---|---|---|---|
| `.kbtn`, `.kbtn--primary`, `.kbtn--danger`, `.kbtn--quiet`, `.kbtn--sm`, `.btn--insufficient`, `.btn--locked`, `.is-disabled` | `.btn` + same modifier suffixes | live | One button system; modifiers map 1:1 |
| `.kslider` | `.slider-*` + `.v-diamond` | live | Already in `styles.css` |
| `.folio` | `.page-header` (or feature-specific, e.g. `.unit-folio`) | pending | Bookish name → semantic; exact name chosen per feature |
| `.kfield`, `.kfield-group`, `.kfield-label`, `.kfield-hint` | `.field`, `.field-group`, `.field-label`, `.field-hint` | live | Campaign handoff renamed `.kfield*` → `.field*`; now in `styles.css` Shared components section |
| `.kattr`, `.kattr-row`, `.kattr--*` | `.attribute`, `.attribute-row`, `.attribute--*` | live · resolved | Units handoff: spelled-out semantic name (mockup's `.attr` is shorthand; code uses the full word) |
| `.kmeter`, `.kmeter-fill`, `.kmeter-ticks` | `.meter` + `.meter-*` (`.meter-track`/`-fill`/`-notches`/`-notch`/`-top`/`-name`/`-qty`/`-eta`) | live · resolved | Units handoff: extracted as shared `.meter` to `20-shared.css`; notches are now explicit flex children (was a single repeating-gradient node) |
| `.host-summary`, `.hs-label`/`-num`/`-sub` | `.units-summary`, `.units-summary-label`/`-num`/`-sub` (with `.units-summary-val` wrapper) | live · resolved | Units handoff: aligned with the registry's existing `.units-summary` name |

---

## Naming rules (apply when adding new components — permanent)

These rules are a standing project instruction, not a transitional scaffold.

### No narrative / in-fiction flavor in class names

Class names describe **structure and semantics**, not in-fiction dressing. Use structural names: `title`, `sub-title`, `section`, `item`, `meta`, `row`, `cell`, `body`, `foot`, `actions`, `toolbar`, `list`, `grid`, `panel`, `card`.

**Rejected (narrative dressing):** `charter`, `oath`, `decree`, `seal`, `missive`, `ledger`, `grimoire`, `folio`, `tome`, `codex`, `edict`, `sigil`, `tablet`, `vellum`, `parchment` (as a class name for a surface — use `card`/`panel`), `scriptorium`, `refectory`, `narthex`, `vestibule`, `herald`, `pennant`, `banner` (when it just means `header`/`section`).

### Game-mechanic nouns are allowed

Nouns that name a real game mechanic or object are semantic, not flavor:

- `unit`, `legion`, `roster`, `muster`, `summon`, `portrait`, `medallion`, `gem`, `legion-card`, `unit-portrait`, `muster-roll`
- `resource` and the resource keys (`wood`, `stone`, `food`, `mana`, `devotion`, `knowledge`)
- `army`, `campaign`, `guild`, `prayer`, `building`, `kingdom`, `world-map`

The test: does the noun name a thing the game simulation or UI treats as a first-class object? If yes, it is semantic. Does it only evoke atmosphere from a medieval scribe's office? If yes, it is flavor and is rejected.

If a class name would sit awkwardly on both sides, use the structural name instead (`section`, `panel`, `item`).

### Other rules

- **Strip decorative prefixes** (`k`, bookish words like `folio`) — use plain semantic names
- **BEM modifier convention** — double-dash for state/variant (`.btn--primary`, `.unit-portrait--lg`), single-dash for sub-elements (`.unit-row-id`, `.muster-roll-head`)
- **Never mix `Class()` and `Classes{}`** on the same element — fold everything into one `Classes{}`

### Existing offenders (to be renamed, not propagated)

These class names pre-date the rule and are slated for replacement — do not use them in new work, and rename them when touching their owning feature:

| Flavor name | Semantic replacement | Where |
|---|---|---|
| `charter-*` (`charter-grid`, `charter-form`, `charter-actions`, `charter-oath`, `charter-text`, `charter-wrap`, `charter-lapse`, `charter-item*`) | `guild-*` / structural (`grid`, `form`, `actions`, `section`, `text`, `wrap`, `item`) | guild |
| `charter-oath` | `guild-preamble` or `guild-text` | guild |
| `msg-seal`, `seal-cell`, `seal-count`, `sealing-meter` | `message-mark`, `mark-cell`, `mark-count`, `mark-meter` (or `message-meter`) | messages |
| `message-mark--decree` | `message-mark--pinned` or `--official` | messages |
| `alloc-decree` | `alloc-section` or `alloc-notice` | allocation |
| `ledger-footnote` | `units-footnote` or `footnote` | units |
| `attribute--ward` | `attribute--defense` (pairs with `--offense`; the other category tags `--offense`/`--arcane`/`--faith`/`--drawback` are game-mechanic and stay) | units |

> **Not offenders (confirmed allowed):** `.allocation-stone` is part of the `.allocation-<resource>` family (wood/stone/food/mana/devotion/knowledge) keyed by resource name — `stone` is the game-mechanic resource key, not flavor. `allocation-stone` stays.
| `letter-*` (`letter-sheet`, `letter-body`, `letter-subject`, `letter-rule`, `letter-corner-mark`, `letter-action`, `letter-action-note`) | `message-detail-*` (`-sheet`→`-card`/`-wrap`, `-body`, `-subject`→`-title`, `-rule`→`-divider`, `-corner-mark`→`-badge`, `-action`, `-action-note`) | messages (the single-message read view) |
| `rite-*` (`rite-list`, `rite-body`, `rite-text`, `rite-note`, `rite-name`) | `steps-*` (`-list`, `-body`, `-text`, `-note`, `-name`) or `found-steps-*` | guild (the ordered founding steps) |
| `door`, `doors`, `door-inner`, `door-title`, `door-text`, `door-meta`, `door-crest`, `doors-seam` | `choice` / `choices`, `choice-inner`, `choice-title`, `choice-text`, `choice-meta`, `choice-icon`, `choices-divider` | guild (the two Find/Found choice cards) |
