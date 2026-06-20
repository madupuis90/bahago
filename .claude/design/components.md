# Component Registry

**Single writer: Claude Code.** Claude Design reads this file at the start of each session to know what is already implemented and how components are named in the codebase. Claude Design supplies "Registry delta" sections in each handoff with the exact rows to add or update — applying them is mechanical.

**Status values:** `live` — in `styles.css` and in use · `pending` — handoff received, not yet implemented · `in-progress` — partially implemented

---

## Shared / cross-feature

| Component | Class(es) | Status | styles.css Section | Design Reference |
|---|---|---|---|---|
| Button | `.btn`, `.btn--primary`, `.btn--accent`, `.btn--danger`, `.btn--lg`, `.btn--sm`, `.btn--quiet`, `.btn--step5`; states `.is-insufficient`, `.is-locked`, `.is-disabled` (legacy aliases `.btn--insufficient`, `.btn--locked` kept) | live · **reskinned** (Comic Lab) | Shared components | Button system |
| Card (surface) | `.card` · `.card-inner` (`.is-lit`/`.is-frost`/`.is-framed` now no-op aliases) | live · **reskinned** (Comic Lab) | Shared components | Card |
| Card tab | `.card-tab` (`--red`/`--green`/`--blue`/`--yellow`; `--wood` = alias of `--blue`) | live · **reskinned** (Comic Lab) | Shared components | Card |
| Slider | `.slider-*`, `.v-diamond`; `.slider-fill` reads `--slider-fill` (default `--green`) — tintable per use-site | live · **reskinned** (Comic Lab) | Allocation | Slider |
| Gem / resource icon | `.gem`, `.gem-<id>` (tree, mountain, wheat, flame, sun, star) — flat `--gem-*`, ink ring, white glyph | live · **reskinned** (Comic Lab) | Kingdom chrome | Resource gems |
| Sprite icon / shield | `.icon` (`--active`=red/`--dim`), `.shield` (`--active`/`--dim`), `.gly` (bare line-art) | live · **reskinned** (Comic Lab) | Kingdom chrome | Sprite icon |
| Alert | `.alert--error`, `.alert--success` | live · **reskinned** (Comic Lab) | Shared components | Alert |
| Tokens | `--ink`, `--shadow-ink`/`-sm`/`-xs`, `--red`/`--blue`/`--green`/`--yellow`, `--card`/`--card-2`, `--paper`/`--paper-2`/`--dot`, `--page-bg`, `--accent` (→blue); fonts `--font-display` (Lilita One), `--font-body`/`--font-ui` (Nunito) | live · **reskinned** (Comic Lab) | Tokens | Foundation — design tokens |
| CommandBar chrome | `.bar`, `.barB2` (red bar, ink border), `.barB2-info`, `.barB2-res`, `.barB2-right`, `.barB2-nav`, `.barB2-nav--ico` | live · **reskinned** (Comic Lab) | Kingdom chrome | CommandBar (chrome redesign) |
| Chrome field pattern | `--chrome-bg` (CSS custom property) | live | Tokens | P3: single horizontal timber seam at `62%`; replaced 95° repeating diagonal seams |
| CommandBar identity | `.id`, `.id-crest`, `.crest`, `.crest-frame`, `.crest-shield`, `.crest-glyph`, `.id-name` (Lilita), `.id-sub` (Nunito caps) | live · **reskinned** (Comic Lab) | Kingdom chrome | CommandBar — identity block |
| CommandBar resource pill | `.pill` (cream, ink ring), `.pill-txt`, `.pill-l`, `.pill-v`, `.is-zero`, `.pill--mini` | live · **reskinned** (Comic Lab) | Kingdom chrome | CommandBar — resource pill |
| CommandBar tick | `.tick` (cream, ink hourglass), `.tick-l`, `.tick-v` | live · **reskinned** (Comic Lab) | Kingdom chrome | CommandBar — tick display |
| CommandBar leave | `.leave` (gold comic), `.leave-l` | live · **reskinned** (Comic Lab) | Kingdom chrome | CommandBar — leave link |
| CommandBar nav link | `.nav-link`, `.is-on` (yellow + ink, scoped under `.barB2-nav`) | live · **reskinned** (Comic Lab) | Kingdom chrome | CommandBar — nav link |
| NavBadge | `.nav-link-ico`, `.nav-gly-ico`, `.nav-badge`, `badge-glow` keyframe | live · **reskinned** (Comic Lab) | Kingdom chrome | CommandBar — unread badge |
| SVG glyph | `.gly` | live | Kingdom chrome | Inline SVG glyph via `<use>` |
| Form field | `.field`, `.field--num`, `.field--area`, `.select`, `.field-group`, `.field-label`, `.field-hint`, `.field-hint.is-invalid`, `.field-affix`, `.field-affix--trail`, `.field-lead`, `.field-trail`, `.check` | live · **reskinned** (Comic Lab) | Shared components | Fields (from Campaign transfer form) |
| Textarea field | `textarea.field` · `.field-hint-row` | live | Shared components | Fields (textarea extension — guild handoff) |
| Section header | `.section-header` · `.section-title` · `.section-rule` · `.section-meta` | live · **reskinned** (Comic Lab) | Shared components | Section header (guild handoff) |
| Card header | `.card-header` · `.card-header-row` · `.card-title` · `.card-subtitle` · `.card-flavour` | live · **reskinned** (Comic Lab) | Shared components | Card header |
| Eyebrow | `.eyebrow` | live · **reskinned** (Comic Lab) | Shared components | Eyebrow (guild handoff) |
| Page header | `.page-header` · `.page-header-kicker` · `.page-header-title` · `.page-header-sub` (left-aligned chunky red plate, rotated; legacy `.page-header h1` covered) | live · **reskinned** (Comic Lab) | Page composition (`41-kingdom-overview.css`) | Page title (Design Reference §3) |
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

## Unit system  _(handoff: `design_handoff_unit/`, re-skin: `handoff/units/`)_

| Component | Class(es) | Status | styles.css Section | Design Reference |
|---|---|---|---|---|
| Unit portrait | `.unit-portrait` (`--lg`/`--sm`/`--empty`/`--unit`/`--summon`) · `.unit-token` · `.unit-tally` | live · **reskinned** (Comic Lab) | Units | Unit portrait |
| Attribute chips | `.attribute` (`--offense`/`--ward`/`--arcane`/`--faith`/`--drawback`) · `.attribute-row` | live · **reskinned** (Comic Lab) | Units | Attribute chips _(was `.kattr`)_ |
| Roster — roll | `.muster-roll` · `.muster-roll-head` · `.muster-roll-col` (`--r`) · `.unit-row` · `.muster-roll-foot` | live · **reskinned** (Comic Lab) | Units | Roster — Roll |
| Roster gallery | `.muster-gallery`, `.unit-card` | pending | Units | Roster — Gallery |
| Tick meter | `.meter` · `.meter-top`/`-name`/`-qty`/`-eta`/`-track`/`-fill` · `.meter-notches`/`.meter-notch`; `.meter-fill` **brass default** + `.meter-fill--support` (green) / `--foreign` (steel) / `--danger` (red) | live · **reskinned** (Comic Lab) · **moved to `20-shared.css`** | Shared components (Meter) | Tick meter _(was `.kmeter`, Units)_ |
| Locked state | `.is-locked` · `.lock-banner` · `.unit-lock-note` | live · **reskinned** (Comic Lab) | Units | Locked / unavailable unit |
| Standing-host summary | `.units-summary` · `.units-summary-stat` · `.units-summary-label`/`-num`/`-sub` | live · **reskinned** (Comic Lab) | Units | Host summary strip _(was `.host-summary`)_ |
| Training slots | `.train-slots` · `.train-slot` (`.is-active`/`.is-idle`) · `.slot-gauge` · `.slot-gauge-dots` · `.slot-gauge-dot` (`.is-on`) · `.train-ctl` | live · **reskinned** (Comic Lab) | Units | Training grounds — slot model |
| Stat pill | `.stat-pill` (`--drain`/`--short`) · `.pill-res` | live · **reskinned** (Comic Lab) | Units | Stat pills |
| Unit token + tally | `.unit-token`, `.unit-tally`, `.unit-medallion` | live | Units + Army | Portrait token — canonical names; `.unit-medallion` is the wooden icon container used in army view |
| Train control | `.train-ctl` · `.stepper` · `.step-btn` (`--l`/`--r`) · `.count-box` (replaces `.train-count`) | live · **reskinned** (Comic Lab) | `20-shared.css` (Units page markup) | Inline train control |
| Lock banner | `.lock-banner` | live · **reskinned** (Comic Lab) | Units | Summons — lock banner |
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

## Buildings  _(handoff: `design_handoff_buildings_v2/`)_

_Replaces old `.buildings-grid` / `.building-card` flat-grid and `.construction-banner` stub — none were registered._

| Component | Class(es) | Status | styles.css Section | Design Reference |
|---|---|---|---|---|
| Page layout | `.builds`, `.builds-stage`, `.tree-scroll`, `.tree-wrap` | live | Buildings | Page wrapper / stage |
| Construction banner | `.build-banner`, `.build-banner.is-idle`, `.build-banner-gem`, `.build-banner-body`, `.build-banner-idle-title`, `.build-banner-idle-text` | live | Buildings | Construction banner |
| Tree (Lineage variant) | `.tree--lineage` + node state modifiers `.is-locked`, `.is-unlocked`, `.is-maxed`, `.is-selected`, `.is-building` | live | Buildings | Lineage variant |
| Tree node | `.node`, `.node-top`, `.node-lock`, `.node-name`, `.node-count`, `.node-building-ring`, `.node-medallion` | live | Buildings | Tree node |
| Pip meter | `.pips`, `.pip`, `.pip.on` | live | Buildings | Pip meter |
| Tree connectors (SVG) | `.tree-links`, `.tree-link`, `.tree-link.is-dim`, `.tree-joint`, `.tree-joint.is-dim` | live | Buildings | Tree connectors |
| Detail panel (empty) | `.detail`, `.detail-empty`, `.detail-empty-title`, `.detail-empty-text` | live | Buildings | Detail panel — empty state |
| Detail panel (head) | `.detail-head`, `.detail-title`, `.detail-tally`, `.detail-flavour`, `.detail-rule` | live | Buildings | Detail panel — head |
| Detail spec rows | `.detail-spec`, `.spec-row`, `.spec-label`, `.spec-val` | live | Buildings | Detail panel — spec |
| Cost chips | `.cost-chips`, `.cost-chip`, `.cost-chip.is-short` | live | Buildings | Detail panel — cost |
| Production bonus | `.bonus-val`, `.bonus-total`, `.bonus-none` | live | Buildings | Detail panel — bonus |
| Prereq list | `.req-list`, `.req-item`, `.req-mark`, `.req-mark.met`, `.req-mark.unmet` | live | Buildings | Detail panel — prereqs |
| Detail footer | `.detail-foot`, `.detail-note`, `.detail-note.is-warn` | live | Buildings | Detail panel — footer |

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

---

## Allocation  _(re-skin: `handoff/allocation/`)_

| Component | Class(es) | Status | styles.css Section | Design Reference |
|---|---|---|---|---|
| Muster summary bar | `.allocation-bar` · `.allocation-legend` · `.allocation-key` · `.allocation-dot` · `.allocation-{wood,stone,food,mana,devotion,knowledge,idle}` | live · **reskinned** (Comic Lab) | Allocation | Muster summary bar |
| Roster table | `.alloc-grid` · `.alloc-head` · `.alloc-row` (+ resource modifier `--{wood,stone,food,mana,devotion,knowledge}`) · `.alloc-col-header` (`--right`) · `.alloc-role-name` · `.alloc-role-res` · `.alloc-share` | live · **reskinned** (Comic Lab) | Allocation | Roster table |
| Net cell | `.alloc-net-cell` (`.is-pending`) · `.alloc-net` (`.neg`/`.zero`) · `.alloc-rate` (`.is-pending`) | live · **reskinned** (Comic Lab) | Allocation | Net cell |
| Idle row | `.idle-gem` · `.alloc-row.idle .alloc-assign-note` | live · **reskinned** (Comic Lab) | Allocation | Idle row |
| Footer / decree | `.alloc-foot` · `.alloc-total*` (`.over`) · `.alloc-decree` · `.alloc-error` · `.alloc-alarm` *(opt)* | live · **reskinned** (Comic Lab) | Allocation | Footer / decree |
| Resource glyphs | `#res-wood`/`-stone`/`-food`/`-mana`/`-devotion`/`-knowledge` (redrawn ligne-claire: tree · mountain · **carrot** · **drop** · sun · star); rendered on all resource gems via `ui.ResourceGem` / `ui.ResourceGlyph` (gem-id `tree/mountain/wheat/flame/sun/star` → res-key `wood/stone/food/mana/devotion/knowledge`). Chrome pills, allocation roster, units stat pills, and building cost chips / banner all use `#res-*` now (legacy `g-*` inline + `shield-*` heraldic resource art no longer used for gems). | live · **reskinned** (Comic Lab) | `sprite.svg` + `internal/ui/icons.go` | Resource glyphs |
