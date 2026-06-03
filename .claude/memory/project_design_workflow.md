---
name: project-design-workflow
description: "Claude Design ↔ Claude Code workflow — how UI design handoffs work, what files to maintain, and where registries live"
metadata: 
  node_type: memory
  type: project
  originSessionId: b20e5875-6209-46f7-b514-6976883ddfe4
---

Claude Design (read-only on GitHub) produces handoff packages; Claude Code implements them and maintains shared registries.

**Flow:**
- Claude Design → Claude Code: `design_handoff_<feature>/` folder at repo root; `README.md` is authoritative
- Claude Code → Claude Design: committed files (`styles.css`, `design/components.md`, `design/naming.md`) readable by Claude Design next session
- Prompt relay: "Relay to Claude Design:" block in response when something diverges from the spec

**Registries (Claude Code is single writer):**
- `.claude/design/components.md` — component registry: class names, status (`live`/`pending`), styles.css section, Design Reference link. Update after each handoff implementation.
- `.claude/design/naming.md` — translation table (Design Reference prefix names → codebase semantic names) + naming rules

**Why:** Claude Design can't write the repo; registries prevent drift between what Claude Design specifies and what's already implemented.

**How to apply:** At the start of any unit/UI session, check `design/components.md` status to know what's already live. Apply "Registry delta" rows from handoff READMEs mechanically after implementing.
