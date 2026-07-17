# Ideas
Raw thoughts. No commitment, no structure. May never happen.

- Find a name
- Spawn near a player
- daily message: the realm holds firm
- [campaing] timeline bar from left to right kingdom to target with a direction
- [skills] Design skills
- [events] Add a Journal of events — could have...
- [timers] safe timers
- [map] add distance calculator
- [winning] conquering zones for winning condition
- [army] prevent attacking if 75% of the tick has passed or
- [army] implement a safe time range
- [army] army bunker, new place like reserve to store army to avoid combat


# TODO
Decided to do. One line is fine — enough to act on.

- [game] Fix startvation formula
- [game] Fix resource forumala with a power formula and building

## Comic Lab restyle — deferred feature work (Option C)

Visual structure landed in the Comic Lab restyle; these are the data/query bits
that were stubbed or omitted. See `.handoff/comic-lab-final-four.md` while it
exists, and the per-file code comments for the exact stub points.

- [prayers] Incoming "Upon Your Realm" foreign-prayers section: add a
  `ListPrayersTargetingKingdom(me)` query (prayers others cast on me, minus my own
  self-casts; read-only, not cancellable) + render as `.realm-prayer` rows with a
  `.meter-fill--foreign` (blue) meter. Handler stub: `internal/handlers/prayers/prayers.go`.
- [prayers] Cross-kingdom targeting (perk-gated): resolve the `target_kingdom`
  signal to an ID and gate behind a `CanCastAbroad` perk; replace the locked
  `.target-perk` notice in `offeringForm` with a real target select.
  `castPrayer` already carries `TargetKingdomName` but ignores it.
- [prayers] Per-prayer cost multiplier: the offering-form total-cost readout
  uses a hardcoded `20` (`$prayer_ticks * 20` in `offeringForm`). Derive it from
  the selected prayer's `DevotionUpkeep` (datastar expression keyed on
  `$prayer_type`) so a future prayer with a different upkeep renders correctly.
- [messages] Add `envelope` and `cross` symbol defs to `web/static/sprite.svg`
  and render them inside `.message-mark--post` / `.message-mark--guild` via
  `ui.Icon`. Cosmetic — marks currently read by shape+colour alone; the glyphs
  match the Comic Lab proof. (Decree already uses the `crown` glyph.)
- [guild] Crest upload by the leader: storage + form + render `<img>` in
  `.guild-crest`; empty state = dashed ring + dim pennant. The restyle will
  render the empty-crest dashed-ring state; the upload endpoint + persistence is
  the missing piece.
- [guild] Add the guild glyphs the Comic Lab proof uses (banner, globe, gavel,
  handshake, quill, back/arrow) to `web/static/sprite.svg` and render them via
  `ui.Icon` in the guild view layer (door crests, breadcrumbs, founding-steps
  eyebrow, manage/settings kickers). Cosmetic — door crests, breadcrumbs, and
  founding steps currently read by shape + number + colour alone (crown is the only proof glyph
  already in the sprite, used on the leader role-tag + found-door crest).
- [guild] Pending-guild lapse chip on the View page: `db.Guild` (from
  `GetGuildBySlug`) has no expiry, so the pending view's `.guild-meta` strip
  omits the "Lapses …" chip that the roll ledger shows (the roll uses
  `ListPendingGuildsRow.ExpiresAt`, computed as `created_at + 7 days`). Either
  carry expiry onto `GetGuildBySlug` or derive it in the view; until then the
- [army] Add the Campaign glyphs the Comic Lab proof uses (swords, house, flag,
  chevron) to `web/static/sprite.svg` and render them via `ui.Icon` in the army
  view layer (strength lines, action tags, the muster CTA, the summary strip).
  Cosmetic — these are currently read by word + colour + number alone; the only
  proof glyphs already in the sprite are `crown` (kingdom identity) and
  `sandglass` (tick timer). Mirror of the [guild]/[messages] glyph-deferral
  entries.

# In Progress
What you're touching right now. Delete entries when done.

_(none)_
