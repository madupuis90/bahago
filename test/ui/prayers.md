# Prayers UI Test Plans

The prayers page (`/kingdom/prayers`) lets a kingdom cast active prayers that consume devotion each tick in exchange for resource production bonuses.

---

## Plan: Prayers Full Flow

**Account:** mad@test.com / 12345678
**Precondition:** Navigate to `/kingdom/prayers`. No active prayers. Devotion allocation is greater than 0% on the Allocation page.

### Steps

**Page structure**

1. Navigate to `/kingdom/prayers` and observe the page.
   - Expected: Title is "Prayers". "Active Prayers" panel shows a cast form with a Prayer dropdown, a Duration input defaulting to 8, and a "Pray" button. Below the form: "No active prayers." Available Prayers section shows a "Mana Prayer" card and a "Wood Prayer" card, each with a description, effect text, and devotion upkeep rate. No cast controls on the cards.

**Duration validation**

2. Set the duration input to `0` and click "Pray".
   - Expected: Error message: "duration must be between 1 and 48 ticks". No prayer is created. "No active prayers." remains.

3. Set the duration input to `49` and click "Pray".
   - Expected: Error message: "duration must be between 1 and 48 ticks". No prayer is created.

**Casting a prayer**

4. Select "Mana Prayer" in the dropdown, set duration to `3`, and click "Pray".
   - Expected: Error message is cleared. Active Prayers table appears with one row showing "Mana Prayer", "+20% mana production", "3 / 3", and a Cancel button. The Pray button becomes disabled.

**Prayer countdown via SSE**

5. Wait for one tick to pass.
   - Expected: Ticks Remaining column updates to "2 / 3" automatically via SSE — no page reload.

**Cancelling a prayer**

6. Click "Cancel" on the active prayer row.
   - Expected: Active Prayers section reverts to "No active prayers." Pray button becomes enabled again.

**Wood Prayer**

7. Select "Wood Prayer" in the dropdown, set duration to `3`, and click "Pray".
   - Expected: Active prayer row shows "Wood Prayer", "+20% wood production", "3 / 3", and a Cancel button.

8. Click "Cancel" on the active prayer row.
   - Expected: Active Prayers section reverts to "No active prayers."

**Prayer expiry**

9. Select any prayer, set duration to `1`, and click "Pray".
   - Expected: Active prayer row shows "1 / 1".

10. Wait for one tick to pass.
    - Expected: Active Prayers section shows "No active prayers." via SSE — the prayer expired and was removed automatically.

**Duplicate prayer type prevention**

11. Cast Mana Prayer with any duration.
    - Expected: Prayer appears as active. Pray button becomes disabled.

12. Open a second browser tab and navigate to `/kingdom/prayers`. Select "Mana Prayer" and click "Pray".
    - Expected: Error message: "Mana Prayer is already active on this kingdom". The active prayer from step 11 is unchanged.

13. Close the second tab. Click "Cancel" on the active prayer row.
    - Expected: Active Prayers section shows "No active prayers."

**Devotion enforcement**

14. Navigate to `/kingdom/allocation`. Set devotion allocation to 0% and save.
    - Expected: Devotion production shows 0/tick.

15. Return to `/kingdom/prayers`. Cast Mana Prayer with a duration long enough that the devotion stockpile will fall below 20 before the prayer expires.
    - Expected: Prayer appears as active.

16. Wait for ticks until the devotion stockpile shown in the sidebar drops below 20.
    - Expected: On the tick where the stockpile falls below 20, the prayer is auto-cancelled. Active Prayers section shows "No active prayers." via SSE.

17. Navigate to `/kingdom/allocation` and restore devotion allocation to a positive value. Save.
    - Expected: Devotion production is positive again.


### Steps

**Page structure**

1. Navigate to `/kingdom/prayers` and observe the page.
   - Expected: Title is "Prayers". "Active Prayers" section shows "No active prayers." Available prayers grid shows a "Mana Prayer" card with a description, effect text, devotion upkeep rate, a duration input defaulting to 8, a total cost hint, and a "Pray" button.

2. Observe the "Total devotion cost" hint with the duration input at its default value of 8.
   - Expected: Cost hint displays `160` devotion (8 × 20/tick).

**Duration input reactivity**

3. Change the duration input to `3`.
   - Expected: Cost hint updates to `60` devotion without a page reload.

4. Change the duration input to `24`.
   - Expected: Cost hint updates to `480` devotion.

**Duration validation**

5. Manually type `0` into the duration input and click "Pray".
   - Expected: Error message: "duration must be between 1 and 24 ticks". No prayer is created. Active Prayers section still shows "No active prayers."

6. Manually type `25` into the duration input and click "Pray".
   - Expected: Error message: "duration must be between 1 and 24 ticks". No prayer is created.

**Casting a prayer**

7. Set duration to `3` and click "Pray" on the Mana Prayer card.
   - Expected: Error message is cleared. Active Prayers table appears with one row showing "Mana Prayer", "+20% mana production", "3 / 3", and a Cancel button. The Mana Prayer card's "Pray" button becomes disabled.

**Prayer countdown via SSE**

8. Wait for one tick to pass.
   - Expected: Ticks Remaining column updates to "2 / 3" automatically via SSE — no page reload.

**Cancelling a prayer**

9. Click "Cancel" on the active prayer row.
   - Expected: Active Prayers section reverts to "No active prayers." Mana Prayer card's "Pray" button becomes enabled again.

**Prayer expiry**

10. Set duration to `1` and click "Pray" on the Mana Prayer card.
    - Expected: Active prayer row shows "1 / 1".

11. Wait for one tick to pass.
    - Expected: Active Prayers section shows "No active prayers." via SSE — the prayer expired and was removed automatically.

**Duplicate prayer prevention**

12. Cast Mana Prayer with any duration.
    - Expected: Prayer appears as active. "Pray" button is disabled.

13. In a second browser tab, navigate to `/kingdom/prayers` and attempt to cast Mana Prayer.
    - Expected: Error message: "Mana Prayer is already active on this kingdom". Active prayer from step 12 is unchanged.

14. Close the second tab. Click "Cancel" on the active prayer row.
    - Expected: Active Prayers section shows "No active prayers."

**Devotion enforcement**

15. Navigate to `/kingdom/allocation`. Set devotion allocation to 0% and save.
    - Expected: Devotion production shows 0/tick.

16. Note the current devotion stockpile. Return to `/kingdom/prayers` and cast Mana Prayer with a duration long enough that the stockpile will not drop below 20 for several ticks, but devotion will not regenerate (production is 0).
    - Expected: Prayer appears as active.

17. Wait for ticks until the devotion stockpile shown in the sidebar drops below 20.
    - Expected: On the tick where the stockpile falls below 20, the prayer is auto-cancelled. Active Prayers section shows "No active prayers." via SSE.

18. Navigate to `/kingdom/allocation` and restore devotion allocation to a positive value. Save.
    - Expected: Devotion production is positive again.


---

## Plan 1: Cast and cancel a prayer

**Account:** any kingdom with devotion allocation > 0
**Precondition:** Navigate to `/kingdom/prayers`. No active prayers.

### Steps

1. Navigate to `/kingdom/prayers` and observe the page.
   - Expected: Title is "Prayers". "Active Prayers" section shows "No active prayers." Available prayers grid shows a "Mana Prayer" card with a duration input, total cost hint, and "Pray" button.

2. Observe the "Total devotion cost" hint with the duration input at its default value of 8.
   - Expected: Cost hint displays `160` devotion (8 ticks × 20/tick). Value is reactive — updates without page reload as the input changes.

3. Change the duration input to `3`.
   - Expected: Cost hint updates to `60` devotion without a page reload.

4. Click "Pray" on the Mana Prayer card.
   - Expected: Active Prayers table appears with one row showing "Mana Prayer", "+20% mana production", "3 / 3", and a Cancel button. The Mana Prayer card's "Pray" button becomes disabled.

5. Wait for one tick to pass.
   - Expected: Ticks Remaining column updates to "2 / 3" automatically via SSE — no page reload.

6. Click "Cancel" on the active prayer row.
   - Expected: Active Prayers section reverts to "No active prayers." Mana Prayer card's "Pray" button becomes enabled again.

---

## Plan 2: Prayer expires naturally

**Account:** any kingdom with devotion allocation > 0
**Precondition:** Navigate to `/kingdom/prayers`. No active prayers.

### Steps

1. Set duration to `1` and click "Pray" on Mana Prayer.
   - Expected: Active prayer row shows "1 / 1".

2. Wait for one tick to pass.
   - Expected: Active Prayers section shows "No active prayers." via SSE refresh — the prayer expired and was removed automatically.

---

## Plan 3: Prayer auto-cancelled when devotion stockpile is too low

**Account:** any kingdom
**Precondition:** Navigate to `/kingdom/prayers`. No active prayers. Note the current devotion stockpile.

### Steps

1. Set devotion allocation to 0% on the Allocation page and save.
   - Expected: Devotion production drops to 0/tick.

2. Return to `/kingdom/prayers`. Cast Mana Prayer with a long enough duration that the stockpile will fall below the upkeep cost (20) before the prayer expires.
   - Expected: Prayer appears as active.

3. Wait for ticks until the devotion stockpile shown in the sidebar drops below 20.
   - Expected: On the tick where the stockpile is below 20, the prayer is auto-cancelled. Active Prayers section shows "No active prayers." via SSE.

---

## Plan 4: Duplicate prayer type prevented

**Account:** any kingdom with devotion allocation > 0
**Precondition:** Navigate to `/kingdom/prayers`. No active prayers.

### Steps

1. Cast Mana Prayer with any duration.
   - Expected: Active prayer row appears. Mana Prayer card "Pray" button is disabled.

2. Open a second browser tab and navigate to `/kingdom/prayers`.
   - Expected: Active prayer row is visible and Mana Prayer card "Pray" button is disabled.

3. In the second tab, attempt to cast Mana Prayer by sending the request directly (e.g. with the duration field forced to a value).
   - Expected: Error message: "Mana Prayer is already active on this kingdom".

---

## Plan 5: Duration validation

**Account:** any kingdom
**Precondition:** Navigate to `/kingdom/prayers`.

### Steps

1. Manually type `0` into the duration input and click "Pray".
   - Expected: Error message: "duration must be between 1 and 24 ticks". No prayer is created.

2. Manually type `25` into the duration input and click "Pray".
   - Expected: Error message: "duration must be between 1 and 24 ticks". No prayer is created.
