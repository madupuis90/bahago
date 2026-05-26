---
name: feedback-subagent-restraint
description: "Use focused implementation subagents for independent, multi-file tasks; avoid exploratory or single-lookup spawns. Decision rule included."
metadata: 
  node_type: memory
  type: feedback
  originSessionId: 1ce6f6d8-2d5b-4519-ae45-755da9e81a5b
---

Be deliberate about when to spawn subagents. They are not always expensive — a focused subagent with a precise prompt on a bounded task is often *cheaper* than doing it inline when the main context is already large. The original blanket ban caused worse outcomes during the legion refactor: sequential inline work ballooned context, forced multiple compactions, and re-read files 2–3 times total.

**Why:** A subagent costs ~15-20k tokens of fixed overhead (system prompt, CLAUDE.md, instructions). But a main-context session that compacts once re-reads every file in it — paying 2x or 3x for the same content. For tasks touching 3+ large files, a focused subagent breaks even quickly and never compacts.

**Decision rule — spawn a subagent when ALL of these hold:**
1. The task is independent of other in-flight work (doesn't need output from a concurrent task)
2. It touches 3+ files, or a single large file (>300 lines)
3. The subagent can be given a precise prompt: exact file paths, what to change, interfaces/signatures already known — no exploration needed

**Do it inline when:**
- Single small file edit (<300 lines, 1 file)
- The files are already in context from earlier this session (re-reading in a subagent costs more than it saves)
- The task requires back-and-forth reasoning with the user

**General principle — separate exploration from implementation:**
- Exploration (finding symbols, understanding structure) should happen in the main context or an Explore subagent early, before implementation starts
- Implementation (editing files) should happen in focused subagents with precise prompts, keeping the main context clean
- Never mix both in the same growing context window

**Safeguards that still apply:**
- Never spawn for a single symbol lookup or file read — use `grep`/`find`/`Read` directly
- Never spawn `general-purpose` for something two file reads away
- Always tell the user before spawning so they can veto
- Never spawn agents "in parallel for thoroughness" when work is sequential

Related: [[feedback-handoff-over-reexploration]], [[user-token-cost-awareness]]
