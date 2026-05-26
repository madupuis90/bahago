package guild

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

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

// ── Input structs ─────────────────────────────────────────────────────────────

type inviteSignals struct {
	InviteKingdomName string `json:"invite_kingdom_name"`
}

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrInvalidInvitationID     = errors.New("invalid invitation id")
	ErrInviteNameRequired      = errors.New("please enter a kingdom name")
	ErrKingdomNotFound         = errors.New("kingdom not found")
	ErrSelfInvite              = errors.New("you cannot invite yourself")
	ErrAlreadyInvited          = errors.New("already invited")
	ErrTargetHasPendingRequest = errors.New("kingdom has a pending join request — approve it instead")
	ErrTargetAlreadyMember     = errors.New("already a member of this guild")
	ErrTargetInAnotherGuild    = errors.New("already a member of another guild")
	ErrInvitationInvalid       = errors.New("this invitation is no longer valid or the guild is full")
	ErrInvitationGone          = errors.New("invitation no longer exists")
)

// ── Validation ────────────────────────────────────────────────────────────────

func validateInvitationID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil || id <= 0 {
		return 0, ErrInvalidInvitationID
	}
	return id, nil
}

func validateInviteInput(in *inviteSignals) []error {
	var errs []error
	if strings.TrimSpace(in.InviteKingdomName) == "" {
		errs = append(errs, ErrInviteNameRequired)
	}
	return errs
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handleAcceptInvitation() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")
		sse := datastar.NewSSE(w, r)

		invitationID, err := validateInvitationID(r.PathValue("id"))
		if err != nil {
			sse.PatchElementGostar(guildAlert(AlertError(err)))
			return
		}

		guildID, guildName, err := h.acceptInvitation(r.Context(), kingdom.ID, slug, invitationID)
		if err != nil {
			if isAcceptInvitationUserError(err) {
				sse.PatchElementGostar(guildAlert(AlertError(err)))
				return
			}
			log.Printf("guild accept invitation: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		// Notify guild managers of the new member.
		if members, err := h.queries.ListGuildMembersWithNames(r.Context(), guildID); err == nil {
			managerIDs := make([]int, 0)
			for _, m := range members {
				if _guild.MemberRole(m.Role).CanManage() {
					managerIDs = append(managerIDs, m.KingdomID)
				}
			}
			h.sendNotifications(r.Context(), kingdom.ID, managerIDs,
				"New Member Joined",
				fmt.Sprintf("%s has accepted their invitation and joined %s.", kingdom.Name, guildName),
				slugURL(routes.GuildViewPath, slug), "Visit Guild",
			)
		}

		if err := sse.Redirect(slugURL(routes.GuildViewPath, slug)); err != nil {
			log.Printf("guild accept invitation: redirect: %v", err)
		}
	}
}

func (h *handler) handleDeclineInvitation() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")
		sse := datastar.NewSSE(w, r)

		invitationID, err := validateInvitationID(r.PathValue("id"))
		if err != nil {
			sse.PatchElementGostar(guildAlert(AlertError(err)))
			return
		}

		guildID, err := h.declineInvitation(r.Context(), kingdom.ID, invitationID)
		if err != nil {
			// "Already gone" is silently treated as success — preserve the
			// existing redirect-only UX.
			if errors.Is(err, ErrInvitationGone) {
				if err := sse.Redirect(slugURL(routes.GuildViewPath, slug)); err != nil {
					log.Printf("guild decline invitation: redirect: %v", err)
				}
				return
			}
			log.Printf("guild decline invitation: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		// Publish to the decliner and guild managers so their UI refreshes.
		refreshIDs := []int{kingdom.ID}
		if members, err := h.queries.ListGuildMembersWithNames(r.Context(), guildID); err == nil {
			for _, m := range members {
				if _guild.MemberRole(m.Role).CanManage() {
					refreshIDs = append(refreshIDs, m.KingdomID)
				}
			}
		}
		h.publishUpdates(r.Context(), refreshIDs)
	}
}

func (h *handler) handleSendInvitation() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")

		input := &inviteSignals{}
		if err := datastar.ReadSignals(r, input); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(errors.New("invalid request"))))
			return
		}

		if errs := validateInviteInput(input); len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(errs...)))
			return
		}

		targetKingdomID, guildID, guildName, err := h.sendInvitation(r.Context(), kingdom, slug, input.InviteKingdomName)
		if err != nil {
			if isSendInvitationUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(err)))
				return
			}
			log.Printf("guild send invitation: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		// Send the in-game invitation message to the target.
		h.sendNotifications(r.Context(), kingdom.ID, []int{targetKingdomID},
			"Guild Invitation from "+guildName,
			fmt.Sprintf("You have been invited to join %s. Visit the guild page to accept or decline.", guildName),
			slugURL(routes.GuildViewPath, slug), "Visit Guild",
		)

		// Publish to all managers so their pending invitations panel refreshes.
		if members, err := h.queries.ListGuildMembersWithNames(r.Context(), guildID); err == nil {
			managerIDs := make([]int, 0)
			for _, m := range members {
				if _guild.MemberRole(m.Role).CanManage() {
					managerIDs = append(managerIDs, m.KingdomID)
				}
			}
			h.publishUpdates(r.Context(), managerIDs)
		}
	}
}

func (h *handler) handleRevokeInvitation() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")
		sse := datastar.NewSSE(w, r)

		invitationID, err := validateInvitationID(r.PathValue("id"))
		if err != nil {
			sse.PatchElementGostar(guildAlert(AlertError(err)))
			return
		}

		invitedKingdomID, guildID, err := h.revokeInvitation(r.Context(), kingdom.ID, slug, invitationID)
		if err != nil {
			// "Already gone" silently refreshes the actor — preserve existing UX.
			if errors.Is(err, ErrInvitationGone) {
				h.publishUpdates(r.Context(), []int{kingdom.ID})
				return
			}
			if isRevokeInvitationUserError(err) {
				sse.PatchElementGostar(guildAlert(AlertError(err)))
				return
			}
			log.Printf("guild revoke invitation: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		// Publish to the invited kingdom and all managers so their UI refreshes.
		toNotify := []int{invitedKingdomID}
		if members, err := h.queries.ListGuildMembersWithNames(r.Context(), guildID); err == nil {
			for _, m := range members {
				if _guild.MemberRole(m.Role).CanManage() {
					toNotify = append(toNotify, m.KingdomID)
				}
			}
		}
		h.publishUpdates(r.Context(), toNotify)
	}
}

// ── Orchestration ─────────────────────────────────────────────────────────────

// acceptInvitation promotes the invited membership to a full member. Returns
// the guild ID and name so the caller can fan out a notification to managers.
func (h *handler) acceptInvitation(ctx context.Context, kingdomID int, slug string, invitationID int) (guildID int, guildName string, err error) {
	g, err := h.queries.GetGuildBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", ErrGuildNotFound
		}
		return 0, "", fmt.Errorf("get guild: %w", err)
	}

	if _, err := h.queries.AcceptGuildInvitation(ctx, db.AcceptGuildInvitationParams{
		InvitationID: invitationID,
		KingdomID:    kingdomID,
		GuildID:      g.ID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, "", ErrInvitationInvalid
		}
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
			return 0, "", ErrAlreadyInGuild
		}
		return 0, "", fmt.Errorf("accept invitation: %w", err)
	}
	return g.ID, g.Name, nil
}

// declineInvitation removes the invitation row. Returns the guild ID so the
// caller can refresh manager UIs, or ErrInvitationGone if the invitation no
// longer exists (the handler treats that as a silent success).
func (h *handler) declineInvitation(ctx context.Context, kingdomID, invitationID int) (int, error) {
	guildID, err := h.queries.DeclineGuildInvitation(ctx, db.DeclineGuildInvitationParams{
		ID:        invitationID,
		KingdomID: kingdomID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrInvitationGone
		}
		return 0, fmt.Errorf("decline invitation: %w", err)
	}
	return guildID, nil
}

// sendInvitation creates a guild invitation for the named target kingdom. The
// branched user-facing errors use the wrapped-sentinel idiom so each carries
// the target name. Returns the target kingdom ID, guild ID, and guild name so
// the caller can fan out notifications.
func (h *handler) sendInvitation(ctx context.Context, actorKingdom *db.Kingdom, slug, targetName string) (targetKingdomID, guildID int, guildName string, err error) {
	name := strings.TrimSpace(targetName)

	g, viewerRole, err := h.getGuildAndViewerRole(ctx, slug, actorKingdom.ID)
	if errors.Is(err, ErrGuildNotFound) {
		return 0, 0, "", ErrGuildNotFound
	}
	if err != nil {
		return 0, 0, "", fmt.Errorf("get guild: %w", err)
	}
	if !viewerRole.CanManage() {
		return 0, 0, "", ErrNotAuthorized
	}

	target, err := h.queries.GetKingdomByName(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, "", fmt.Errorf("%w: %s", ErrKingdomNotFound, name)
		}
		return 0, 0, "", fmt.Errorf("get kingdom: %w", err)
	}

	if target.ID == actorKingdom.ID {
		return 0, 0, "", ErrSelfInvite
	}

	// Reject if the kingdom already has any membership row for this guild.
	if existing, err := h.queries.GetMembershipByKingdomAndGuild(ctx, db.GetMembershipByKingdomAndGuildParams{
		KingdomID: target.ID,
		GuildID:   g.ID,
	}); err == nil {
		switch _guild.MemberRole(existing.Role) {
		case _guild.RoleInvited:
			return 0, 0, "", fmt.Errorf("%w: %s", ErrAlreadyInvited, target.Name)
		case _guild.RolePendingApproval:
			return 0, 0, "", fmt.Errorf("%w: %s", ErrTargetHasPendingRequest, target.Name)
		default:
			return 0, 0, "", fmt.Errorf("%w: %s", ErrTargetAlreadyMember, target.Name)
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, "", fmt.Errorf("check membership: %w", err)
	}

	// Reject if the target is already committed to a different guild.
	if _, err := h.queries.GetKingdomGuildMembership(ctx, target.ID); err == nil {
		return 0, 0, "", fmt.Errorf("%w: %s", ErrTargetInAnotherGuild, target.Name)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, "", fmt.Errorf("check target commitment: %w", err)
	}

	if err := h.queries.CreateGuildInvitation(ctx, db.CreateGuildInvitationParams{
		GuildID:   g.ID,
		KingdomID: target.ID,
	}); err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
			return 0, 0, "", fmt.Errorf("%w: %s", ErrAlreadyInvited, target.Name)
		}
		return 0, 0, "", fmt.Errorf("create invitation: %w", err)
	}
	return target.ID, g.ID, g.Name, nil
}

// revokeInvitation cancels a guild invitation. Returns the invited kingdom ID
// and guild ID so the caller can refresh affected UIs. Returns ErrInvitationGone
// when the invitation no longer exists (handler treats that as a silent refresh).
func (h *handler) revokeInvitation(ctx context.Context, actorKingdomID int, slug string, invitationID int) (invitedKingdomID, guildID int, err error) {
	g, viewerRole, err := h.getGuildAndViewerRole(ctx, slug, actorKingdomID)
	if errors.Is(err, ErrGuildNotFound) {
		return 0, 0, ErrGuildNotFound
	}
	if err != nil {
		return 0, 0, fmt.Errorf("get guild: %w", err)
	}
	if !viewerRole.CanManage() {
		return 0, 0, ErrNotAuthorized
	}

	invitedKingdomID, err = h.queries.RevokeGuildInvitation(ctx, db.RevokeGuildInvitationParams{
		ID:      invitationID,
		GuildID: g.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, ErrInvitationGone
		}
		return 0, 0, fmt.Errorf("revoke invitation: %w", err)
	}
	return invitedKingdomID, g.ID, nil
}

func isAcceptInvitationUserError(err error) bool {
	return errors.Is(err, ErrGuildNotFound) ||
		errors.Is(err, ErrInvitationInvalid) ||
		errors.Is(err, ErrAlreadyInGuild)
}

func isSendInvitationUserError(err error) bool {
	return errors.Is(err, ErrGuildNotFound) ||
		errors.Is(err, ErrNotAuthorized) ||
		errors.Is(err, ErrKingdomNotFound) ||
		errors.Is(err, ErrSelfInvite) ||
		errors.Is(err, ErrAlreadyInvited) ||
		errors.Is(err, ErrTargetHasPendingRequest) ||
		errors.Is(err, ErrTargetAlreadyMember) ||
		errors.Is(err, ErrTargetInAnotherGuild)
}

func isRevokeInvitationUserError(err error) bool {
	return errors.Is(err, ErrGuildNotFound) ||
		errors.Is(err, ErrNotAuthorized)
}
