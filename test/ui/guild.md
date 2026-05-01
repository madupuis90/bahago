# Guild UI Test Plans

The guild system lets kingdoms form guilds through a proposal-and-support phase, then recruit members once active. Plans are ordered to build on each other's end state; each can also be run in isolation if the DB is seeded to match its precondition.

## Test Accounts

All accounts use password `password`.

| Handle | Email | Kingdom |
|--------|-------|---------|
| A (leader) | seed1@dev.local | Kingdom1 |
| B (supporter) | seed2@dev.local | Kingdom2 |
| C (supporter) | seed3@dev.local | Kingdom3 |
| D (applicant) | seed4@dev.local | Kingdom4 |
| E (applicant) | seed5@dev.local | Kingdom5 |

> **Note:** Accounts A–E must have kingdoms and no existing guild memberships. If any are already in a guild, remove that membership via the manage page or DB before running.

---

## Plan 1: Create Guild Proposal

**Account:** seed1@dev.local (Kingdom1)
**Precondition:** Navigate to `/guild`. Kingdom1 has no guild membership.
**Teardown:** None — end state feeds Plan 2.

### Steps

1. Navigate to `/guild`.
   - Expected: Page shows "You are not a member of any guild." and a "Create a Guild" button.

2. Click "Create a Guild".
   - Expected: Navigated to `/guild/new`. Form shows Name, Description fields and a Create button.

3. Submit the form with Name = "Test Guild Alpha" and Description = "A test guild.".
   - Expected: Redirected to the guild's view page at `/guild/test-guild-alpha`. Status shows "Pending". Kingdom1 is listed as the Applicant.

4. Observe the action buttons.
   - Expected: "Cancel Proposal" button is visible. No "Request to Join" button.

---

## Plan 2: Support a Pending Guild (2 Supporters)

**Account:** seed2@dev.local (Kingdom2), then seed3@dev.local (Kingdom3)
**Precondition:** Plan 1 complete — `/guild/test-guild-alpha` exists with status Pending and 1 supporter needed (needs 3 total to activate; 1 applicant already counts, so 2 more supporters needed).
**Teardown:** None — end state feeds Plan 3.

### Steps

1. Log in as seed2@dev.local. Navigate to `/guild/test-guild-alpha`.
   - Expected: Guild page shows status "Pending". "Pledge Support" button is visible.

2. Click "Pledge Support".
   - Expected: Kingdom2 appears in the member list as Supporter. Supporter count increases.

3. Log out and log in as seed3@dev.local. Navigate to `/guild/test-guild-alpha`.
   - Expected: Guild page shows Kingdom2 listed as Supporter. "Pledge Support" button is visible for Kingdom3.

4. Click "Pledge Support".
   - Expected: Kingdom3 appears as Supporter. Guild status changes to "Active". Applicant (Kingdom1) role changes to "Leader".

5. Observe the member list.
   - Expected: Kingdom1 shown as Leader, Kingdom2 and Kingdom3 shown as Member. No "Pending" status visible.

---

## Plan 3: Withdraw Support (Before Activation)

**Account:** seed2@dev.local (Kingdom2)
**Precondition:** A second pending guild exists — repeat Plan 1 with a different name (e.g. "Test Guild Beta") using seed6@dev.local as applicant, seed2@dev.local has pledged support but the guild has not yet activated.
**Teardown:** None.

### Steps

1. Log in as seed2@dev.local. Navigate to the pending guild's page.
   - Expected: Kingdom2 is listed as Supporter. "Withdraw Support" button is visible.

2. Click "Withdraw Support".
   - Expected: Kingdom2 is removed from the member list. "Pledge Support" button reappears.

---

## Plan 4: Cancel a Guild Proposal

**Account:** seed6@dev.local (applicant/leader of pending guild from Plan 3)
**Precondition:** "Test Guild Beta" is still pending (Plan 3 left it without enough supporters to activate).
**Teardown:** None.

### Steps

1. Log in as seed6@dev.local. Navigate to `/guild/test-guild-beta`.
   - Expected: Status is "Pending". "Cancel Proposal" button is visible.

2. Click "Cancel Proposal".
   - Expected: Redirected to `/guild`. The guild `/guild/test-guild-beta` no longer exists (navigating there returns a 404 or "not found" page).

---

## Plan 5: Request to Join an Active Guild

**Account:** seed4@dev.local (Kingdom4), then seed5@dev.local (Kingdom5)
**Precondition:** "Test Guild Alpha" is active (Plan 2 complete). Kingdom4 and Kingdom5 have no guild membership.
**Teardown:** None — end state feeds Plan 6.

### Steps

1. Log in as seed4@dev.local. Navigate to `/guild/test-guild-alpha`.
   - Expected: Guild is active. "Request to Join" button is visible.

2. Click "Request to Join".
   - Expected: Button changes to "Cancel Request" (or a pending message appears). Kingdom4 no longer sees "Request to Join".

3. Log out and log in as seed5@dev.local. Navigate to `/guild/test-guild-alpha`.
   - Expected: "Request to Join" button is visible for Kingdom5.

4. Click "Request to Join".
   - Expected: Same as step 2 — Kingdom5's request is pending.

---

## Plan 6: Cancel a Join Request

**Account:** seed5@dev.local (Kingdom5)
**Precondition:** Plan 5 complete — Kingdom5 has a pending join request for Test Guild Alpha.
**Teardown:** None.

### Steps

1. Log in as seed5@dev.local. Navigate to `/guild/test-guild-alpha`.
   - Expected: "Cancel Request" button (or equivalent) is visible.

2. Click "Cancel Request".
   - Expected: Button changes back to "Request to Join". Kingdom5's request is removed.

---

## Plan 7: Approve and Reject Join Requests

**Account:** seed1@dev.local (Kingdom1, guild leader) and seed4@dev.local (Kingdom4)
**Precondition:** Plan 5 complete — Kingdom4 has a pending join request. Kingdom5's request was cancelled in Plan 6.
**Teardown:** None — end state feeds Plans 8–11.

### Steps

1. Log in as seed1@dev.local. Navigate to `/guild/test-guild-alpha/manage`.
   - Expected: "Pending Requests" section shows Kingdom4. Approve and Reject buttons visible.

2. Click "Reject" next to Kingdom4.
   - Expected: Kingdom4 disappears from the Pending Requests list. Kingdom4 is not in the Members list.

3. Log out and log in as seed4@dev.local. Navigate to `/guild/test-guild-alpha`.
   - Expected: "Request to Join" button is visible again (request was rejected).

4. Click "Request to Join" again as Kingdom4.
   - Expected: Request is pending.

5. Log out and log in as seed1@dev.local. Navigate to `/guild/test-guild-alpha/manage`.
   - Expected: Kingdom4 appears in Pending Requests.

6. Click "Approve" next to Kingdom4.
   - Expected: Kingdom4 disappears from Pending Requests and appears in the Members list with role "Member".

---

## Plan 8: Promote and Demote an Officer

**Account:** seed1@dev.local (Kingdom1, leader)
**Precondition:** Plan 7 complete — Kingdom4 is a Member of Test Guild Alpha.
**Teardown:** Demote Kingdom4 back to Member (step 3) to restore clean state.

### Steps

1. Log in as seed1@dev.local. Navigate to `/guild/test-guild-alpha/manage`.
   - Expected: Kingdom4 is listed as Member. "Promote" button is visible next to Kingdom4.

2. Click "Promote" next to Kingdom4.
   - Expected: Kingdom4's role changes to "Officer". "Demote" button now visible. "Promote" button gone.

3. Click "Demote" next to Kingdom4.
   - Expected: Kingdom4's role changes back to "Member". "Promote" button reappears.

---

## Plan 9: Remove a Member

**Account:** seed1@dev.local (Kingdom1, leader)
**Precondition:** Plan 7 complete — Kingdom4 is a Member of Test Guild Alpha.
**Teardown:** None.

### Steps

1. Log in as seed1@dev.local. Navigate to `/guild/test-guild-alpha/manage`.
   - Expected: Kingdom4 is in the Members list with a "Remove" button.

2. Click "Remove" next to Kingdom4.
   - Expected: Kingdom4 disappears from the Members list.

3. Log out and log in as seed4@dev.local. Navigate to `/guild/test-guild-alpha`.
   - Expected: "Request to Join" button is visible again (Kingdom4 is no longer a member).

---

## Plan 10: Leave a Guild

**Account:** seed2@dev.local (Kingdom2, Member)
**Precondition:** Test Guild Alpha is active with Kingdom2 as a Member.
**Teardown:** None.

### Steps

1. Log in as seed2@dev.local. Navigate to `/guild/test-guild-alpha`.
   - Expected: Kingdom2 is shown as a Member. "Leave Guild" button is visible.

2. Click "Leave Guild".
   - Expected: Redirected to `/guild`. Kingdom2 is no longer listed as a member when navigating back to `/guild/test-guild-alpha`.

---

## Plan 11: Transfer Leadership

**Account:** seed1@dev.local (Kingdom1, leader), viewing Kingdom3 as target
**Precondition:** Test Guild Alpha is active with Kingdom1 as Leader and Kingdom3 as Member.
**Teardown:** Transfer leadership back to Kingdom1 after the plan.

### Steps

1. Log in as seed1@dev.local. Navigate to `/guild/test-guild-alpha/manage`.
   - Expected: Transfer Leadership section is visible with a member selector.

2. Select "Kingdom3" in the transfer selector and confirm.
   - Expected: Redirected to the guild view page. Kingdom3 is now shown as Leader. Kingdom1 is shown as Member.

3. Log out and log in as seed3@dev.local. Navigate to `/guild/test-guild-alpha/manage`.
   - Expected: Kingdom3 can access the manage page. Transfer Leadership section is visible.

4. Transfer leadership back to Kingdom1 (select Kingdom1 in the selector).
   - Expected: Kingdom1 is Leader again.

---

## Plan 12: Disband a Guild

**Account:** seed1@dev.local (Kingdom1, leader)
**Precondition:** Test Guild Alpha is active and Kingdom1 is the Leader. Use a throwaway guild for this plan (create "Test Guild Gamma" via Plan 1 steps with seed7@dev.local, activate it with seed8 and seed9 as supporters).
**Teardown:** None — the guild is deleted.

### Steps

1. Log in as seed7@dev.local. Navigate to the throwaway guild's manage page.
   - Expected: "Disband Guild" button is visible.

2. Click "Disband Guild".
   - Expected: Redirected to `/guild`. The guild no longer appears in the guild list at `/guild/list`.

---

## Plan 13: Duplicate Guild Name

**Account:** seed1@dev.local
**Precondition:** "Test Guild Alpha" already exists (active).
**Teardown:** None.

### Steps

1. Log in as seed1@dev.local. Navigate to `/guild/new`.
   - Expected: Create form is visible.

2. Submit the form with Name = "Test Guild Alpha" (exact same name).
   - Expected: Error message "a guild with this name already exists" appears. No redirect.

---

## Plan 14: Cannot Join Two Guilds

**Account:** seed2@dev.local (already a member of Test Guild Alpha)
**Precondition:** Test Guild Alpha is active and Kingdom2 is a Member.
**Teardown:** None.

### Steps

1. Log in as seed2@dev.local. Navigate to `/guild/list`.
   - Expected: Guild list is visible.

2. Navigate to any *other* active guild's page (create a second guild first if needed).
   - Expected: "Request to Join" button is NOT visible. Instead a message indicates Kingdom2 is already committed to a guild (or the button is absent entirely).

---

## Plan 15: Full Guild Cap (20 Members)

**Account:** seed1@dev.local (leader of a guild with 19 active members)
**Precondition:** A guild exists with exactly 19 active members (leader + 18 members). One pending join request exists.
**Teardown:** None.

### Steps

1. Log in as seed1@dev.local. Navigate to the guild's manage page.
   - Expected: 19 members listed. One pending request visible.

2. Click "Approve" on the pending request.
   - Expected: The applicant is approved and now appears in the member list as the 20th member. No error.

3. Have a 21st kingdom submit a join request (log in as that kingdom, request to join).
   - Expected: The request succeeds (pending approval).

4. Log back in as seed1@dev.local. Navigate to the manage page and click "Approve" on the 21st request.
   - Expected: Error message "guild is full" appears. The applicant remains in the pending list.
