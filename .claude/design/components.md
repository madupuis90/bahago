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
| CommandBar chrome | `.bar`, `.barB2`, `.barB2-info`, `.barB2-res`, `.barB2-right`, `.barB2-nav`, `.barB2-nav--ico` | live | Kingdom chrome | CommandBar (chrome redesign) |
| Chrome field pattern | `--chrome-bg` (CSS custom property) | live | Tokens | P3: single horizontal timber seam at `62%`; replaced 95° repeating diagonal seams |
| CommandBar identity | `.id`, `.id-crest`, `.crest`, `.crest-frame`, `.crest-shield`, `.crest-glyph`, `.id-name`, `.id-sub` | live | Kingdom chrome | CommandBar — identity block |
| CommandBar resource pill | `.pill`, `.pill-txt`, `.pill-l`, `.pill-v`, `.is-zero` | live | Kingdom chrome | CommandBar — resource pill |
| CommandBar tick | `.tick`, `.tick-l`, `.tick-v` | live | Kingdom chrome | CommandBar — tick display |
| CommandBar leave | `.leave`, `.leave-l` | live | Kingdom chrome | CommandBar — leave link |
| CommandBar nav link | `.nav-link`, `.is-on` | live | Kingdom chrome | CommandBar — nav link |
| NavBadge | `.nav-link-ico`, `.nav-gly-ico`, `.nav-badge`, `badge-glow` keyframe | live | Kingdom chrome | CommandBar — unread badge |
| SVG glyph | `.gly` | live | Kingdom chrome | Inline SVG glyph via `<use>` |
| Form field | `.field`, `.field-group`, `.field-label`, `.field-hint` | live | Shared components | Fields (from Campaign transfer form) |
| Textarea field | `textarea.field` · `.field-hint-row` | live | Shared components | Fields (textarea extension — guild handoff) |
| Section header | `.section-header` · `.section-title` · `.section-rule` · `.section-meta` | live | Shared components | Section header (guild handoff) |
| Card header | `.card-header` · `.card-header-row` · `.card-title` · `.card-subtitle` | live | Shared components | Card header (guild handoff) |
| Eyebrow | `.eyebrow` | live | Shared components | Eyebrow (guild handoff) |
| Status pill | `.status-pill`, `--home`, `--march`, `--return`, `--combat`, `--guard` | live | Army | Status pill (shared) |
| Action tag | `.action-tag`, `--attack`, `--defend` | live | Army | Action tag (shared) |
| Roster strip / unit token | `.roster-strip`, `.unit-token`, `.unit-medallion`, `.unit-tally`, `.roster-more`, `.roster-empty` | live | Army + Units | Roster / unit token |

---

## Home & Auth  _(handoff: `design_handoff_home_auth/`)_

| Component | Class(es) | Status | styles.css Section | Design Reference |
|---|---|---|---|---|
| Home chrome bar | `.home-chrome`, `.home-chrome-brand`, `.home-chrome-crest`, `.home-chrome-name`, `.home-chrome-sep`, `.home-chrome-nav`, `.home-chrome-right`, `.home-chrome-register`, `.home-chrome-login` | live | Home shell | Home & Auth |
| Auth stage + card | `.auth-stage`, `.auth-wrap`, `.auth-wrap--wide`, `.auth-body`, `.auth-crest`, `.crest-lg`, `.auth-wordmark`, `.auth-tagline`, `.auth-divider`, `.auth-form`, `.auth-btn`, `.auth-quiet`, `.auth-foot`, `.auth-foot-text`, `.auth-foot-link`, `.auth-foot-link--muted`, `.auth-instruct`, `.auth-alert`, `.auth-alert--success` | live | Auth | Home & Auth |
| Password toggle | `.password-field`, `.btn-text` | live | Shared components | Home & Auth |
| Side nav live indicator | `.nav-live`, `.nav-live-dot`, `.nav-live-n`, `.nav-live-l` | live | Home shell | Home & Auth |
| Home hero | `.home-hero`, `.home-hero-title`, `.home-hero-sub`, `.home-hero-cta`, `.home-rule`, `.home-cards`, `.home-card` | live | Home (flip card) | Home & Auth |

---

## Campaign / Army  _(handoff: `design_handoff_campaign/`)_

| Component | Class(es) | Status | styles.css Section | Design Reference |
|---|---|---|---|---|
| Quartermaster card | `.qm-card`, `.qm-stacked`, `.qm-strip`, `.qm-reserve-list`, `.qm-reserve-row`, `.qm-reserve-name`, `.qm-stacked-form` | live | Army | Quartermaster (Reserve + Transfer) |
| Transfer form grid | `.xfer-grid` | live | Army | Transfer form grid |
| Reserve pool | `.reserve-pool`, `.reserve-item`, `.reserve-name` | live | Army | Reserve |
| Legion card (at home) | `.legion-card`, `.legion-home`, `.legion-muster`, `.legion-dispatch`, `.legion-crest` | live | Army | Legion card |
| Legion empty slot | `.legion-slot` | live | Army | Empty slot card |
| In-flight campaign | `.legion-campaign`, `.timeline`, `.tl-stop`, `.cc-body`, `.cc-eta`, `.cc-foot` | live | Army | In-flight campaign |
| Strength line | `.strength-line`, `.sl-item`, `.sl-sep` (no `.sl-drain`) | live | Army | Strength line |

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
| Unit token + tally | `.unit-token`, `.unit-tally`, `.unit-medallion` | live | Units + Army | Portrait token — canonical names; `.unit-medallion` is the wooden icon container used in army view |
| Train control | `.train-ctl` | pending | Units | Inline train control |
| Lock banner | `.lock-banner` | pending | Units | Summons — lock banner |
| Toast | `.toast` | pending | Units | Toast notification |

---

## World Map  _(handoff: `design_handoff_worldmap_messages/`)_

| Component | Class(es) | Status | styles.css Section | Design Reference |
|---|---|---|---|---|
| World page layout | `.world`, `.world-main`, `.world-board`, `.world-cmd` | live | World map | WorldPage |
| Board head | `.board-head`, `.board-name`, `.board-region`, `.board-stage` | live | World map | WorldPage — board |
| Flat board | `.map-flat`, `.map-flat-corner`, `.map-flat-cols`, `.map-flat-rows`, `.map-flat-axis`, `.map-flat-grid` | live | World map | FlatBoard |
| Map tile | `.map-cell`, `.map-cell--occupied`, `.map-cell--own`, `.map-cell--clickable`, `.map-cell--selected` | live | World map | Cell |
| Map grid container | `.map-grid-container`, `.map-grid-middle` | live | World map | FlatBoard |
| Kingdom marker | `.map-marker`, `.map-marker--crest`, `.marker-crest`, `.marker-rel-dot`, `.rel-self`, `.rel-neutral` | live | World map | Marker |
| Command nav | `.cmd-nav`, `.cmd-nav-bar`, `.cmd-section-title`, `.cmd-coords`, `.cmd-minimap-wrap` | live | World map | WorldPage — command panel |
| Command detail | `.cmd-detail`, `.cmd-empty`, `.cmd-empty-rose`, `.cmd-empty-h`, `.cmd-empty-sub` | live | World map | WorldPage — detail panel |
| Find bar | `.cmd-search`, `.cmd-search-input`, `.cmd-search-btn` | live | World map | WorldPage — find bar |
| Kingdom detail panel | `.kd-panel`, `.kd-head`, `.kd-crest`, `.kd-name`, `.kd-sub`, `.kd-actions`, `.kd-action-btn`, `.kd-action-btn--attack`, `.kd-action-btn--defend`, `.kd-self-note` | live | World map | KingdomDetailPanel |

---

## Guild  _(handoff: `design_handoff_guild/`)_

| Component | Class(es) | Status | styles.css Section | Design Reference |
|---|---|---|---|---|
| Table / ledger | `.table` (full design-system version) | live | Guild | Table / ledger |
| Table identity cell | `.table-id` · `.table-id-name` · `.table-id-sub` | live | Guild | Table identity cell |
| Sortable head | `.table-sort` · `.sort-diamond` | live | Guild | Sortable head |
| Empty state | `.empty-state` (`--row`) · `.empty-state-title` · `.empty-state-hint` | live | Guild | Empty state |
| Guild crest slot | `.guild-crest` (`--sm`/`--lg`/`--empty`) · `.guild-crest-tag` | live | Guild | Guild crest slot |
| Support meter | `.meter--support` · `.meter-fill--support` · `.seal-cell` · `.seal-count` | live | Guild | Support meter |
| Role tag | `.role-tag` (`--leader`/`--officer`/`--supporter`) | live | Guild | Role tag |
| Door layout | `.doors` · `.door` · `.door-inner` · `.door-crest` · `.door-title` · `.door-text` · `.door-meta` · `.doors-seam` · `.seam-rule` · `.seam-or` | live | Guild | Door layout |
| Guild head | `.guild-head` (`--row`) · `.guild-meta` · `.guild-meta-sep` | live | Guild | Guild head |
| Standing bar | `.standing-bare` (`--foot`) · `.standing-actions` · `.standing-note` | live | Guild | Standing bar |
| Manage head | `.manage-head` · `.manage-head-body` · `.manage-head-guild` · `.manage-head-sub` · `.manage-back` | live | Guild | Manage head |
| Invite form | `.invite-form` · `.invite-row` | live | Guild | Invite form |
| Leader grid | `.manage-leader-grid` | live | Guild | Leader grid |
| Disband note | `.disband-note` | live | Guild | Disband note |
| Charter layout | `.charter-wrap` · `.charter-grid` · `.charter-form` · `.charter-actions` · `.charter-oath` | live | Guild | Charter layout |
| Founding rite | `.rite-list` · `.rite-body` · `.rite-name` · `.rite-text` · `.rite-note` | live | Guild | Founding rite |
| Roll grid | `.roll-grid` · `.charter-item` · `.charter-item-head` · `.charter-item-foot` · `.charter-lapse` | live | Guild | Roll grid |
| Viewer row | `.is-you` · `.you-mark` | live | Guild | Viewer row |
| Ledger footnote | `.ledger-footnote` | live | Guild | Ledger footnote |
