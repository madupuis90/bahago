# Guild UI Test Plans

The guild system (`/guild`) lets kingdoms form guilds through a proposal-and-support phase, then recruit members once active.

---

## Plan: Guild Lifecycle

**Accounts:**

| Email | Kingdom | Role in this plan |
|-------|---------|-------------------|
| dev1@test.com | Kingdom1 | Applicant → Leader |
| dev2@test.com | Kingdom2 | Supporter → Member |
| dev3@test.com | Kingdom3 | Supporter → Member |
| dev4@test.com | Kingdom4 | Join requester → Member |
| dev5@test.com | Kingdom5 | Invitee |

**Precondition:** None of the five accounts have an existing guild membership. Start logged in as dev1@test.com.

### Steps

**Page structure**

1. Navigate to `/guild` as dev1@test.com.
   - Expected: Landing page shows options to browse guilds or create one. No guild membership shown.

2. Navigate to `/guild/list`.
   - Expected: Page shows an active guilds table and a guild applications section (or empty state messages for each).

**Create and cancel a proposal**

3. Click "Create a Guild" (or navigate to `/guild/new`).
   - Expected: Form shows Name and Description fields and a "Submit Application" button.

4. Submit with Name = "Test Guild Beta" and Description = "Temporary.".
   - Expected: Redirected to `/guild/test-guild-beta`. Status shows "Pending". Kingdom1 listed as Applicant. "Cancel Proposal" button visible. No "Request to Join" button.

5. Click "Cancel Proposal".
   - Expected: Redirected to `/guild`. Navigating to `/guild/test-guild-beta` returns a not-found page.

**Create the real proposal**

6. Navigate to `/guild/new`. Submit with Name = "Test Guild Alpha" and Description = "Our main guild.".
   - Expected: Redirected to `/guild/test-guild-alpha`. Status "Pending". Kingdom1 listed as Applicant. "Cancel Proposal" button visible.

**Supporting the proposal**

7. Log out and log in as dev2@test.com. Navigate to `/guild/test-guild-alpha`.
   - Expected: Status "Pending". "Pledge Support" button visible.

8. Click "Pledge Support".
   - Expected: Kingdom2 appears in the member list as Supporter.

9. Click "Withdraw Support".
   - Expected: Kingdom2 removed from the list. "Pledge Support" button reappears.

10. Click "Pledge Support" again.
    - Expected: Kingdom2 appears as Supporter again.

**Activating the guild**

11. Log out and log in as dev3@test.com. Navigate to `/guild/test-guild-alpha`.
    - Expected: Kingdom2 listed as Supporter. "Pledge Support" button visible.

12. Click "Pledge Support".
    - Expected: Kingdom3 appears as Supporter. Guild status changes to "Active". Kingdom1's role changes to "Leader". Kingdom2 and Kingdom3 shown as Member.

**Requesting to join, rejection, and approval**

13. Log out and log in as dev4@test.com. Navigate to `/guild/test-guild-alpha`.
    - Expected: Guild is active. "Request to Join" button visible.

14. Click "Request to Join".
    - Expected: "Request to Join" button replaced by a "Cancel Request" button. Kingdom4 is no longer seen as a non-member.

15. Log out and log in as dev1@test.com. Navigate to `/guild/test-guild-alpha/manage`.
    - Expected: "Pending Requests" section shows Kingdom4 with Approve and Reject buttons.

16. Click "Reject" next to Kingdom4.
    - Expected: Kingdom4 disappears from Pending Requests and is not in the Members list.

17. Log out and log in as dev4@test.com. Navigate to `/guild/test-guild-alpha`.
    - Expected: "Request to Join" button visible again.

18. Click "Request to Join".
    - Expected: Request is pending.

19. Log out and log in as dev1@test.com. Navigate to `/guild/test-guild-alpha/manage`.
    - Expected: Kingdom4 appears in Pending Requests.

20. Click "Approve" next to Kingdom4.
    - Expected: Kingdom4 disappears from Pending Requests and appears in the Members list with role "Member".

**Promote and demote an officer**

21. On `/guild/test-guild-alpha/manage`, locate Kingdom2.
    - Expected: Kingdom2 shows role "Member" with a "Promote" button.

22. Click "Promote" next to Kingdom2.
    - Expected: Kingdom2's role changes to "Officer". "Demote" button appears. "Promote" button gone.

23. Click "Demote" next to Kingdom2.
    - Expected: Kingdom2's role changes back to "Member". "Promote" button reappears.

**Remove a member**

24. Click "Remove" next to Kingdom4.
    - Expected: Kingdom4 disappears from the Members list.

25. Log out and log in as dev4@test.com. Navigate to `/guild/test-guild-alpha`.
    - Expected: "Request to Join" button visible again.

**Leave a guild**

26. Log out and log in as dev2@test.com. Navigate to `/guild/test-guild-alpha`.
    - Expected: Kingdom2 is shown as Member. "Leave Guild" button visible.

27. Click "Leave Guild".
    - Expected: Redirected to `/guild`. Kingdom2 no longer listed as a member on `/guild/test-guild-alpha`.

**Transfer leadership**

28. Log out and log in as dev1@test.com. Navigate to `/guild/test-guild-alpha/manage`.
    - Expected: Transfer Leadership section visible with a member selector.

29. Select Kingdom3 in the transfer selector and confirm.
    - Expected: Redirected to the guild view page. Kingdom3 shown as Leader. Kingdom1 shown as Member.

30. Log out and log in as dev3@test.com. Navigate to `/guild/test-guild-alpha/manage`.
    - Expected: Kingdom3 can access the manage page. Transfer Leadership section visible.

31. Transfer leadership back to Kingdom1.
    - Expected: Kingdom1 is Leader again.

**Send and accept an invitation**

32. Log out and log in as dev1@test.com. Navigate to `/guild/test-guild-alpha/manage`.
    - Expected: "Invite a Kingdom" section visible with a text input and "Send Invitation" button.

33. Type "Kingdom4" in the invite input and click "Send Invitation".
    - Expected: Kingdom4 appears in the "Pending Invitations" panel. No error.

34. Log out and log in as dev4@test.com. Navigate to `/guild`.
    - Expected: An invitation from Test Guild Alpha is listed on the landing page with Accept and Decline buttons.

35. Navigate to `/guild/test-guild-alpha`.
    - Expected: Invitation notice with "Accept Invitation" and "Decline" buttons visible. "Request to Join" button not shown.

36. Click "Accept Invitation".
    - Expected: Redirected to `/guild/test-guild-alpha`. Kingdom4 appears in the Members list. Invitation notice gone.

37. Log out and log in as dev1@test.com. Navigate to `/guild/test-guild-alpha/manage`.
    - Expected: Kingdom4 no longer in the Pending Invitations panel. Kingdom4 in the Members list.

**Send and decline an invitation**

38. Type "Kingdom5" in the invite input and click "Send Invitation". (dev5 has no membership.)
    - Expected: Kingdom5 appears in the Pending Invitations panel.

39. Log out and log in as dev5@test.com. Navigate to `/guild/test-guild-alpha`.
    - Expected: Invitation notice with Accept and Decline buttons visible.

40. Click "Decline".
    - Expected: Invitation notice disappears. "Request to Join" button reappears.

41. Navigate to `/guild`.
    - Expected: No invitation from Test Guild Alpha on the landing page.

42. Log out and log in as dev1@test.com. Navigate to `/guild/test-guild-alpha/manage`.
    - Expected: Kingdom5 no longer in the Pending Invitations panel.

**Revoke an invitation**

43. Type "Kingdom5" in the invite input and click "Send Invitation".
    - Expected: Kingdom5 appears in the Pending Invitations panel.

44. Click "Revoke" next to Kingdom5.
    - Expected: Kingdom5 disappears from the Pending Invitations panel immediately.

45. Log out and log in as dev5@test.com. Navigate to `/guild`.
    - Expected: No invitation on the landing page.

46. Navigate to `/guild/test-guild-alpha`.
    - Expected: No invitation notice. "Request to Join" button visible.

**Disband**

47. Log out and log in as dev1@test.com. Navigate to `/guild/test-guild-alpha/manage`.
    - Expected: "Disband Guild" button visible in the Leader Actions section.

48. Click "Disband Guild".
    - Expected: Redirected to `/guild`. Navigating to `/guild/test-guild-alpha` returns a not-found page. The guild no longer appears in `/guild/list`.

---

## Plan: Guild Constraints

**Accounts:** dev1@test.com (leader of a fresh active guild), dev2@test.com (Member of that guild), dev3@test.com–dev5@test.com (no memberships).

**Precondition:** An active guild ("Test Guild Alpha") exists with dev1@test.com as Leader and dev2@test.com as Member. dev3–dev5 have no memberships.

### Steps

**Duplicate guild name**

1. Log in as dev3@test.com. Navigate to `/guild/new`. Submit with Name = "Test Guild Alpha".
   - Expected: Error message "a guild with this name already exists". No redirect.

**Cannot join two guilds**

2. Log in as dev2@test.com (already a Member of Test Guild Alpha). Navigate to any other active guild's view page.
   - Expected: "Request to Join" button is NOT visible. No way to submit a join request.

**Cannot invite a committed kingdom**

3. Log in as dev1@test.com. Navigate to `/guild/test-guild-alpha/manage`. Type "Kingdom2" (already a Member) in the invite input and click "Send Invitation".
   - Expected: Error message "Kingdom2 is already a member of this guild". No invitation created.

4. Have dev3@test.com create a second active guild ("Test Guild Beta") with dev4 and dev5 as supporters. Then join dev4 to Test Guild Beta as a full member via a join request and approval.
   - Expected: dev4 is now a Member of Test Guild Beta.

5. As dev1@test.com on `/guild/test-guild-alpha/manage`, type Kingdom4's name in the invite input and click "Send Invitation".
   - Expected: Error message "Kingdom4 is already a member of another guild". No invitation created.

**Accepting an invitation clears other pending invitations**

6. As dev1@test.com, invite dev5@test.com to Test Guild Alpha. As dev3@test.com (leader of Test Guild Beta), invite dev5@test.com to Test Guild Beta.
   - Expected: dev5 has two invitations on their `/guild` landing page.

7. Log in as dev5@test.com. Navigate to `/guild`. Click "Accept" on Test Guild Alpha's invitation.
   - Expected: Redirected to Test Guild Alpha's view page. dev5 is listed as Member.

8. Navigate to `/guild`.
   - Expected: No remaining invitations — the Test Guild Beta invitation was automatically cleared on join.

9. Navigate to Test Guild Beta's view page.
   - Expected: No invitation notice for dev5.

10. Log in as dev3@test.com. Navigate to Test Guild Beta's manage page.
    - Expected: dev5 no longer in the Pending Invitations panel.

**Guild cap (20 members)**

> Note: Requires a guild with exactly 19 active members and a pending join request. Set this up directly in the database before running these steps.

11. Log in as dev1@test.com. Navigate to the full guild's manage page.
    - Expected: 19 members listed. One pending join request visible.

12. Click "Approve" on the pending request.
    - Expected: The applicant joins as the 20th member. No error.

13. Have another kingdom submit a join request to the now-full guild.
    - Expected: The request is accepted as pending (the cap only blocks approval, not submissions).

14. As dev1@test.com, click "Approve" on the new pending request.
    - Expected: Error message "guild is full or request no longer exists". The request remains in the pending list.
