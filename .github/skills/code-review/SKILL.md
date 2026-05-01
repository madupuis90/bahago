---
name: code-review
description: Structured Go code review for this project. Diffs local main against origin/main, reads all changed files in full, and works through issues one at a time using the project checklist. Use when you want a thorough review of recently added or modified features.
---

Read `.github/instructions/review.instructions.md` before doing anything else.

## Workflow

1. Run `git log origin/main..HEAD --oneline` to identify commits not yet on origin. Summarise the feature under review; if commits span multiple unrelated features, ask the user which to focus on.
2. Run `git diff origin/main..HEAD` to pull the full diff.
3. Read every changed file in full — do not form opinions from the diff alone.
4. Apply the full checklist from `review.instructions.md`. Check each category explicitly: SQL & Database, Go, UI / Templates, Structure & Conventions, Security.
5. Present the review in two sections:
   - **Must change** — specific issues with file and line references, ordered most impactful first (architecture and correctness before style)
   - **Questions** — ambiguities or design decisions that need the user's input
6. Work through **Must change** items one at a time. Wait for the user to resolve or dismiss each before moving to the next. Do not batch-fix.

## Principles

- Flag missing features, absent error feedback, and unclear variable names — not just rule violations
- Prefer asking over assuming when a design decision could go either way
- If a `test/ui/<feature>.md` test plan exists for any changed feature, note it and recommend running it after fixes are applied