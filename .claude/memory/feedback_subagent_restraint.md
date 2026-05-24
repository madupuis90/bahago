---
name: feedback-subagent-restraint
description: "Default to direct tools (grep, find, Read) instead of spawning subagents. User was burned by 93% of token usage going to subagent-heavy sessions."
metadata: 
  node_type: memory
  type: feedback
  originSessionId: 1ce6f6d8-2d5b-4519-ae45-755da9e81a5b
---

Default to direct tools over spawning subagents. The user did not realize subagents were being spawned and got billed heavily for it.

**Why:** A `/usage` summary showed 93% of quota was consumed by subagent-heavy sessions. Each Agent spawn re-sends the full system prompt, tool catalog, and all CLAUDE.md/instructions files (~15-20k tokens of overhead before work begins), the subagent re-reads files I could already read directly, and its final report comes back as tokens in my context too — paying twice for the same work. A single Explore call to find one symbol can cost 20-50x more than `grep -rn`.

**How to apply:**
- For finding a known symbol, file, or string: use `grep`/`find` via Bash, or `Read` for a known path. Never spawn Explore for a single lookup.
- Only spawn `Explore` when the search genuinely spans the codebase with ≥3 queries or multiple naming variations to try.
- Skip `Plan` for mechanical refactors and small changes — only use it for genuine architectural decisions the user wants help thinking through.
- **During large refactors: do NOT parallelize with multiple subagents.** Each agent cold-starts, re-reads the same files the main context already has, and multiplies the token cost. Do the refactor sequentially in the main context, file by file, using the file list built up from the initial grep/find. One read of a file in the main context is always cheaper than an agent re-reading it.
- Never spawn agents "in parallel for thoroughness." Only when work is truly independent and would otherwise serialize.
- Never spawn `general-purpose` for something that's two file reads away.
- When unsure whether a task warrants a subagent, do it directly first; escalate only if the direct path balloons.
- If a subagent really is the right call, tell the user before spawning so they can veto.

Related: [[feedback-handoff-over-reexploration]]
