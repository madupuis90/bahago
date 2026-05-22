---
name: pickup
description: Resume from a /handoff document — read the context, brief the user, then enter plan mode to discuss and refine the work before implementation begins. Cleans up handoff files once the work is done.
argument-hint: "Optional: filename in .handoff/ to pick up (defaults to most recent)"
---

Pick up from a handoff document left by a previous session.

## Steps

1. **Find the handoff file** — list `.handoff/` at the workspace root. If an argument was passed, treat it as the filename to use. Otherwise pick the most recently modified file. If the directory is missing or empty, tell the user there is nothing to pick up and stop.

2. **Read the document in full** — understand what was in flight, what decisions were already made, what still needs to happen, and which skills are suggested.

3. **Brief the user** — present a concise summary (a few short paragraphs at most):
   - What feature or task was being worked on
   - What is already done
   - What the next session should tackle
   - Any key constraints or decisions already locked in

   Keep it tight — this is orientation, not a re-read of the document.

4. **Enter plan mode** — invoke `/plan` so the work can be discussed and refined before any code is written. Bring the handoff context into the plan so the user does not have to re-explain it.

5. **Clean up when done** — once the planned work is implemented and the user confirms the session is complete, delete all files inside `.handoff/` and tell the user the cleanup is done.

Do not delete handoff files at the start of the session or mid-implementation. If the session ends before the work is finished, leave the files in place for the next `/pickup`.
