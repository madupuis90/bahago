# Naming Conventions: Design Reference → Codebase

**Single writer: Claude Code.** Claude Design reads this file to know the canonical semantic names before specifying new components. Claude Design flags renames as deltas in handoff READMEs rather than re-documenting the full table.

> **Note:** Claude Design has been instructed to use semantic names directly in the Reference (no flavor/prefix names). If that holds, this file becomes unnecessary and can be deleted — it exists only to cover the transition from the old prefix system (`.kbtn`, `.folio`, etc.). Keep it until a full handoff arrives using purely semantic names, then reassess.

---

## Translation table

| Prototype / `reference.css` name | Codebase (semantic) name | Status | Note |
|---|---|---|---|
| `.kbtn`, `.kbtn--primary`, `.kbtn--danger`, `.kbtn--quiet`, `.kbtn--sm`, `.btn--insufficient`, `.btn--locked`, `.is-disabled` | `.btn` + same modifier suffixes | live | One button system; modifiers map 1:1 |
| `.kslider` | `.slider-*` + `.v-diamond` | live | Already in `styles.css` |
| `.folio` | `.page-header` (or feature-specific, e.g. `.unit-folio`) | pending | Bookish name → semantic; exact name chosen per feature |
| `.kfield`, `.kfield-group`, `.kfield-label`, `.kfield-hint` | `.field`, `.field-group`, `.field-label`, `.field-hint` | live | Campaign handoff renamed `.kfield*` → `.field*`; now in `styles.css` Shared components section |
| `.kattr`, `.kattr-row`, `.kattr--*` | `.kattr`, `.kattr-row`, `.kattr--*` | pending | `k` prefix acceptable — domain-flavored but unambiguous; no rename planned |
| `.kmeter`, `.kmeter-fill` | `.kmeter`, `.kmeter-fill` | pending | Same as above — keep as-is |

---

## Naming rules (apply when adding new components)

- **Strip decorative prefixes** (`k`, bookish words like `folio`) — use plain semantic names
- **Domain vocabulary is fine** — `unit`, `summon`, `portrait`, `roster`, `muster`, `legion` are real game terms, not flavor; `.unit-portrait`, `.muster-roll` are semantic
- **BEM modifier convention** — double-dash for state/variant (`.btn--primary`, `.unit-portrait--lg`), single-dash for sub-elements (`.unit-row-id`, `.muster-roll-head`)
- **Never mix `Class()` and `Classes{}`** on the same element — fold everything into one `Classes{}`
