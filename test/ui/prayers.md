# Prayers UI Test Plans

The prayers page (`/kingdom/prayers`) lets a kingdom cast active prayers that consume devotion each tick in exchange for resource production bonuses.

---

## Plan: Prayers Full Flow

**Account:** dev1@test.com  
**Precondition:** Navigate to `/kingdom/prayers`. No active prayers. Devotion allocation is greater than 0% on the Allocation page.

### Steps

**Page structure**

1. Navigate to `/kingdom/prayers` and observe the page.
   - Expected: Title is "Prayers". "Active Prayers" section shows "No active prayers." Available prayers grid shows a "Mana Prayer" card with a description, effect text, devotion upkeep rate, a duration input defaulting to 8, a total cost hint, and a "Pray" button.

**Cost hint reactivity**

2. Observe the "Total devotion cost" hint with the duration input at its default value of 8.
   - Expected: Cost hint displays `160` devotion (8 ticks × 20/tick).

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

16. Note the current devotion stockpile. Return to `/kingdom/prayers` and cast Mana Prayer with a duration long enough that the stockpile will drop below 20 before the prayer expires.
    - Expected: Prayer appears as active.

17. Wait for ticks until the devotion stockpile shown in the sidebar drops below 20.
    - Expected: On the tick where the stockpile falls below 20, the prayer is auto-cancelled. Active Prayers section shows "No active prayers." via SSE.

18. Navigate to `/kingdom/allocation` and restore devotion allocation to a positive value. Save.
    - Expected: Devotion production is positive again.
