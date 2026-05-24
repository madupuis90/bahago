---
name: user-token-cost-awareness
description: User is cost-sensitive about token usage and wants visibility into expensive operations (especially subagent spawns) before they happen.
metadata: 
  node_type: memory
  type: user
  originSessionId: 1ce6f6d8-2d5b-4519-ae45-755da9e81a5b
---

The user is on a quota-limited plan and is sensitive to token cost. They were surprised to learn that subagent spawns were happening at all — they did not realize this was a hidden expense.

**How to apply:**
- Be transparent about expensive operations before doing them — particularly subagent spawns, large file reads, or broad searches.
- If a task could be done cheaply or expensively, mention the trade-off and let the user choose.
- Default to the cheaper path.
