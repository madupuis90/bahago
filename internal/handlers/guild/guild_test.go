package guild_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/handlers/guild"
	"bahago/internal/routes"
	"bahago/internal/testhelper"
)

// ── Stub querier ──────────────────────────────────────────────────────────────

// stubQuerier embeds a nil db.Querier. Any method not explicitly overridden
// panics via a nil pointer dereference, making unexpected DB calls immediately
// visible.
type stubQuerier struct {
	db.Querier
	onGetGuildBySlug                 func(ctx context.Context, slug string) (db.Guild, error)
	onGetMembershipByKingdomAndGuild func(ctx context.Context, arg db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error)
	onListGuildMembersWithNames      func(ctx context.Context, guildID int) ([]db.ListGuildMembersWithNamesRow, error)
	onRemoveMembership               func(ctx context.Context, arg db.RemoveMembershipParams) error
	onSetMembershipRole              func(ctx context.Context, arg db.SetMembershipRoleParams) error
	onApproveMembershipIfNotFull     func(ctx context.Context, arg db.ApproveMembershipIfNotFullParams) (int, error)
	onGetKingdomGuildMembership      func(ctx context.Context, kingdomID int) (db.GetKingdomGuildMembershipRow, error)
	onGetKingdomsByIDs               func(ctx context.Context, ids []int) ([]db.Kingdom, error)
	onBulkCreateMessages             func(ctx context.Context, arg db.BulkCreateMessagesParams) error
	onListActiveGuilds               func(ctx context.Context) ([]db.ListActiveGuildsRow, error)
	onListPendingGuilds              func(ctx context.Context) ([]db.ListPendingGuildsRow, error)
}

func (s *stubQuerier) GetGuildBySlug(ctx context.Context, slug string) (db.Guild, error) {
	if s.onGetGuildBySlug != nil {
		return s.onGetGuildBySlug(ctx, slug)
	}
	panic("stubQuerier: unexpected call to GetGuildBySlug")
}

func (s *stubQuerier) GetMembershipByKingdomAndGuild(ctx context.Context, arg db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
	if s.onGetMembershipByKingdomAndGuild != nil {
		return s.onGetMembershipByKingdomAndGuild(ctx, arg)
	}
	panic("stubQuerier: unexpected call to GetMembershipByKingdomAndGuild")
}

func (s *stubQuerier) ListGuildMembersWithNames(ctx context.Context, guildID int) ([]db.ListGuildMembersWithNamesRow, error) {
	if s.onListGuildMembersWithNames != nil {
		return s.onListGuildMembersWithNames(ctx, guildID)
	}
	panic("stubQuerier: unexpected call to ListGuildMembersWithNames")
}

func (s *stubQuerier) RemoveMembership(ctx context.Context, arg db.RemoveMembershipParams) error {
	if s.onRemoveMembership != nil {
		return s.onRemoveMembership(ctx, arg)
	}
	panic("stubQuerier: unexpected call to RemoveMembership")
}

func (s *stubQuerier) SetMembershipRole(ctx context.Context, arg db.SetMembershipRoleParams) error {
	if s.onSetMembershipRole != nil {
		return s.onSetMembershipRole(ctx, arg)
	}
	panic("stubQuerier: unexpected call to SetMembershipRole")
}

func (s *stubQuerier) ApproveMembershipIfNotFull(ctx context.Context, arg db.ApproveMembershipIfNotFullParams) (int, error) {
	if s.onApproveMembershipIfNotFull != nil {
		return s.onApproveMembershipIfNotFull(ctx, arg)
	}
	panic("stubQuerier: unexpected call to ApproveMembershipIfNotFull")
}

func (s *stubQuerier) GetKingdomGuildMembership(ctx context.Context, kingdomID int) (db.GetKingdomGuildMembershipRow, error) {
	if s.onGetKingdomGuildMembership != nil {
		return s.onGetKingdomGuildMembership(ctx, kingdomID)
	}
	panic("stubQuerier: unexpected call to GetKingdomGuildMembership")
}

func (s *stubQuerier) GetKingdomsByIDs(ctx context.Context, ids []int) ([]db.Kingdom, error) {
	if s.onGetKingdomsByIDs != nil {
		return s.onGetKingdomsByIDs(ctx, ids)
	}
	panic("stubQuerier: unexpected call to GetKingdomsByIDs")
}

func (s *stubQuerier) BulkCreateMessages(ctx context.Context, arg db.BulkCreateMessagesParams) error {
	if s.onBulkCreateMessages != nil {
		return s.onBulkCreateMessages(ctx, arg)
	}
	panic("stubQuerier: unexpected call to BulkCreateMessages")
}

func (s *stubQuerier) ListActiveGuilds(ctx context.Context) ([]db.ListActiveGuildsRow, error) {
	if s.onListActiveGuilds != nil {
		return s.onListActiveGuilds(ctx)
	}
	panic("stubQuerier: unexpected call to ListActiveGuilds")
}

func (s *stubQuerier) ListPendingGuilds(ctx context.Context) ([]db.ListPendingGuildsRow, error) {
	if s.onListPendingGuilds != nil {
		return s.onListPendingGuilds(ctx)
	}
	panic("stubQuerier: unexpected call to ListPendingGuilds")
}

// ── Shared fixtures ───────────────────────────────────────────────────────────

var (
	leaderKingdom  = &db.Kingdom{ID: 1, Name: "Camelot"}
	memberKingdom  = &db.Kingdom{ID: 2, Name: "Avalon"}
	officerKingdom = &db.Kingdom{ID: 3, Name: "Logres"}

	activeGuild = db.Guild{ID: 10, Name: "Round Table", Slug: "round-table", Status: "active"}
)

func membershipFor(kingdomID, guildID int, role string) db.GuildMembership {
	return db.GuildMembership{ID: 99, GuildID: guildID, KingdomID: kingdomID, Role: role}
}

// ── Handler extractors ────────────────────────────────────────────────────────

func leaveHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	guild.RegisterRoutes(cr, q, nil, nil)
	return cr.Handlers["POST "+routes.GuildLeavePath]
}

func removeHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	guild.RegisterRoutes(cr, q, nil, nil)
	return cr.Handlers["POST "+routes.GuildRemoveMemberPath]
}

func demoteHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	guild.RegisterRoutes(cr, q, nil, nil)
	return cr.Handlers["POST "+routes.GuildDemotePath]
}

func promoteHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	guild.RegisterRoutes(cr, q, nil, nil)
	return cr.Handlers["POST "+routes.GuildPromotePath]
}

func approveHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	guild.RegisterRoutes(cr, q, nil, nil)
	return cr.Handlers["POST "+routes.GuildApproveMemberPath]
}

func createHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	guild.RegisterRoutes(cr, q, nil, nil)
	return cr.Handlers["POST "+routes.GuildCreatePath]
}

// ── Request builders ──────────────────────────────────────────────────────────

func slugReq(method, path, slug string, kingdom *db.Kingdom) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.SetPathValue("slug", slug)
	return r.WithContext(context.WithValue(r.Context(), contextkeys.Kingdom, kingdom))
}

func slugIDReq(method, path, slug, id string, kingdom *db.Kingdom) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.SetPathValue("slug", slug)
	r.SetPathValue("id", id)
	return r.WithContext(context.WithValue(r.Context(), contextkeys.Kingdom, kingdom))
}

func signalsReq(method, path, body string, kingdom *db.Kingdom) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	return r.WithContext(context.WithValue(r.Context(), contextkeys.Kingdom, kingdom))
}

// ── handleLeave tests ─────────────────────────────────────────────────────────

func stubForLeave(viewerKingdomID int, role string) *stubQuerier {
	return &stubQuerier{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return activeGuild, nil
		},
		onGetMembershipByKingdomAndGuild: func(_ context.Context, arg db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
			if arg.KingdomID == viewerKingdomID {
				return membershipFor(viewerKingdomID, activeGuild.ID, role), nil
			}
			return db.GuildMembership{}, pgx.ErrNoRows
		},
	}
}

func TestHandleLeave_LeaderCannotLeave(t *testing.T) {
	h := leaveHandler(stubForLeave(leaderKingdom.ID, "leader"))
	w := httptest.NewRecorder()
	h(w, slugReq("POST", routes.GuildLeavePath, activeGuild.Slug, leaderKingdom))
	testhelper.AssertContains(t, w.Body.String(), "transfer leadership")
}

func TestHandleLeave_NonMemberDenied(t *testing.T) {
	stub := &stubQuerier{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return activeGuild, nil
		},
		onGetMembershipByKingdomAndGuild: func(_ context.Context, _ db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
			return db.GuildMembership{}, pgx.ErrNoRows
		},
	}
	h := leaveHandler(stub)
	w := httptest.NewRecorder()
	h(w, slugReq("POST", routes.GuildLeavePath, activeGuild.Slug, memberKingdom))
	testhelper.AssertContains(t, w.Body.String(), "permission denied")
}

// ── handleRemove tests ────────────────────────────────────────────────────────

func TestHandleRemove_OfficerCannotRemoveOfficer(t *testing.T) {
	stub := &stubQuerier{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return activeGuild, nil
		},
		onGetMembershipByKingdomAndGuild: func(_ context.Context, arg db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
			// viewer = officer; target = also officer
			return membershipFor(arg.KingdomID, activeGuild.ID, "officer"), nil
		},
	}
	h := removeHandler(stub)
	w := httptest.NewRecorder()
	h(w, slugIDReq("POST", routes.GuildRemoveMemberPath, activeGuild.Slug, "2", officerKingdom))
	testhelper.AssertContains(t, w.Body.String(), "officers can only remove regular members")
}

// ── handleDemote tests ────────────────────────────────────────────────────────

func TestHandleDemote_OfficerCannotDemoteOfficer(t *testing.T) {
	stub := &stubQuerier{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return activeGuild, nil
		},
		onGetMembershipByKingdomAndGuild: func(_ context.Context, arg db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
			// viewer = officer; target = also officer
			return membershipFor(arg.KingdomID, activeGuild.ID, "officer"), nil
		},
	}
	h := demoteHandler(stub)
	w := httptest.NewRecorder()
	h(w, slugIDReq("POST", routes.GuildDemotePath, activeGuild.Slug, "2", officerKingdom))
	testhelper.AssertContains(t, w.Body.String(), "officers cannot demote other officers")
}

// ── handlePromote tests ───────────────────────────────────────────────────────

func TestHandlePromote_OfficerCapEnforced(t *testing.T) {
	stub := &stubQuerier{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return activeGuild, nil
		},
		onGetMembershipByKingdomAndGuild: func(_ context.Context, arg db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
			if arg.KingdomID == leaderKingdom.ID {
				return membershipFor(leaderKingdom.ID, activeGuild.ID, "leader"), nil
			}
			return db.GuildMembership{}, pgx.ErrNoRows
		},
		onListGuildMembersWithNames: func(_ context.Context, _ int) ([]db.ListGuildMembersWithNamesRow, error) {
			rows := []db.ListGuildMembersWithNamesRow{
				{KingdomID: leaderKingdom.ID, Role: "leader"},
				{KingdomID: 101, Role: "officer"},
				{KingdomID: 102, Role: "officer"},
				{KingdomID: 103, Role: "officer"},
				{KingdomID: 104, Role: "officer"},
			}
			return rows, nil
		},
		onGetKingdomGuildMembership: func(_ context.Context, _ int) (db.GetKingdomGuildMembershipRow, error) {
			return db.GetKingdomGuildMembershipRow{}, pgx.ErrNoRows
		},
	}

	h := promoteHandler(stub)
	w := httptest.NewRecorder()
	h(w, slugIDReq("POST", routes.GuildPromotePath, activeGuild.Slug, "2", leaderKingdom))
	testhelper.AssertContains(t, w.Body.String(), "at most 4 officers")
}

func TestHandlePromote_OnlyLeaderCanPromote(t *testing.T) {
	stub := &stubQuerier{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return activeGuild, nil
		},
		onGetMembershipByKingdomAndGuild: func(_ context.Context, arg db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
			return membershipFor(arg.KingdomID, activeGuild.ID, "officer"), nil
		},
		onListGuildMembersWithNames: func(_ context.Context, _ int) ([]db.ListGuildMembersWithNamesRow, error) {
			return []db.ListGuildMembersWithNamesRow{
				{KingdomID: officerKingdom.ID, Role: "officer"},
			}, nil
		},
		onGetKingdomGuildMembership: func(_ context.Context, _ int) (db.GetKingdomGuildMembershipRow, error) {
			return db.GetKingdomGuildMembershipRow{}, pgx.ErrNoRows
		},
	}

	h := promoteHandler(stub)
	w := httptest.NewRecorder()
	h(w, slugIDReq("POST", routes.GuildPromotePath, activeGuild.Slug, "2", officerKingdom))
	testhelper.AssertContains(t, w.Body.String(), "only the guild leader can promote")
}

// ── handleApprove tests ───────────────────────────────────────────────────────

func TestHandleApprove_GuildFull(t *testing.T) {
	stub := &stubQuerier{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return activeGuild, nil
		},
		onGetMembershipByKingdomAndGuild: func(_ context.Context, arg db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
			if arg.KingdomID == leaderKingdom.ID {
				return membershipFor(leaderKingdom.ID, activeGuild.ID, "leader"), nil
			}
			return db.GuildMembership{}, pgx.ErrNoRows
		},
		onApproveMembershipIfNotFull: func(_ context.Context, _ db.ApproveMembershipIfNotFullParams) (int, error) {
			return 0, pgx.ErrNoRows
		},
	}

	h := approveHandler(stub)
	w := httptest.NewRecorder()
	h(w, slugIDReq("POST", routes.GuildApproveMemberPath, activeGuild.Slug, "99", leaderKingdom))
	testhelper.AssertContains(t, w.Body.String(), "guild is full or request no longer exists")
}

func TestHandleApprove_NotAuthorized(t *testing.T) {
	stub := &stubQuerier{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return activeGuild, nil
		},
		onGetMembershipByKingdomAndGuild: func(_ context.Context, _ db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
			return membershipFor(memberKingdom.ID, activeGuild.ID, "member"), nil
		},
	}

	h := approveHandler(stub)
	w := httptest.NewRecorder()
	h(w, slugIDReq("POST", routes.GuildApproveMemberPath, activeGuild.Slug, "99", memberKingdom))
	testhelper.AssertContains(t, w.Body.String(), "not authorized")
}

// ── handleCreate name validation tests ───────────────────────────────────────

func TestHandleCreate_NameTooShort(t *testing.T) {
	h := createHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, signalsReq("POST", routes.GuildCreatePath,
		`{"guild_name":"Hi","guild_description":""}`, leaderKingdom))
	testhelper.AssertContains(t, w.Body.String(), "between 5 and 60")
}

func TestHandleCreate_NameTooLong(t *testing.T) {
	h := createHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, signalsReq("POST", routes.GuildCreatePath,
		`{"guild_name":"`+strings.Repeat("a", 61)+`","guild_description":""}`, leaderKingdom))
	testhelper.AssertContains(t, w.Body.String(), "between 5 and 60")
}

func TestHandleCreate_DescriptionTooLong(t *testing.T) {
	h := createHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, signalsReq("POST", routes.GuildCreatePath,
		`{"guild_name":"Valid Name","guild_description":"`+strings.Repeat("x", 501)+`"}`, leaderKingdom))
	testhelper.AssertContains(t, w.Body.String(), "cannot exceed 500")
}

// ── Stub extensions for invitation handlers ───────────────────────────────────

type inviteStubQuerier struct {
	stubQuerier
	onGetKingdomByName          func(ctx context.Context, name string) (db.Kingdom, error)
	onGetKingdomGuildInvitation func(ctx context.Context, arg db.GetKingdomGuildInvitationParams) (int, error)
	onCreateGuildInvitation     func(ctx context.Context, arg db.CreateGuildInvitationParams) error
	onRevokeGuildInvitation     func(ctx context.Context, arg db.RevokeGuildInvitationParams) (int, error)
	onAcceptGuildInvitation     func(ctx context.Context, arg db.AcceptGuildInvitationParams) (int, error)
	onDeclineGuildInvitation    func(ctx context.Context, arg db.DeclineGuildInvitationParams) (int, error)
	onListGuildInvitations      func(ctx context.Context, guildID int) ([]db.ListGuildInvitationsRow, error)
	onListPendingRequests       func(ctx context.Context, guildID int) ([]db.ListPendingRequestsRow, error)
}

func (s *inviteStubQuerier) GetKingdomByName(ctx context.Context, name string) (db.Kingdom, error) {
	if s.onGetKingdomByName != nil {
		return s.onGetKingdomByName(ctx, name)
	}
	panic("inviteStubQuerier: unexpected call to GetKingdomByName")
}

func (s *inviteStubQuerier) GetKingdomGuildInvitation(ctx context.Context, arg db.GetKingdomGuildInvitationParams) (int, error) {
	if s.onGetKingdomGuildInvitation != nil {
		return s.onGetKingdomGuildInvitation(ctx, arg)
	}
	panic("inviteStubQuerier: unexpected call to GetKingdomGuildInvitation")
}

func (s *inviteStubQuerier) CreateGuildInvitation(ctx context.Context, arg db.CreateGuildInvitationParams) error {
	if s.onCreateGuildInvitation != nil {
		return s.onCreateGuildInvitation(ctx, arg)
	}
	panic("inviteStubQuerier: unexpected call to CreateGuildInvitation")
}

func (s *inviteStubQuerier) RevokeGuildInvitation(ctx context.Context, arg db.RevokeGuildInvitationParams) (int, error) {
	if s.onRevokeGuildInvitation != nil {
		return s.onRevokeGuildInvitation(ctx, arg)
	}
	panic("inviteStubQuerier: unexpected call to RevokeGuildInvitation")
}

func (s *inviteStubQuerier) AcceptGuildInvitation(ctx context.Context, arg db.AcceptGuildInvitationParams) (int, error) {
	if s.onAcceptGuildInvitation != nil {
		return s.onAcceptGuildInvitation(ctx, arg)
	}
	panic("inviteStubQuerier: unexpected call to AcceptGuildInvitation")
}

func (s *inviteStubQuerier) DeclineGuildInvitation(ctx context.Context, arg db.DeclineGuildInvitationParams) (int, error) {
	if s.onDeclineGuildInvitation != nil {
		return s.onDeclineGuildInvitation(ctx, arg)
	}
	panic("inviteStubQuerier: unexpected call to DeclineGuildInvitation")
}

func (s *inviteStubQuerier) ListGuildInvitations(ctx context.Context, guildID int) ([]db.ListGuildInvitationsRow, error) {
	if s.onListGuildInvitations != nil {
		return s.onListGuildInvitations(ctx, guildID)
	}
	panic("inviteStubQuerier: unexpected call to ListGuildInvitations")
}

func (s *inviteStubQuerier) ListPendingRequests(ctx context.Context, guildID int) ([]db.ListPendingRequestsRow, error) {
	if s.onListPendingRequests != nil {
		return s.onListPendingRequests(ctx, guildID)
	}
	panic("inviteStubQuerier: unexpected call to ListPendingRequests")
}

// ── Handler extractors for invitation handlers ────────────────────────────────

func sendInviteHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	guild.RegisterRoutes(cr, q, nil, nil)
	return cr.Handlers["POST "+routes.GuildInvitePath]
}

func revokeInviteHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	guild.RegisterRoutes(cr, q, nil, nil)
	return cr.Handlers["POST "+routes.GuildInvitationRevokePath]
}

func acceptInviteHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	guild.RegisterRoutes(cr, q, nil, nil)
	return cr.Handlers["POST "+routes.GuildInvitationAcceptPath]
}

func declineInviteHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	guild.RegisterRoutes(cr, q, nil, nil)
	return cr.Handlers["POST "+routes.GuildInvitationDeclinePath]
}

// ── handleSendInvitation tests ────────────────────────────────────────────────

// baseInviteStub returns a stub wired for a leader viewer with no existing
// membership for the target kingdom in this guild and no other guild commitment.
func baseInviteStub(targetKingdom db.Kingdom) *inviteStubQuerier {
	return &inviteStubQuerier{
		stubQuerier: stubQuerier{
			onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
				return activeGuild, nil
			},
			onGetMembershipByKingdomAndGuild: func(_ context.Context, arg db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
				if arg.KingdomID == leaderKingdom.ID {
					return membershipFor(leaderKingdom.ID, activeGuild.ID, "leader"), nil
				}
				return db.GuildMembership{}, pgx.ErrNoRows
			},
			onGetKingdomGuildMembership: func(_ context.Context, _ int) (db.GetKingdomGuildMembershipRow, error) {
				return db.GetKingdomGuildMembershipRow{}, pgx.ErrNoRows
			},
		},
		onGetKingdomByName: func(_ context.Context, _ string) (db.Kingdom, error) {
			return targetKingdom, nil
		},
	}
}

func TestHandleSendInvitation_NonManagerDenied(t *testing.T) {
	stub := &inviteStubQuerier{
		stubQuerier: stubQuerier{
			onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
				return activeGuild, nil
			},
			onGetMembershipByKingdomAndGuild: func(_ context.Context, _ db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
				return membershipFor(memberKingdom.ID, activeGuild.ID, "member"), nil
			},
		},
	}
	h := sendInviteHandler(stub)
	w := httptest.NewRecorder()
	h(w, signalsReq("POST", routes.GuildInvitePath,
		`{"invite_kingdom_name":"Avalon"}`, memberKingdom))
	testhelper.AssertContains(t, w.Body.String(), "not authorized")
}

func TestHandleSendInvitation_TargetNotFound(t *testing.T) {
	stub := &inviteStubQuerier{
		stubQuerier: stubQuerier{
			onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
				return activeGuild, nil
			},
			onGetMembershipByKingdomAndGuild: func(_ context.Context, _ db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
				return membershipFor(leaderKingdom.ID, activeGuild.ID, "leader"), nil
			},
		},
		onGetKingdomByName: func(_ context.Context, _ string) (db.Kingdom, error) {
			return db.Kingdom{}, pgx.ErrNoRows
		},
	}
	h := sendInviteHandler(stub)
	w := httptest.NewRecorder()
	h(w, signalsReq("POST", routes.GuildInvitePath,
		`{"invite_kingdom_name":"Unknown"}`, leaderKingdom))
	testhelper.AssertContains(t, w.Body.String(), "not found")
}

func TestHandleSendInvitation_TargetAlreadyInvited(t *testing.T) {
	stub := &inviteStubQuerier{
		stubQuerier: stubQuerier{
			onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
				return activeGuild, nil
			},
			onGetMembershipByKingdomAndGuild: func(_ context.Context, arg db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
				if arg.KingdomID == leaderKingdom.ID {
					return membershipFor(leaderKingdom.ID, activeGuild.ID, "leader"), nil
				}
				// Target already has an invited row.
				return membershipFor(memberKingdom.ID, activeGuild.ID, "invited"), nil
			},
		},
		onGetKingdomByName: func(_ context.Context, _ string) (db.Kingdom, error) {
			return *memberKingdom, nil
		},
	}
	h := sendInviteHandler(stub)
	w := httptest.NewRecorder()
	h(w, signalsReq("POST", routes.GuildInvitePath,
		`{"invite_kingdom_name":"Avalon"}`, leaderKingdom))
	testhelper.AssertContains(t, w.Body.String(), "already been invited")
}

func TestHandleSendInvitation_TargetAlreadyMember(t *testing.T) {
	stub := &inviteStubQuerier{
		stubQuerier: stubQuerier{
			onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
				return activeGuild, nil
			},
			onGetMembershipByKingdomAndGuild: func(_ context.Context, arg db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
				if arg.KingdomID == leaderKingdom.ID {
					return membershipFor(leaderKingdom.ID, activeGuild.ID, "leader"), nil
				}
				// Target is already a full member of this guild.
				return membershipFor(memberKingdom.ID, activeGuild.ID, "member"), nil
			},
		},
		onGetKingdomByName: func(_ context.Context, _ string) (db.Kingdom, error) {
			return *memberKingdom, nil
		},
	}
	h := sendInviteHandler(stub)
	w := httptest.NewRecorder()
	h(w, signalsReq("POST", routes.GuildInvitePath,
		`{"invite_kingdom_name":"Avalon"}`, leaderKingdom))
	testhelper.AssertContains(t, w.Body.String(), "already a member of this guild")
}

func TestHandleSendInvitation_TargetCommittedElsewhere(t *testing.T) {
	otherGuild := db.Guild{ID: 99, Name: "Other Guild", Slug: "other-guild", Status: "active"}
	stub := baseInviteStub(*memberKingdom)
	// Override: target IS committed to a different guild.
	stub.stubQuerier.onGetKingdomGuildMembership = func(_ context.Context, kingdomID int) (db.GetKingdomGuildMembershipRow, error) {
		if kingdomID == memberKingdom.ID {
			return db.GetKingdomGuildMembershipRow{GuildID: otherGuild.ID, GuildSlug: otherGuild.Slug}, nil
		}
		return db.GetKingdomGuildMembershipRow{}, pgx.ErrNoRows
	}
	h := sendInviteHandler(stub)
	w := httptest.NewRecorder()
	h(w, signalsReq("POST", routes.GuildInvitePath,
		`{"invite_kingdom_name":"Avalon"}`, leaderKingdom))
	testhelper.AssertContains(t, w.Body.String(), "already a member of another guild")
}

// ── handleRevokeInvitation tests ──────────────────────────────────────────────

func TestHandleRevokeInvitation_NonManagerDenied(t *testing.T) {
	stub := &inviteStubQuerier{
		stubQuerier: stubQuerier{
			onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
				return activeGuild, nil
			},
			onGetMembershipByKingdomAndGuild: func(_ context.Context, _ db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
				return membershipFor(memberKingdom.ID, activeGuild.ID, "member"), nil
			},
		},
	}
	h := revokeInviteHandler(stub)
	w := httptest.NewRecorder()
	h(w, slugIDReq("POST", routes.GuildInvitationRevokePath, activeGuild.Slug, "5", memberKingdom))
	testhelper.AssertContains(t, w.Body.String(), "not authorized")
}

func TestHandleRevokeInvitation_AlreadyGone(t *testing.T) {
	stub := &inviteStubQuerier{
		stubQuerier: stubQuerier{
			onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
				return activeGuild, nil
			},
			onGetMembershipByKingdomAndGuild: func(_ context.Context, _ db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
				return membershipFor(leaderKingdom.ID, activeGuild.ID, "leader"), nil
			},
			onGetKingdomsByIDs: func(_ context.Context, _ []int) ([]db.Kingdom, error) {
				return nil, nil
			},
		},
		onRevokeGuildInvitation: func(_ context.Context, _ db.RevokeGuildInvitationParams) (int, error) {
			return 0, pgx.ErrNoRows
		},
	}
	h := revokeInviteHandler(stub)
	w := httptest.NewRecorder()
	// ErrNoRows on a gone invitation must not show an error; it publishes and returns 200.
	h(w, slugIDReq("POST", routes.GuildInvitationRevokePath, activeGuild.Slug, "5", leaderKingdom))
	testhelper.AssertNotContains(t, w.Body.String(), "internal error")
}

// ── handleAcceptInvitation tests ──────────────────────────────────────────────

func TestHandleAcceptInvitation_GuildFullOrInvitationGone(t *testing.T) {
	stub := &inviteStubQuerier{
		stubQuerier: stubQuerier{
			onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
				return activeGuild, nil
			},
		},
		onAcceptGuildInvitation: func(_ context.Context, _ db.AcceptGuildInvitationParams) (int, error) {
			return 0, pgx.ErrNoRows
		},
	}
	h := acceptInviteHandler(stub)
	w := httptest.NewRecorder()
	h(w, slugIDReq("POST", routes.GuildInvitationAcceptPath, activeGuild.Slug, "7", memberKingdom))
	testhelper.AssertContains(t, w.Body.String(), "no longer valid or the guild is full")
}

// ── handleDeclineInvitation tests ─────────────────────────────────────────────

func TestHandleDeclineInvitation_AlreadyGone(t *testing.T) {
	stub := &inviteStubQuerier{
		onDeclineGuildInvitation: func(_ context.Context, _ db.DeclineGuildInvitationParams) (int, error) {
			return 0, pgx.ErrNoRows
		},
	}
	h := declineInviteHandler(stub)
	w := httptest.NewRecorder()
	// ErrNoRows on a gone invitation should redirect silently, not return an error.
	h(w, slugIDReq("POST", routes.GuildInvitationDeclinePath, activeGuild.Slug, "7", memberKingdom))
	testhelper.AssertNotContains(t, w.Body.String(), "internal error")
}
