# Allocation UI Test Plans

The allocation page (`/kingdom/allocation`) lets a player distribute their population across six roles. Percentages must sum to ≤ 100; the remainder is shown as Idle. Changes take effect immediately in the UI (reactive signals) but only persist after clicking Save.

---

## Plan: Allocation Full Flow

**Account:** mad@test.com
**Precondition:** Navigate to `/kingdom/allocation`. All rows saved at 0% (clean state).

### Steps

**Page structure**

1. Navigate to `/kingdom/allocation` and observe the page.
   - Expected: Title is "Allocation". Table shows 6 resource rows — Woodcutter, Miner, Farmer, Clergy, Disciple, Scholar — plus a separator row and an Idle row. Each resource row has −5, −, a slider, +, and +5 buttons. A Save button is visible. All rows show 0%. Idle shows 100%.

2. Observe the Idle row Total column at 0% farm allocation.
   - Expected: Total is negative (starvation — food deficit causes population loss with no farmers assigned).

**Button reactivity**

3. Click `+` on Woodcutter.
   - Expected: Woodcutter shows 1%. Idle shows 99%. No page reload.

4. Click `+5` on Woodcutter.
   - Expected: Woodcutter shows 6%. Idle shows 94%.

5. Click `−` on Woodcutter.
   - Expected: Woodcutter shows 5%. Idle shows 95%.

6. Click `−5` on Woodcutter.
   - Expected: Woodcutter shows 0%. Idle shows 100%.

7. Click `−5` on Woodcutter again (already at 0%).
   - Expected: Woodcutter stays at 0%. Idle stays at 100%. The value does not go below zero.

**Setting allocations**

8. Click `+5` on Woodcutter three times (to 15%).
   - Expected: Woodcutter shows 15%. Idle shows 85%.

9. Click `+5` on Miner four times (to 20%).
   - Expected: Miner shows 20%. Idle shows 65%.

10. Click `+5` on Farmer eight times (to 40%).
    - Expected: Farmer shows 40%. Idle shows 25%.

11. Click Save.
    - Expected: No error message. Page re-renders with Woodcutter 15%, Miner 20%, Farmer 40%, Idle 25%.

**Production rates after save**

12. Observe the Woodcutter Production column.
    - Expected: Production is positive. We are earning wood each tick.

13. Observe the Miner Production column.
    - Expected: Production is positive. We are earning stone each tick.

14. Observe the Farmer row Production, Upkeep, and Total columns.
    - Expected: Production is positive, Upkeep is negative, Total is positive. Food surplus — not starving.

15. Observe the Idle row Total column.
    - Expected: Total is positive (food surplus means births exceed losses — population is growing).

**Persistence**

16. Navigate to `/kingdom/allocation` again.
    - Expected: Woodcutter 15%, Miner 20%, Farmer 40%, all others 0%, Idle 25% — values persisted.

**Exceeding 100%**

17. Click `+5` on Miner six times (Miner rises from 20% to 50%). Total allocated = 15 + 50 + 40 = 105%.
    - Expected: Idle shows a negative percentage (signals can go below zero client-side without saving).

18. Click Save.
    - Expected: An error message appears containing "allocation cannot exceed 100%". Values are not saved.

19. Click `−5` on Miner six times (back to 20%).
    - Expected: Idle is positive again (105% → 100%, Idle back to 25%).

20. Click Save.
    - Expected: Error message is gone. Page re-renders successfully with the same allocations as before (Woodcutter 15%, Miner 20%, Farmer 40%).

**Saving at exactly 100%**

21. Click `+5` on Clergy four times, Disciple four times, Scholar four times (each to 20%). Total = 15 + 20 + 40 + 20 + 20 + 20 = 135% — wait, that's too much. Instead: navigate fresh, then set only Scholar to 25% (100 − 75 = 25).
    - Action: Click `+5` on Scholar five times (to 25%). Total = 15 + 20 + 40 + 25 = 100%.
    - Expected: Idle shows 0%.

22. Click Save.
    - Expected: No error. Page re-renders with Idle at 0%.

**Live sidebar updates**

23. Note the current Wood and Stone values shown in the sidebar. Wait a few seconds without interacting.
    - Expected: Wood and Stone values in the sidebar increase automatically without a page reload (SSE tick). If no change is observed within the wait window, report as SKIP — tick interval may be longer than expected in this environment.

**Reset**

24. Click `−5` on Woodcutter three times, `−5` on Miner four times, `−5` on Farmer eight times, `−5` on Scholar five times.
    - Expected: All six resource rows show 0%. Idle shows 100%.

25. Click Save.
    - Expected: No error. Page re-renders with all rows at 0% and Idle at 100%.

