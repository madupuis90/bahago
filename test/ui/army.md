# Army UI Test Plans

The army page (`/kingdom/army`) lets a player manage Legions, transfer units between Reserve and Legions, send Campaigns to other kingdoms, and cancel active Campaigns.

---

## Plan: Army Full Flow

**Account:** mad@test.com / 12345678
**Precondition:** Navigate to `/kingdom/army`. The kingdom "Bob" must have at least 10 Recruit units trained (visible in Reserve). At least one other kingdom must exist in the world (any kingdom will do as the campaign target). No active Campaigns and no Legions exist at the start of this plan.

### Steps

**Page structure**

1. Navigate to `/kingdom/army` and observe the page.
   - Expected: Title "Army". Sections visible: "Transfer Units" form, "Reserve" panel, "Send Campaign" form, "Active Campaigns" panel. No Legion panels are visible. Active Campaigns panel shows "No active campaigns."

2. Observe the Reserve panel.
   - Expected: At least one row with unit type "Recruit" and a count ≥ 10. Columns are Unit, Count, Power.

3. Observe the Transfer Units form.
   - Expected: "From" dropdown contains "Reserve". "To" dropdown contains "Reserve" and "New Legion". Unit type dropdown shows "Recruit". Count field shows 1.

**Creating a Legion via Transfer**

4. Set "From" to "Reserve", "To" to "New Legion", unit type to "Recruit", count to 5. Click Transfer.
   - Expected: No error. Page re-renders. A new Legion panel "Legion 1" appears with an "At Home" badge (green). Legion 1 shows 5 Recruits. Reserve count decreases by 5. The "To" dropdown now includes "Legion 1" alongside "New Legion".

5. Observe the Legion 1 panel.
   - Expected: Badge reads "At Home". A "Disband" button is visible. The unit table shows Recruit, 5, and a power value.

**Transferring to a second Legion**

6. Set "From" to "Reserve", "To" to "New Legion", unit type to "Recruit", count to 3. Click Transfer.
   - Expected: A second Legion panel "Legion 2" appears with an "At Home" badge. Legion 2 shows 3 Recruits. Reserve count decreases by a further 3.

7. Set "From" to "Legion 1", "To" to "Legion 2", unit type to "Recruit", count to 2. Click Transfer.
   - Expected: Legion 1 now shows 3 Recruits. Legion 2 now shows 5 Recruits.

**Cap enforcement**

8. Set "From" to "Reserve", "To" to "New Legion", count to 1. Click Transfer.
   - Expected: Legion 3 appears with an "At Home" badge and 1 Recruit. The "To" dropdown no longer shows "New Legion" (cap of 3 reached).

**Transferring back to Reserve**

9. Set "From" to "Legion 3", "To" to "Reserve", count to 1. Click Transfer.
   - Expected: Legion 3 panel shows "No units assigned." Reserve count increases by 1.

**Disband**

10. Click "Disband" on the Legion 3 panel.
    - Expected: Legion 3 panel disappears. "New Legion" option reappears in the "To" dropdown (cap slot freed).

**Validation — source same as destination**

11. Set "From" to "Reserve", "To" to "Reserve". Click Transfer.
    - Expected: An error alert appears: "source and destination must be different".

**Validation — insufficient units**

12. Set "From" to "Legion 2", "To" to "Reserve", unit type to "Recruit", count to 999. Click Transfer.
    - Expected: An error alert appears: "not enough units in source".

**Validation — count less than 1**

13. Set count to 0. Click Transfer.
    - Expected: An error alert appears: "count must be at least 1".

**Send Campaign — validation**

14. In the Send Campaign form, clear the Target kingdom field and click Send.
    - Expected: Error alert appears: "target kingdom name is required".

15. Type a non-existent kingdom name (e.g. "zzz_no_such_kingdom") in Target. Click Send.
    - Expected: Error alert appears: "target kingdom not found".

**Send Campaign — success**

16. Set Legion to "Legion 1" (5 Recruits), action to "Attack", duration to 4 ticks, target to the name of another existing kingdom. Click Send.
    - Expected: No error. Page re-renders. The Legion 1 panel now shows an "En Route" badge (amber). A row appears in the Active Campaigns table with the correct Legion name, unit composition, action "attack", target kingdom name, status "En route", and a Cancel button.

17. Observe the Transfer form after sending.
    - Expected: "Legion 1" does not appear in the "From" or "To" dropdowns (deployed legions are excluded).

**Send Campaign — deployed legion cannot be sent again**

18. Click the Legion 1 "At Home" badge area or look at the send form Legion dropdown.
    - Expected: Legion 1 is not available in the Send Campaign Legion dropdown.

**Disband deployed legion**

19. Observe the Legion 1 panel while it is deployed.
    - Expected: The "Disband" button is not present on Legion 1's panel.

**Cancel Campaign**

20. In the Active Campaigns table, click the Cancel button on the Legion 1 campaign.
    - Expected: The campaign row's status changes to "Returning" and the Cancel button disappears from that row.

21. Attempt to cancel the same campaign again (if possible).
    - Expected: Either the Cancel button is already gone, or an error appears: "campaign not found or already returning".

**Return of units**

22. After the campaign status shows "Returning" and eventually the campaign row disappears (requires waiting for a tick), navigate back to `/kingdom/army`.
    - Expected: Legion 1 panel shows "At Home" badge. Units are restored to the legion (count may differ from original if casualties occurred during any active phase).
