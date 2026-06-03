# Component Registry

**Single writer: Claude Code.** Claude Design reads this file at the start of each session to know what is already implemented and how components are named in the codebase. Claude Design supplies "Registry delta" sections in each handoff with the exact rows to add or update — applying them is mechanical.

**Status values:** `live` — in `styles.css` and in use · `pending` — handoff received, not yet implemented · `in-progress` — partially implemented

---

## Shared / cross-feature

| Component | Class(es) | Status | styles.css Section | Design Reference |
|---|---|---|---|---|
| Button | `.btn`, `.btn--sm`, `.btn--primary`, `.btn--danger`, `.btn--quiet`, `.is-disabled`, `.btn--insufficient`, `.btn--locked` | live | Shared components | Button system |
| Card | `.card`, `.card-tab--wood`, `.card-tab--red`, `.card-tab--green` | live | Shared components | Card |
| Slider | `.slider-*`, `.v-diamond` | live | Allocation | Slider |
| Gem / resource icon | `.gem`, `.gem-<id>` (tree, mountain, wheat, flame, sun, star) | live | Kingdom chrome | Resource gems |
| Alert | `.alert--error`, `.alert--success` | live | Shared components | Alert |
| Topbar chrome | `.topbar`, `.tick-chip` | live | Kingdom chrome | Topbar |

---

## Unit system  _(handoff: `design_handoff_unit/`)_

| Component | Class(es) | Status | styles.css Section | Design Reference |
|---|---|---|---|---|
| Unit portrait | `.unit-portrait`, `.unit-portrait--lg`, `.unit-portrait--sm`, `.unit-portrait--empty` | pending | Units | Unit portrait |
| Attribute chips | `.kattr`, `.kattr-row`, `.kattr--offense`, `.kattr--ward`, `.kattr--arcane`, `.kattr--faith`, `.kattr--drawback` | pending | Units | Attribute chips |
| Roster roll | `.muster-roll`, `.muster-roll-head`, `.muster-roll-foot`, `.unit-row`, `.unit-row-id`, `.unit-name`, `.unit-flavour` | pending | Units | Roster — Roll |
| Roster gallery | `.muster-gallery`, `.unit-card` | pending | Units | Roster — Gallery |
| Tick meter | `.kmeter`, `.kmeter-fill` | pending | Units | Tick meter |
| Locked state | `.is-locked`, `.unit-lock-note` | pending | Units | Locked / unavailable unit |
| Host summary strip | `.host-summary` | pending | Units | Host summary strip |
| Training slots | `.train-slots`, `.train-slot`, `.train-slot.is-active`, `.train-slot.is-idle`, `.slot-gauge` | pending | Units | Training grounds — slot model |
| Stat pill | `.stat-pill`, `.stat-pill--drain`, `.stat-pill--short`, `.pill-short` | pending | Units | Stat pills |
| Unit token + tally | `.unit-token`, `.unit-tally` | pending | Units | Portrait token |
| Train control | `.train-ctl` | pending | Units | Inline train control |
| Lock banner | `.lock-banner` | pending | Units | Summons — lock banner |
| Toast | `.toast` | pending | Units | Toast notification |
