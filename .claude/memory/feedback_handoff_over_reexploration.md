---
name: feedback-handoff-over-reexploration
description: Prefer /handoff between sessions over re-exploring the codebase from scratch in a new conversation.
metadata: 
  node_type: memory
  type: feedback
  originSessionId: 1ce6f6d8-2d5b-4519-ae45-755da9e81a5b
---

When work spans multiple sessions, use `/handoff` to compact context for the next session rather than letting the next session re-discover everything via fresh exploration.

**Why:** Fresh exploration in a new session re-reads files, re-greps for symbols, and re-derives context that was already established — this is one of the patterns that contributed to the user's 93% subagent-heavy quota burn. A handoff document is a one-time write that saves repeated discovery cost.

**How to apply:**
- Toward the end of a session involving non-trivial multi-step work, offer to run `/handoff` so the next session can `/pickup` cleanly.
- In a new session, if the user references prior work, check for a handoff document before exploring.

Related: [[feedback-subagent-restraint]]
