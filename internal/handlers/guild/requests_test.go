package guild

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"bahago/internal/database/db"
	_guild "bahago/internal/guild"
)

// ── validateMembershipID ──────────────────────────────────────────────────────

func TestValidateMembershipID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantID  int
		wantErr error
	}{
		{"valid", "42", 42, nil},
		{"zero", "0", 0, ErrInvalidMembershipID},
		{"negative", "-1", 0, ErrInvalidMembershipID},
		{"not_a_number", "abc", 0, ErrInvalidMembershipID},
		{"empty", "", 0, ErrInvalidMembershipID},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, err := validateMembershipID(tc.input)
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

// ── requestJoinGuild ──────────────────────────────────────────────────────────

type requestJoinStub struct {
	db.Querier
	onGetGuildBySlug            func(ctx context.Context, slug string) (db.Guild, error)
	onGetKingdomGuildMembership func(ctx context.Context, kingdomID int) (db.GetKingdomGuildMembershipRow, error)
	onRequestJoinIfNotFull      func(ctx context.Context, arg db.RequestJoinIfNotFullParams) (db.GuildMembership, error)
}

func (s *requestJoinStub) GetGuildBySlug(ctx context.Context, slug string) (db.Guild, error) {
	return s.onGetGuildBySlug(ctx, slug)
}
func (s *requestJoinStub) GetKingdomGuildMembership(ctx context.Context, kingdomID int) (db.GetKingdomGuildMembershipRow, error) {
	return s.onGetKingdomGuildMembership(ctx, kingdomID)
}
func (s *requestJoinStub) RequestJoinIfNotFull(ctx context.Context, arg db.RequestJoinIfNotFullParams) (db.GuildMembership, error) {
	return s.onRequestJoinIfNotFull(ctx, arg)
}

func TestRequestJoinGuild_Success(t *testing.T) {
	q := &requestJoinStub{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return db.Guild{ID: 7, Name: "Knights", Status: string(_guild.GuildStatusActive)}, nil
		},
		onGetKingdomGuildMembership: func(_ context.Context, _ int) (db.GetKingdomGuildMembershipRow, error) {
			return db.GetKingdomGuildMembershipRow{}, pgx.ErrNoRows
		},
		onRequestJoinIfNotFull: func(_ context.Context, _ db.RequestJoinIfNotFullParams) (db.GuildMembership, error) {
			return db.GuildMembership{ID: 99}, nil
		},
	}
	h := &handler{queries: q}
	guildID, guildName, err := h.requestJoinGuild(context.Background(), 1, "knights")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if guildID != 7 {
		t.Errorf("guildID = %d, want 7", guildID)
	}
	if guildName != "Knights" {
		t.Errorf("guildName = %q, want %q", guildName, "Knights")
	}
}

func TestRequestJoinGuild_GuildNotFound(t *testing.T) {
	q := &requestJoinStub{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return db.Guild{}, pgx.ErrNoRows
		},
	}
	h := &handler{queries: q}
	_, _, err := h.requestJoinGuild(context.Background(), 1, "knights")
	if !errors.Is(err, ErrGuildNotFound) {
		t.Fatalf("err = %v, want ErrGuildNotFound", err)
	}
}

func TestRequestJoinGuild_GuildNotActive(t *testing.T) {
	q := &requestJoinStub{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return db.Guild{ID: 7, Status: string(_guild.GuildStatusPending)}, nil
		},
	}
	h := &handler{queries: q}
	_, _, err := h.requestJoinGuild(context.Background(), 1, "knights")
	if !errors.Is(err, ErrGuildNotActive) {
		t.Fatalf("err = %v, want ErrGuildNotActive", err)
	}
}

func TestRequestJoinGuild_AlreadyInGuild(t *testing.T) {
	q := &requestJoinStub{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return db.Guild{ID: 7, Status: string(_guild.GuildStatusActive)}, nil
		},
		onGetKingdomGuildMembership: func(_ context.Context, _ int) (db.GetKingdomGuildMembershipRow, error) {
			return db.GetKingdomGuildMembershipRow{GuildID: 99}, nil
		},
	}
	h := &handler{queries: q}
	_, _, err := h.requestJoinGuild(context.Background(), 1, "knights")
	if !errors.Is(err, ErrAlreadyInGuild) {
		t.Fatalf("err = %v, want ErrAlreadyInGuild", err)
	}
}

func TestRequestJoinGuild_GuildFull(t *testing.T) {
	q := &requestJoinStub{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return db.Guild{ID: 7, Status: string(_guild.GuildStatusActive)}, nil
		},
		onGetKingdomGuildMembership: func(_ context.Context, _ int) (db.GetKingdomGuildMembershipRow, error) {
			return db.GetKingdomGuildMembershipRow{}, pgx.ErrNoRows
		},
		onRequestJoinIfNotFull: func(_ context.Context, _ db.RequestJoinIfNotFullParams) (db.GuildMembership, error) {
			return db.GuildMembership{}, pgx.ErrNoRows
		},
	}
	h := &handler{queries: q}
	_, _, err := h.requestJoinGuild(context.Background(), 1, "knights")
	if !errors.Is(err, ErrGuildFull) {
		t.Fatalf("err = %v, want ErrGuildFull", err)
	}
}

func TestRequestJoinGuild_DuplicateRequest(t *testing.T) {
	q := &requestJoinStub{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return db.Guild{ID: 7, Status: string(_guild.GuildStatusActive)}, nil
		},
		onGetKingdomGuildMembership: func(_ context.Context, _ int) (db.GetKingdomGuildMembershipRow, error) {
			return db.GetKingdomGuildMembershipRow{}, pgx.ErrNoRows
		},
		onRequestJoinIfNotFull: func(_ context.Context, _ db.RequestJoinIfNotFullParams) (db.GuildMembership, error) {
			return db.GuildMembership{}, &pgconn.PgError{Code: pgerrcode.UniqueViolation}
		},
	}
	h := &handler{queries: q}
	_, _, err := h.requestJoinGuild(context.Background(), 1, "knights")
	if !errors.Is(err, ErrDuplicateRequest) {
		t.Fatalf("err = %v, want ErrDuplicateRequest", err)
	}
}

// ── approveMember / rejectMember ──────────────────────────────────────────────

// guildAndRoleStub satisfies getGuildAndViewerRole's two queries plus any
// extra methods needed by the orchestrator under test.
type guildAndRoleStub struct {
	db.Querier
	onGetGuildBySlug                 func(ctx context.Context, slug string) (db.Guild, error)
	onGetMembershipByKingdomAndGuild func(ctx context.Context, arg db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error)
	onApproveMembershipIfNotFull     func(ctx context.Context, arg db.ApproveMembershipIfNotFullParams) (int, error)
	onGetMembershipByID              func(ctx context.Context, arg db.GetMembershipByIDParams) (db.GuildMembership, error)
	onRejectMembership               func(ctx context.Context, arg db.RejectMembershipParams) error
}

func (s *guildAndRoleStub) GetGuildBySlug(ctx context.Context, slug string) (db.Guild, error) {
	return s.onGetGuildBySlug(ctx, slug)
}
func (s *guildAndRoleStub) GetMembershipByKingdomAndGuild(ctx context.Context, arg db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
	return s.onGetMembershipByKingdomAndGuild(ctx, arg)
}
func (s *guildAndRoleStub) ApproveMembershipIfNotFull(ctx context.Context, arg db.ApproveMembershipIfNotFullParams) (int, error) {
	return s.onApproveMembershipIfNotFull(ctx, arg)
}
func (s *guildAndRoleStub) GetMembershipByID(ctx context.Context, arg db.GetMembershipByIDParams) (db.GuildMembership, error) {
	return s.onGetMembershipByID(ctx, arg)
}
func (s *guildAndRoleStub) RejectMembership(ctx context.Context, arg db.RejectMembershipParams) error {
	return s.onRejectMembership(ctx, arg)
}

func leaderRoleStub(guildID int) func(context.Context, db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
	return func(_ context.Context, _ db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
		return db.GuildMembership{GuildID: guildID, Role: string(_guild.RoleLeader)}, nil
	}
}

func memberRoleStub(guildID int) func(context.Context, db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
	return func(_ context.Context, _ db.GetMembershipByKingdomAndGuildParams) (db.GuildMembership, error) {
		return db.GuildMembership{GuildID: guildID, Role: string(_guild.RoleMember)}, nil
	}
}

func TestApproveMember_Success(t *testing.T) {
	q := &guildAndRoleStub{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return db.Guild{ID: 7, Name: "Knights"}, nil
		},
		onGetMembershipByKingdomAndGuild: leaderRoleStub(7),
		onApproveMembershipIfNotFull: func(_ context.Context, _ db.ApproveMembershipIfNotFullParams) (int, error) {
			return 42, nil
		},
	}
	h := &handler{queries: q}
	approved, name, err := h.approveMember(context.Background(), 1, "knights", 99)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if approved != 42 {
		t.Errorf("approved = %d, want 42", approved)
	}
	if name != "Knights" {
		t.Errorf("name = %q, want %q", name, "Knights")
	}
}

func TestApproveMember_NotAuthorized(t *testing.T) {
	q := &guildAndRoleStub{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return db.Guild{ID: 7}, nil
		},
		onGetMembershipByKingdomAndGuild: memberRoleStub(7),
	}
	h := &handler{queries: q}
	_, _, err := h.approveMember(context.Background(), 1, "knights", 99)
	if !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("err = %v, want ErrNotAuthorized", err)
	}
}

func TestApproveMember_GuildFullOrRequestGone(t *testing.T) {
	q := &guildAndRoleStub{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return db.Guild{ID: 7}, nil
		},
		onGetMembershipByKingdomAndGuild: leaderRoleStub(7),
		onApproveMembershipIfNotFull: func(_ context.Context, _ db.ApproveMembershipIfNotFullParams) (int, error) {
			return 0, pgx.ErrNoRows
		},
	}
	h := &handler{queries: q}
	_, _, err := h.approveMember(context.Background(), 1, "knights", 99)
	if !errors.Is(err, ErrGuildFullOrRequestGone) {
		t.Fatalf("err = %v, want ErrGuildFullOrRequestGone", err)
	}
}

func TestApproveMember_TargetInOtherGuild(t *testing.T) {
	q := &guildAndRoleStub{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return db.Guild{ID: 7}, nil
		},
		onGetMembershipByKingdomAndGuild: leaderRoleStub(7),
		onApproveMembershipIfNotFull: func(_ context.Context, _ db.ApproveMembershipIfNotFullParams) (int, error) {
			return 0, &pgconn.PgError{Code: pgerrcode.UniqueViolation}
		},
	}
	h := &handler{queries: q}
	_, _, err := h.approveMember(context.Background(), 1, "knights", 99)
	if !errors.Is(err, ErrTargetInOtherGuild) {
		t.Fatalf("err = %v, want ErrTargetInOtherGuild", err)
	}
}

func TestApproveMember_GuildNotFound(t *testing.T) {
	q := &guildAndRoleStub{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return db.Guild{}, pgx.ErrNoRows
		},
	}
	h := &handler{queries: q}
	_, _, err := h.approveMember(context.Background(), 1, "knights", 99)
	if !errors.Is(err, ErrGuildNotFound) {
		t.Fatalf("err = %v, want ErrGuildNotFound", err)
	}
}

func TestRejectMember_Success(t *testing.T) {
	q := &guildAndRoleStub{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return db.Guild{ID: 7, Name: "Knights"}, nil
		},
		onGetMembershipByKingdomAndGuild: leaderRoleStub(7),
		onGetMembershipByID: func(_ context.Context, _ db.GetMembershipByIDParams) (db.GuildMembership, error) {
			return db.GuildMembership{KingdomID: 42, Role: string(_guild.RolePendingApproval)}, nil
		},
		onRejectMembership: func(_ context.Context, _ db.RejectMembershipParams) error { return nil },
	}
	h := &handler{queries: q}
	rejected, name, err := h.rejectMember(context.Background(), 1, "knights", 99)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if rejected != 42 {
		t.Errorf("rejected = %d, want 42", rejected)
	}
	if name != "Knights" {
		t.Errorf("name = %q, want %q", name, "Knights")
	}
}

func TestRejectMember_NotAuthorized(t *testing.T) {
	q := &guildAndRoleStub{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return db.Guild{ID: 7}, nil
		},
		onGetMembershipByKingdomAndGuild: memberRoleStub(7),
	}
	h := &handler{queries: q}
	_, _, err := h.rejectMember(context.Background(), 1, "knights", 99)
	if !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("err = %v, want ErrNotAuthorized", err)
	}
}

func TestRejectMember_MembershipNotFound(t *testing.T) {
	q := &guildAndRoleStub{
		onGetGuildBySlug: func(_ context.Context, _ string) (db.Guild, error) {
			return db.Guild{ID: 7}, nil
		},
		onGetMembershipByKingdomAndGuild: leaderRoleStub(7),
		onGetMembershipByID: func(_ context.Context, _ db.GetMembershipByIDParams) (db.GuildMembership, error) {
			return db.GuildMembership{}, pgx.ErrNoRows
		},
	}
	h := &handler{queries: q}
	_, _, err := h.rejectMember(context.Background(), 1, "knights", 99)
	if !errors.Is(err, ErrMembershipNotFound) {
		t.Fatalf("err = %v, want ErrMembershipNotFound", err)
	}
}
