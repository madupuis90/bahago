---
name: user-token-cost-awareness
description: User is cost-sensitive; surface expensive operations before doing them. "Cheaper" means lower total cost, not always "no subagents."
metadata: 
  node_type: memory
  type: user
  originSessionId: 1ce6f6d8-2d5b-4519-ae45-755da9e81a5b
---

The user is on a quota-limited plan and is sensitive to token cost. They were surprised to learn that subagent spawns were happening at all — they did not realize this was a hidden expense.

**How to apply:**
- Be transparent about expensive operations before doing them — particularly subagent spawns, large file reads, or broad searches.
- If a task could be done cheaply or expensively, mention the trade-off and let the user choose.
- "Default to the cheaper path" means lower *total* cost, not always "do everything inline." A focused subagent on a large bounded task is often cheaper than inline work that compacts and re-reads. See [[feedback-subagent-restraint]] for the decision rule.
- The most expensive pattern is: inline sequential work on many large files → compaction → re-reads → repeat. Avoid this by using handoffs and focused subagents at the right moments.
