package guild

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"bahago/internal/database/db"
	_guild "bahago/internal/guild"
)

// ── Validators ────────────────────────────────────────────────────────────────

func TestValidateKingdomID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantID  int
		wantErr error
	}{
		{"valid", "42", 42, nil},
		{"zero", "0", 0, ErrInvalidKingdomID},
		{"negative", "-1", 0, ErrInvalidKingdomID},
		{"not_a_number", "abc", 0, ErrInvalidKingdomID},
		{"empty", "", 0, ErrInvalidKingdomID},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, err := validateKingdomID(tc.input)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				if id != tc.wantID {
					t.Errorf("id = %d, want %d", id, tc.wantID)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateTransferInput(t *testing.T) {
	tests := []struct {
		name     string
		input    *transferLeaderSignals
		actorID  int
		wantErrs []error
	}{
		{"valid", &transferLeaderSignals{TargetKingdomID: 42}, 1, nil},
		{"zero_target", &transferLeaderSignals{TargetKingdomID: 0}, 1, []error{ErrInvalidTransferTarget}},
		{"self_target", &transferLeaderSignals{TargetKingdomID: 1}, 1, []error{ErrInvalidTransferTarget}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validateTransferInput(tc.input, tc.actorID)
			if len(got) != len(tc.wantErrs) {
				t.Fatalf("got %d errs (%v), want %d (%v)", len(got), got, len(tc.wantErrs), tc.wantErrs)
			}
			for i, want := range tc.wantErrs {
				if !errors.Is(got[i], want) {
					t.Errorf("errs[%d] = %v, want %v", i, got[i], want)
				}
			}
		})
	}
}

func TestValidateEditDescription(t *testing.T) {
	tests := []struct {
		name     string
		input    *editDescriptionSignals
		wantErrs []error
	}{
		{"empty", &editDescriptionSignals{GuildDescription: ""}, nil},
		{"short", &editDescriptionSignals{GuildDescription: "A short blurb"}, nil},
		{"exactly_500", &editDescriptionSignals{GuildDescription: strings.Repeat("x", 500)}, nil},
		{"too_long", &editDescriptionSignals{GuildDescription: strings.Repeat("x", 501)}, []error{ErrDescriptionTooLong}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validateEditDescription(tc.input)
			if len(got) != len(tc.wantErrs) {
				t.Fatalf("got %d errs (%v), want %d (%v)", len(got), got, len(tc.wantErrs), tc.wantErrs)
			}
			for i, want := range tc.wantErrs {
				if !errors.Is(got[i], want) {
					t.Errorf("errs[%d] = %v, want %v", i, got[i], want)
				}
			}
		})
	}
}

// ── manageStub ────────────────────────────────────────────────────────────────
//
// manageStub satisfies the queries used by the manage.go orchestrators. The
// orchestrators call getGuildAndViewerRole (GetGuildBySlug +
// GetMembershipByKingdomAndGuild) plus a per-operation method. promoteToOfficer
// also uses loadGuildAndMembership which adds ListGuildMembersWithNames and
// GetKingdomGuildMembership.

type manageStub struct {
	db.Querier
	onGetGuildBySlug                 func(ctx context.Context, slug string) (db.Guild, error)
	onGetMembershipByKingdomAndGuild func(ctx context.Context, arg db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error)
	onListGuildMembersWithNames      func(ctx context.Context, guildID int) ([]db.ListGuildMembersWithNamesRow, error)
	onGetKingdomGuildMembership      func(ctx context.Context, kingdomID int) (db.GetKingdomGuildMembershipRow, error)
	onRemoveMembership               func(ctx context.Context, arg db.RemoveMembershipParams) error
	onSetMembershipRole              func(ctx context.Context, arg db.SetMembershipRoleParams) error
	onTransferLeadership             func(ctx context.Context, arg db.TransferLeadershipParams) error
	onDisbandGuild                   func(ctx context.Context, guildID int) error
	onUpdateGuildDescription         func(ctx context.Context, arg db.UpdateGuildDescriptionParams) error
}

func (s *manageStub) GetGuildBySlug(ctx context.Context, slug string) (db.Guild, error) {
	return s.onGetGuildBySlug(ctx, slug)
}
func (s *manageStub) GetMembershipByKingdomAndGuild(ctx context.Context, arg db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
	return s.onGetMembershipByKingdomAndGuild(ctx, arg)
}
func (s *manageStub) ListGuildMembersWithNames(ctx context.Context, guildID int) ([]db.ListGuildMembersWithNamesRow, error) {
	return s.onListGuildMembersWithNames(ctx, guildID)
}
func (s *manageStub) GetKingdomGuildMembership(ctx context.Context, kingdomID int) (db.GetKingdomGuildMembershipRow, error) {
	return s.onGetKingdomGuildMembership(ctx, kingdomID)
}
func (s *manageStub) RemoveMembership(ctx context.Context, arg db.RemoveMembershipParams) error {
	return s.onRemoveMembership(ctx, arg)
}
func (s *manageStub) SetMembershipRole(ctx context.Context, arg db.SetMembershipRoleParams) error {
	return s.onSetMembershipRole(ctx, arg)
}
func (s *manageStub) TransferLeadership(ctx context.Context, arg db.TransferLeadershipParams) error {
	return s.onTransferLeadership(ctx, arg)
}
func (s *manageStub) DisbandGuild(ctx context.Context, guildID int) error {
	return s.onDisbandGuild(ctx, guildID)
}
func (s *manageStub) UpdateGuildDescription(ctx context.Context, arg db.UpdateGuildDescriptionParams) error {
	return s.onUpdateGuildDescription(ctx, arg)
}

// roleStub returns a GetMembershipByKingdomAndGuild override that yields the
// given role for the actor (kingdomID matches actorID) and the given other-role
// for any non-actor kingdom.
func roleStub(actorID, guildID int, actorRole, otherRole _guild.MemberRole) func(context.Context, db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
	return func(_ context.Context, arg db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
		if arg.KingdomID == actorID {
			return db.GuildMembership{GuildID: guildID, KingdomID: actorID, Role: string(actorRole)}, nil
		}
		return db.GuildMembership{GuildID: guildID, KingdomID: arg.KingdomID, Role: string(otherRole)}, nil
	}
}

// ── removeMember ──────────────────────────────────────────────────────────────

func TestRemoveMember_Success(t *testing.T) {
	var removed db.RemoveMembershipParams
	q := &manageStub{
		onGetGuildBySlug:                 func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: 7, Name: "Knights"}, nil },
		onGetMembershipByKingdomAndGuild: roleStub(1, 7, _guild.RoleLeader, _guild.RoleMember),
		onRemoveMembership: func(_ context.Context, arg db.RemoveMembershipParams) error {
			removed = arg
			return nil
		},
	}
	h := &handler{queries: q}
	name, err := h.removeMember(context.Background(), 1, "knights", 42)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if name != "Knights" {
		t.Errorf("name = %q, want %q", name, "Knights")
	}
	if removed.KingdomID != 42 || removed.GuildID != 7 {
		t.Errorf("removed = %+v, want KingdomID=42 GuildID=7", removed)
	}
}

func TestRemoveMember_NotAuthorized(t *testing.T) {
	q := &manageStub{
		onGetGuildBySlug:                 func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: 7}, nil },
		onGetMembershipByKingdomAndGuild: roleStub(1, 7, _guild.RoleMember, _guild.RoleMember),
	}
	h := &handler{queries: q}
	_, err := h.removeMember(context.Background(), 1, "knights", 42)
	if !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("err = %v, want ErrNotAuthorized", err)
	}
}

func TestRemoveMember_CannotRemoveTarget(t *testing.T) {
	// Officer trying to remove another officer.
	q := &manageStub{
		onGetGuildBySlug:                 func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: 7}, nil },
		onGetMembershipByKingdomAndGuild: roleStub(1, 7, _guild.RoleOfficer, _guild.RoleOfficer),
	}
	h := &handler{queries: q}
	_, err := h.removeMember(context.Background(), 1, "knights", 42)
	if !errors.Is(err, ErrCannotRemoveTarget) {
		t.Fatalf("err = %v, want ErrCannotRemoveTarget", err)
	}
}

func TestRemoveMember_GuildNotFound(t *testing.T) {
	q := &manageStub{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{}, pgx.ErrNoRows },
	}
	h := &handler{queries: q}
	_, err := h.removeMember(context.Background(), 1, "knights", 42)
	if !errors.Is(err, ErrGuildNotFound) {
		t.Fatalf("err = %v, want ErrGuildNotFound", err)
	}
}

// ── leaveGuild ────────────────────────────────────────────────────────────────

func TestLeaveGuild_Success(t *testing.T) {
	q := &manageStub{
		onGetGuildBySlug:                 func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: 7, Name: "Knights"}, nil },
		onGetMembershipByKingdomAndGuild: roleStub(1, 7, _guild.RoleMember, _guild.RoleMember),
		onRemoveMembership:               func(_ context.Context, _ db.RemoveMembershipParams) error { return nil },
		onListGuildMembersWithNames: func(_ context.Context, _ int) ([]db.ListGuildMembersWithNamesRow, error) {
			return []db.ListGuildMembersWithNamesRow{
				{KingdomID: 5, Role: string(_guild.RoleLeader)},
				{KingdomID: 6, Role: string(_guild.RoleOfficer)},
				{KingdomID: 7, Role: string(_guild.RoleMember)},
			}, nil
		},
	}
	h := &handler{queries: q}
	name, managerIDs, err := h.leaveGuild(context.Background(), 1, "knights")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if name != "Knights" {
		t.Errorf("name = %q, want %q", name, "Knights")
	}
	if len(managerIDs) != 2 || managerIDs[0] != 5 || managerIDs[1] != 6 {
		t.Errorf("managerIDs = %v, want [5 6]", managerIDs)
	}
}

func TestLeaveGuild_LeaderMustTransferFirst(t *testing.T) {
	q := &manageStub{
		onGetGuildBySlug:                 func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: 7}, nil },
		onGetMembershipByKingdomAndGuild: roleStub(1, 7, _guild.RoleLeader, _guild.RoleMember),
	}
	h := &handler{queries: q}
	_, _, err := h.leaveGuild(context.Background(), 1, "knights")
	if !errors.Is(err, ErrLeaderMustTransferFirst) {
		t.Fatalf("err = %v, want ErrLeaderMustTransferFirst", err)
	}
}

func TestLeaveGuild_CannotLeaveSupporter(t *testing.T) {
	q := &manageStub{
		onGetGuildBySlug:                 func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: 7}, nil },
		onGetMembershipByKingdomAndGuild: roleStub(1, 7, _guild.RoleSupporter, _guild.RoleMember),
	}
	h := &handler{queries: q}
	_, _, err := h.leaveGuild(context.Background(), 1, "knights")
	if !errors.Is(err, ErrCannotLeave) {
		t.Fatalf("err = %v, want ErrCannotLeave", err)
	}
}

// ── promoteToOfficer ──────────────────────────────────────────────────────────

// promoteStub adds ListGuildMembersWithNames + GetKingdomGuildMembership for
// the loadGuildAndMembership call inside promoteToOfficer.
func promoteStubFor(actorID, guildID int, members []db.ListGuildMembersWithNamesRow) *manageStub {
	return &manageStub{
		onGetGuildBySlug:                 func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: guildID, Name: "Knights"}, nil },
		onGetMembershipByKingdomAndGuild: roleStub(actorID, guildID, _guild.RoleLeader, _guild.RoleMember),
		onListGuildMembersWithNames:      func(_ context.Context, _ int) ([]db.ListGuildMembersWithNamesRow, error) { return members, nil },
		onGetKingdomGuildMembership: func(_ context.Context, _ int) (db.GetKingdomGuildMembershipRow, error) {
			return db.GetKingdomGuildMembershipRow{GuildID: guildID}, nil
		},
		onSetMembershipRole: func(_ context.Context, _ db.SetMembershipRoleParams) error { return nil },
	}
}

func TestPromoteToOfficer_Success(t *testing.T) {
	q := promoteStubFor(1, 7, []db.ListGuildMembersWithNamesRow{
		{KingdomID: 1, Role: string(_guild.RoleLeader)},
		{KingdomID: 2, Role: string(_guild.RoleOfficer)},
		{KingdomID: 42, Role: string(_guild.RoleMember)},
	})
	h := &handler{queries: q}
	name, err := h.promoteToOfficer(context.Background(), 1, "knights", 42)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if name != "Knights" {
		t.Errorf("name = %q, want %q", name, "Knights")
	}
}

func TestPromoteToOfficer_NotLeader(t *testing.T) {
	q := promoteStubFor(1, 7, nil)
	q.onGetMembershipByKingdomAndGuild = roleStub(1, 7, _guild.RoleOfficer, _guild.RoleMember)
	h := &handler{queries: q}
	_, err := h.promoteToOfficer(context.Background(), 1, "knights", 42)
	if !errors.Is(err, ErrOnlyLeaderCanPromote) {
		t.Fatalf("err = %v, want ErrOnlyLeaderCanPromote", err)
	}
}

func TestPromoteToOfficer_OfficerCap(t *testing.T) {
	q := promoteStubFor(1, 7, []db.ListGuildMembersWithNamesRow{
		{KingdomID: 1, Role: string(_guild.RoleLeader)},
		{KingdomID: 2, Role: string(_guild.RoleOfficer)},
		{KingdomID: 3, Role: string(_guild.RoleOfficer)},
		{KingdomID: 4, Role: string(_guild.RoleOfficer)},
		{KingdomID: 5, Role: string(_guild.RoleOfficer)},
		{KingdomID: 42, Role: string(_guild.RoleMember)},
	})
	h := &handler{queries: q}
	_, err := h.promoteToOfficer(context.Background(), 1, "knights", 42)
	if !errors.Is(err, ErrOfficerCapReached) {
		t.Fatalf("err = %v, want ErrOfficerCapReached", err)
	}
}

// ── demoteFromOfficer ─────────────────────────────────────────────────────────

func TestDemoteFromOfficer_Success(t *testing.T) {
	var set db.SetMembershipRoleParams
	q := &manageStub{
		onGetGuildBySlug:                 func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: 7, Name: "Knights"}, nil },
		onGetMembershipByKingdomAndGuild: roleStub(1, 7, _guild.RoleLeader, _guild.RoleOfficer),
		onSetMembershipRole: func(_ context.Context, arg db.SetMembershipRoleParams) error {
			set = arg
			return nil
		},
	}
	h := &handler{queries: q}
	name, err := h.demoteFromOfficer(context.Background(), 1, "knights", 42)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if name != "Knights" {
		t.Errorf("name = %q, want %q", name, "Knights")
	}
	if set.Role != string(_guild.RoleMember) || set.KingdomID != 42 {
		t.Errorf("set = %+v, want Role=member KingdomID=42", set)
	}
}

func TestDemoteFromOfficer_OfficerCannotDemoteOfficer(t *testing.T) {
	q := &manageStub{
		onGetGuildBySlug:                 func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: 7}, nil },
		onGetMembershipByKingdomAndGuild: roleStub(1, 7, _guild.RoleOfficer, _guild.RoleOfficer),
	}
	h := &handler{queries: q}
	_, err := h.demoteFromOfficer(context.Background(), 1, "knights", 42)
	if !errors.Is(err, ErrCannotDemoteOfficer) {
		t.Fatalf("err = %v, want ErrCannotDemoteOfficer", err)
	}
}

func TestDemoteFromOfficer_NotAuthorized(t *testing.T) {
	q := &manageStub{
		onGetGuildBySlug:                 func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: 7}, nil },
		onGetMembershipByKingdomAndGuild: roleStub(1, 7, _guild.RoleMember, _guild.RoleOfficer),
	}
	h := &handler{queries: q}
	_, err := h.demoteFromOfficer(context.Background(), 1, "knights", 42)
	if !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("err = %v, want ErrNotAuthorized", err)
	}
}

// ── transferLeadership ────────────────────────────────────────────────────────

func TestTransferLeadership_Success(t *testing.T) {
	var transferred db.TransferLeadershipParams
	q := &manageStub{
		onGetGuildBySlug:                 func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: 7, Name: "Knights"}, nil },
		onGetMembershipByKingdomAndGuild: roleStub(1, 7, _guild.RoleLeader, _guild.RoleOfficer),
		onTransferLeadership: func(_ context.Context, arg db.TransferLeadershipParams) error {
			transferred = arg
			return nil
		},
	}
	h := &handler{queries: q}
	name, err := h.transferLeadership(context.Background(), 1, "knights", 42)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if name != "Knights" {
		t.Errorf("name = %q, want %q", name, "Knights")
	}
	if transferred.NewLeaderKingdomID != 42 || transferred.GuildID != 7 {
		t.Errorf("transferred = %+v, want NewLeaderKingdomID=42 GuildID=7", transferred)
	}
}

func TestTransferLeadership_NotLeader(t *testing.T) {
	q := &manageStub{
		onGetGuildBySlug:                 func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: 7}, nil },
		onGetMembershipByKingdomAndGuild: roleStub(1, 7, _guild.RoleOfficer, _guild.RoleOfficer),
	}
	h := &handler{queries: q}
	_, err := h.transferLeadership(context.Background(), 1, "knights", 42)
	if !errors.Is(err, ErrOnlyLeaderCanTransfer) {
		t.Fatalf("err = %v, want ErrOnlyLeaderCanTransfer", err)
	}
}

func TestTransferLeadership_TargetNotMember(t *testing.T) {
	q := &manageStub{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: 7}, nil },
		onGetMembershipByKingdomAndGuild: func(_ context.Context, arg db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
			if arg.KingdomID == 1 {
				return db.GuildMembership{GuildID: 7, Role: string(_guild.RoleLeader)}, nil
			}
			return db.GuildMembership{}, pgx.ErrNoRows
		},
	}
	h := &handler{queries: q}
	_, err := h.transferLeadership(context.Background(), 1, "knights", 42)
	if !errors.Is(err, ErrTargetNotMember) {
		t.Fatalf("err = %v, want ErrTargetNotMember", err)
	}
}

func TestTransferLeadership_TargetCannotBeLeader(t *testing.T) {
	q := &manageStub{
		onGetGuildBySlug:                 func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: 7}, nil },
		onGetMembershipByKingdomAndGuild: roleStub(1, 7, _guild.RoleLeader, _guild.RoleSupporter),
	}
	h := &handler{queries: q}
	_, err := h.transferLeadership(context.Background(), 1, "knights", 42)
	if !errors.Is(err, ErrTargetCannotBeLeader) {
		t.Fatalf("err = %v, want ErrTargetCannotBeLeader", err)
	}
}

// ── disbandGuild ──────────────────────────────────────────────────────────────

func TestDisbandGuild_Success(t *testing.T) {
	var disbanded int
	q := &manageStub{
		onGetGuildBySlug:                 func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: 7, Name: "Knights"}, nil },
		onGetMembershipByKingdomAndGuild: roleStub(1, 7, _guild.RoleLeader, _guild.RoleMember),
		onListGuildMembersWithNames: func(_ context.Context, _ int) ([]db.ListGuildMembersWithNamesRow, error) {
			return []db.ListGuildMembersWithNamesRow{
				{KingdomID: 1, Role: string(_guild.RoleLeader)},
				{KingdomID: 2, Role: string(_guild.RoleOfficer)},
				{KingdomID: 3, Role: string(_guild.RoleMember)},
				{KingdomID: 4, Role: string(_guild.RoleSupporter)},
			}, nil
		},
		onDisbandGuild: func(_ context.Context, guildID int) error {
			disbanded = guildID
			return nil
		},
	}
	h := &handler{queries: q}
	name, memberIDs, err := h.disbandGuild(context.Background(), 1, "knights")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if name != "Knights" || disbanded != 7 {
		t.Errorf("name=%q disbanded=%d, want Knights/7", name, disbanded)
	}
	// Should include officer + member, excluding leader (the actor) and supporter (non-active).
	if len(memberIDs) != 2 || memberIDs[0] != 2 || memberIDs[1] != 3 {
		t.Errorf("memberIDs = %v, want [2 3]", memberIDs)
	}
}

func TestDisbandGuild_NotLeader(t *testing.T) {
	q := &manageStub{
		onGetGuildBySlug:                 func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: 7}, nil },
		onGetMembershipByKingdomAndGuild: roleStub(1, 7, _guild.RoleOfficer, _guild.RoleMember),
	}
	h := &handler{queries: q}
	_, _, err := h.disbandGuild(context.Background(), 1, "knights")
	if !errors.Is(err, ErrOnlyLeaderCanDisband) {
		t.Fatalf("err = %v, want ErrOnlyLeaderCanDisband", err)
	}
}

// ── updateGuildDescription ────────────────────────────────────────────────────

func TestUpdateGuildDescription_Success(t *testing.T) {
	var saved db.UpdateGuildDescriptionParams
	q := &manageStub{
		onGetGuildBySlug:                 func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: 7}, nil },
		onGetMembershipByKingdomAndGuild: roleStub(1, 7, _guild.RoleLeader, _guild.RoleMember),
		onUpdateGuildDescription: func(_ context.Context, arg db.UpdateGuildDescriptionParams) error {
			saved = arg
			return nil
		},
	}
	h := &handler{queries: q}
	err := h.updateGuildDescription(context.Background(), 1, "knights", &editDescriptionSignals{GuildDescription: "  New blurb  "})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if saved.Description != "New blurb" || saved.ID != 7 {
		t.Errorf("saved = %+v, want Description=\"New blurb\" ID=7", saved)
	}
}

func TestUpdateGuildDescription_NotLeader(t *testing.T) {
	q := &manageStub{
		onGetGuildBySlug:                 func(_ context.Context, _ string) (db.Guild, error) { return db.Guild{ID: 7}, nil },
		onGetMembershipByKingdomAndGuild: roleStub(1, 7, _guild.RoleOfficer, _guild.RoleMember),
	}
	h := &handler{queries: q}
	err := h.updateGuildDescription(context.Background(), 1, "knights", &editDescriptionSignals{GuildDescription: "blurb"})
	if !errors.Is(err, ErrOnlyLeaderCanEditDescription) {
		t.Fatalf("err = %v, want ErrOnlyLeaderCanEditDescription", err)
	}
}
