---
name: feedback-handoff-over-reexploration
description: Use /handoff aggressively — between sessions AND within sessions when context grows large — to avoid re-reading files after compaction.
metadata: 
  node_type: memory
  type: feedback
  originSessionId: 1ce6f6d8-2d5b-4519-ae45-755da9e81a5b
---

Use `/handoff` to preserve context rather than letting compaction degrade it. Compaction is the hidden cost multiplier: it preserves summaries but drops file contents, forcing re-reads that can cost 2–3x the original read.

**Why:** The legion refactor required multiple compactions within a single session because exploration and implementation were mixed in one growing context. After each compaction, files had to be re-read. A handoff at a natural seam — even within a session — would have reset the context cleanly.

**How to apply:**

*Between sessions:*
- At the end of any session with non-trivial multi-step work, offer `/handoff` so the next session can `/pickup` cleanly.
- In a new session, if the user references prior work, check for a handoff document before exploring.

*Within a session — proactively suggest `/handoff` when:*
- A logical sub-task is fully complete (one file fully edited and tested) and more work remains
- The context has grown large (many tool calls, many files read) and the next sub-task is a fresh scope
- The user is about to start something that requires reading several new large files

*Handoff file quality matters:*
- A vague handoff ("implement the handler") forces re-exploration in the next session
- A precise handoff includes: exact file paths, function signatures, DB query names, line numbers, interface contracts between steps
- The planning session (where context is richest) should produce maximally specific handoff content — not task descriptions but implementation specs

Related: [[feedback-subagent-restraint]]
