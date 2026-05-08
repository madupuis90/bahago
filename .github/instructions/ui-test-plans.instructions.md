---
description: "Use when asked to run a UI test plan"
---

# UI Test Plans

Test plans live in `test/ui/`, one file per feature. Each file contains multiple named plans that verify specific UI behaviours in the browser.

## Test Accounts

- **mad@test.com** / `12345678` — kingdom name "Bob". This is the primary test account.

## Running a Plan

When asked to run a test plan:

1. **Do not start or stop the dev server** — the user manages the server lifecycle. If the server appears unreachable, tell the user and wait for them to start it.
2. **Reuse the existing browser page** — use the active `pageId` from context. If no page is open for the app, open one with `open_browser_page`. Do not open extra pages unless the plan explicitly requires a persistence check (see SSE notes below).
3. **Check login** — use `read_page` to check the current URL. If on `/login`, fill credentials using `click_element` and `type_in_page`, then navigate to the plan's precondition URL after login.
4. **Execute the plan regardless of initial state** — if the page state does not exactly match the precondition (e.g. some rows are non-zero when 0% is expected), run the plan steps anyway. Do not abort or adjust the plan silently. If a step fails due to unexpected initial state, report it as a FAIL with the actual value observed.
5. **Execute each step using browser tools** — use `click_element` and `type_in_page` for interactions, `read_page` to observe page state. After each step, verify the expected outcome from the page snapshot and report PASS or FAIL immediately. **For sliders:** when a step sets a value that requires multiple button clicks (e.g. clicking `+5` eight times), use `run_playwright_code` to set the slider directly via `fill('value')` + `dispatchEvent('input')` + `dispatchEvent('change')` instead — unless the step is specifically testing button behaviour.
6. **Report a summary** at the end: plan name, each step result (PASS/FAIL), and overall result. If a step fails, report the actual value observed.
7. **If anything is unclear**, do not guess — follow the plan literally or ask the user before deviating.
8. **When the plan is done**, close the browser page if it was opened by the agent.

## SSE Pages — Navigation and Assertion Rules

Pages with a persistent SSE stream (e.g. `/kingdom/allocation`) keep an HTTP connection open indefinitely. Apply these rules on all kingdom pages:

- **Navigation:** use `navigate_page` with `type="url"` to go to kingdom pages. This uses `domcontentloaded` under the hood and is not blocked by the SSE stream.
- **Persistence checks (reload):** Do **not** use a browser reload. Instead, navigate to the same URL again with `navigate_page` — this opens a fresh connection and avoids the SSE stream blocking the load.
- **Asserting after Save:** Call `read_page` after clicking Save to get the updated snapshot. If a value appears stale (a concurrent tick may have patched the page), call `read_page` again.
- **Tick-dependent assertions (e.g. "value changes after N seconds"):** The server tick interval varies — it may be 1 second in dev or several minutes. If a step waits for a value to change and it does not change within the wait window, **do not FAIL the step** — report it as SKIP with a warning: "Could not detect a tick during the wait window; tick interval may be longer than expected." Continue with the remaining steps.

## When Making Changes to a Feature

If a feature has a test plan file in `test/ui/`, run all plans in that file after the changes are complete.

- If a plan **passes**: continue normally.
- If a plan **fails**: continue but report the failure to the user with the exact step and actual value observed. Do **not** silently ignore the failure. Do **not** fix the plan to match the new behaviour unless the user explicitly instructs you to. The user decides whether the failure is a bug in the code or a plan that needs updating.

## Plan File Format

Each feature file contains **one plan** that covers the feature end-to-end as a single continuous sequence of steps. Use bold section headers inside the plan to group related steps (e.g. **Page structure**, **Validation**, **Persistence**). There is no Precondition or Teardown block.

```markdown
## Plan: <Feature> Full Flow

**Account:** mad@test.com / 12345678
**Precondition:** <URL to navigate to and any required DB/UI state before step 1>

### Steps

**<Section heading>**

1. <Action description>
   - Expected: <what should be true after this step>

2. <Action description>
   - Expected: <what should be true after this step>

**<Next section heading>**

3. <Action description>
   - Expected: <what should be true after this step>
```

## Writing New Plans

- One file per feature in `test/ui/<feature>.md`
- One plan per file — a single continuous flow that covers the full feature: happy path, validation errors, reactive UI updates, and persistence
- Use bold section headers inside the plan to separate logical groups of steps
- If the feature requires multiple accounts (e.g. guild), list all accounts at the top of the plan and indicate which account is active at each section transition with a step that says "Log out and log in as X"
- Expected outcomes must be observable (text visible, element present/absent, URL changed, value shown)
- Do not test implementation details — test what a user would see and do
