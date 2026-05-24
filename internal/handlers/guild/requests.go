package guild

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/starfederation/datastar-go/datastar"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	_guild "bahago/internal/guild"
	. "bahago/internal/ui"
	"bahago/internal/routes"
)

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrInvalidMembershipID    = errors.New("invalid membership id")
	ErrGuildNotActive         = errors.New("this guild is not accepting requests")
	ErrGuildFull              = errors.New("this guild is full")
	ErrDuplicateRequest       = errors.New("you already have a pending request to this guild")
	ErrGuildFullOrRequestGone = errors.New("guild is full or request no longer exists")
	ErrTargetInOtherGuild     = errors.New("this kingdom is already committed to another guild")
	ErrMembershipNotFound     = errors.New("request not found")
)

// ── Validation ────────────────────────────────────────────────────────────────

func validateMembershipID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil || id <= 0 {
		return 0, ErrInvalidMembershipID
	}
	return id, nil
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handleRequestJoin() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")
		sse := datastar.NewSSE(w, r)

		guildID, guildName, err := h.requestJoinGuild(r.Context(), kingdom.ID, slug)
		if err != nil {
			if isRequestJoinUserError(err) {
				sse.PatchElementGostar(guildAlert(AlertError(err)))
				return
			}
			log.Printf("guild request join: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		// Notify guild managers that a new join request has arrived.
		if members, err := h.queries.ListGuildMembersWithNames(r.Context(), guildID); err == nil {
			managerIDs := make([]int, 0)
			for _, m := range members {
				if _guild.MemberRole(m.Role).CanManage() {
					managerIDs = append(managerIDs, m.KingdomID)
				}
			}
			h.sendNotifications(r.Context(), kingdom.ID, managerIDs,
				"New Join Request",
				kingdom.Name+" has requested to join "+guildName+".",
				slugURL(routes.GuildManagePath, slug), "Manage Guild",
			)
		}

		h.publishUpdates(r.Context(), []int{kingdom.ID})
	}
}

func (h *handler) handleCancelRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")
		sse := datastar.NewSSE(w, r)

		g, err := h.queries.GetGuildBySlug(r.Context(), slug)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				sse.PatchElementGostar(guildAlert(AlertError(errors.New("guild not found"))))
				return
			}
			log.Printf("guild cancel request: get guild: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		if err := h.queries.CancelJoinRequest(r.Context(), db.CancelJoinRequestParams{
			KingdomID: kingdom.ID,
			GuildID:   g.ID,
		}); err != nil {
			log.Printf("guild cancel request: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		// Push a live refresh to managers and the actor so their UI updates.
		toNotify := []int{kingdom.ID}
		if members, err := h.queries.ListGuildMembersWithNames(r.Context(), g.ID); err == nil {
			for _, m := range members {
				if _guild.MemberRole(m.Role).CanManage() {
					toNotify = append(toNotify, m.KingdomID)
				}
			}
		}
		h.publishUpdates(r.Context(), toNotify)
	}
}

func (h *handler) handleApprove() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")

		membershipID, err := validateMembershipID(r.PathValue("id"))
		if err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(err)))
			return
		}

		approvedKingdomID, guildName, err := h.approveMember(r.Context(), kingdom.ID, slug, membershipID)
		if err != nil {
			if isApproveUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(err)))
				return
			}
			log.Printf("guild approve: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		// Notify applicant via a message from the approving officer/leader's kingdom.
		h.sendNotifications(r.Context(), kingdom.ID, []int{approvedKingdomID},
			"Guild Application Accepted",
			fmt.Sprintf("Your request to join %s has been accepted. Welcome!", guildName),
			slugURL(routes.GuildViewPath, slug), "Visit Guild",
		)

		h.publishUpdates(r.Context(), []int{kingdom.ID})
	}
}

func (h *handler) handleReject() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")

		membershipID, err := validateMembershipID(r.PathValue("id"))
		if err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(err)))
			return
		}

		rejectedKingdomID, guildName, err := h.rejectMember(r.Context(), kingdom.ID, slug, membershipID)
		if err != nil {
			if isRejectUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(err)))
				return
			}
			log.Printf("guild reject: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		h.sendNotifications(r.Context(), kingdom.ID, []int{rejectedKingdomID},
			"Guild Application Declined",
			fmt.Sprintf("Your request to join %s has been declined.", guildName),
			"", "",
		)

		h.publishUpdates(r.Context(), []int{kingdom.ID})
	}
}

// ── Orchestration ─────────────────────────────────────────────────────────────

// requestJoinGuild creates a pending-approval membership row for the kingdom on
// the active guild identified by slug. Returns the guild ID so the caller can
// fan out manager notifications.
func (h *handler) requestJoinGuild(ctx context.Context, kingdomID int, slug string) (int, string, error) {
	g, err := h.queries.GetGuildBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", ErrGuildNotFound
		}
		return 0, "", fmt.Errorf("get guild: %w", err)
	}
	if !_guild.GuildStatus(g.Status).IsActive() {
		return 0, "", ErrGuildNotActive
	}

	if _, err := h.queries.GetKingdomGuildMembership(ctx, kingdomID); err == nil {
		return 0, "", ErrAlreadyInGuild
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return 0, "", fmt.Errorf("check membership: %w", err)
	}

	if _, err := h.queries.RequestJoinIfNotFull(ctx, db.RequestJoinIfNotFullParams{
		GuildID:   g.ID,
		KingdomID: kingdomID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", ErrGuildFull
		}
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
			return 0, "", ErrDuplicateRequest
		}
		return 0, "", fmt.Errorf("create membership: %w", err)
	}
	return g.ID, g.Name, nil
}

// approveMember promotes a pending-approval membership to a full member when
// the actor has manage rights. Returns the approved kingdom ID and the guild
// name so the caller can send a notification.
func (h *handler) approveMember(ctx context.Context, actorKingdomID int, slug string, membershipID int) (int, string, error) {
	g, viewerRole, err := h.getGuildAndViewerRole(ctx, slug, actorKingdomID)
	if errors.Is(err, ErrGuildNotFound) {
		return 0, "", ErrGuildNotFound
	}
	if err != nil {
		return 0, "", fmt.Errorf("get guild: %w", err)
	}
	if !viewerRole.CanManage() {
		return 0, "", ErrNotAuthorized
	}

	approvedKingdomID, err := h.queries.ApproveMembershipIfNotFull(ctx, db.ApproveMembershipIfNotFullParams{
		MembershipID: membershipID,
		GuildID:      g.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", ErrGuildFullOrRequestGone
		}
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
			return 0, "", ErrTargetInOtherGuild
		}
		return 0, "", fmt.Errorf("approve membership: %w", err)
	}
	return approvedKingdomID, g.Name, nil
}

// rejectMember marks a pending-approval membership as rejected when the actor
// has manage rights. Returns the rejected kingdom ID and the guild name so the
// caller can send a notification.
func (h *handler) rejectMember(ctx context.Context, actorKingdomID int, slug string, membershipID int) (int, string, error) {
	g, viewerRole, err := h.getGuildAndViewerRole(ctx, slug, actorKingdomID)
	if errors.Is(err, ErrGuildNotFound) {
		return 0, "", ErrGuildNotFound
	}
	if err != nil {
		return 0, "", fmt.Errorf("get guild: %w", err)
	}
	if !viewerRole.CanManage() {
		return 0, "", ErrNotAuthorized
	}

	membership, err := h.queries.GetMembershipByID(ctx, db.GetMembershipByIDParams{
		ID:      membershipID,
		GuildID: g.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", ErrMembershipNotFound
		}
		return 0, "", fmt.Errorf("get membership: %w", err)
	}

	if err := h.queries.RejectMembership(ctx, db.RejectMembershipParams{
		MembershipID: membershipID,
		GuildID:      g.ID,
	}); err != nil {
		return 0, "", fmt.Errorf("reject membership: %w", err)
	}
	return membership.KingdomID, g.Name, nil
}

func isRequestJoinUserError(err error) bool {
	return errors.Is(err, ErrGuildNotFound) ||
		errors.Is(err, ErrGuildNotActive) ||
		errors.Is(err, ErrAlreadyInGuild) ||
		errors.Is(err, ErrGuildFull) ||
		errors.Is(err, ErrDuplicateRequest)
}

func isApproveUserError(err error) bool {
	return errors.Is(err, ErrGuildNotFound) ||
		errors.Is(err, ErrNotAuthorized) ||
		errors.Is(err, ErrGuildFullOrRequestGone) ||
		errors.Is(err, ErrTargetInOtherGuild)
}

func isRejectUserError(err error) bool {
	return errors.Is(err, ErrGuildNotFound) ||
		errors.Is(err, ErrNotAuthorized) ||
		errors.Is(err, ErrMembershipNotFound)
}
