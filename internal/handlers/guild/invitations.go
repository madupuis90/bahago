package guild

import (
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

type inviteSignals struct {
	InviteKingdomName string `json:"invite_kingdom_name"`
}

func (h *handler) handleAcceptInvitation() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")
		sse := datastar.NewSSE(w, r)

		invitationID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("invalid invitation id"))))
			return
		}

		g, err := h.queries.GetGuildBySlug(r.Context(), slug)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				sse.PatchElementGostar(guildAlert(AlertError(errors.New("guild not found"))))
				return
			}
			log.Printf("guild accept invitation: get guild: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		_, err = h.queries.AcceptGuildInvitation(r.Context(), db.AcceptGuildInvitationParams{
			InvitationID: invitationID,
			KingdomID:    kingdom.ID,
			GuildID:      g.ID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				sse.PatchElementGostar(guildAlert(AlertError(errors.New("this invitation is no longer valid or the guild is full"))))
				return
			}
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
				sse.PatchElementGostar(guildAlert(AlertError(errors.New("you are already committed to a guild"))))
				return
			}
			log.Printf("guild accept invitation: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		// Notify guild managers of the new member.
		if members, err := h.queries.ListGuildMembersWithNames(r.Context(), g.ID); err == nil {
			managerIDs := make([]int, 0)
			for _, m := range members {
				if _guild.MemberRole(m.Role).CanManage() {
					managerIDs = append(managerIDs, m.KingdomID)
				}
			}
			h.sendNotifications(r, kingdom.ID, managerIDs,
				"New Member Joined",
				fmt.Sprintf("%s has accepted their invitation and joined %s.", kingdom.Name, g.Name),
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

		invitationID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("invalid invitation id"))))
			return
		}

		guildID, err := h.queries.DeclineGuildInvitation(r.Context(), db.DeclineGuildInvitationParams{
			ID:        invitationID,
			KingdomID: kingdom.ID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Already gone; redirect silently.
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
		h.publishUpdates(r, refreshIDs)
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
		sse := datastar.NewSSE(w, r)

		name := strings.TrimSpace(input.InviteKingdomName)
		if name == "" {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("please enter a kingdom name"))))
			return
		}

		g, viewerRole, err := h.getGuildAndViewerRole(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("guild not found"))))
			return
		}
		if err != nil {
			log.Printf("guild send invitation: get guild: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}
		if !viewerRole.CanManage() {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("not authorized"))))
			return
		}

		target, err := h.queries.GetKingdomByName(r.Context(), name)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				sse.PatchElementGostar(guildAlert(AlertError(fmt.Errorf("kingdom %q not found", name))))
				return
			}
			log.Printf("guild send invitation: get kingdom: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		if target.ID == kingdom.ID {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("you cannot invite yourself"))))
			return
		}

		// Reject if the kingdom already has any membership row for this guild.
		if existing, err := h.queries.GetMembershipByKingdomAndGuild(r.Context(), db.GetMembershipByKingdomAndGuildParams{
			KingdomID: target.ID,
			GuildID:   g.ID,
		}); err == nil {
			switch _guild.MemberRole(existing.Role) {
			case _guild.RoleInvited:
				sse.PatchElementGostar(guildAlert(AlertError(fmt.Errorf("%s has already been invited to this guild", target.Name))))
			case _guild.RolePendingApproval:
				sse.PatchElementGostar(guildAlert(AlertError(fmt.Errorf("%s already has a pending join request — approve it instead", target.Name))))
			default:
				sse.PatchElementGostar(guildAlert(AlertError(fmt.Errorf("%s is already a member of this guild", target.Name))))
			}
			return
		} else if !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("guild send invitation: check membership: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		// Reject if the target is already committed to a different guild.
		if _, err := h.queries.GetKingdomGuildMembership(r.Context(), target.ID); err == nil {
			sse.PatchElementGostar(guildAlert(AlertError(fmt.Errorf("%s is already a member of another guild", target.Name))))
			return
		} else if !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("guild send invitation: check target commitment: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		if err := h.queries.CreateGuildInvitation(r.Context(), db.CreateGuildInvitationParams{
			GuildID:   g.ID,
			KingdomID: target.ID,
		}); err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
				sse.PatchElementGostar(guildAlert(AlertError(fmt.Errorf("%s has already been invited to this guild", target.Name))))
				return
			}
			log.Printf("guild send invitation: create: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		h.sendNotifications(r, kingdom.ID, []int{target.ID},
			"Guild Invitation from "+g.Name,
			fmt.Sprintf("You have been invited to join %s. Visit the guild page to accept or decline.", g.Name),
			slugURL(routes.GuildViewPath, slug), "Visit Guild",
		)

		// Publish to all managers so their pending invitations panel refreshes.
		if members, err := h.queries.ListGuildMembersWithNames(r.Context(), g.ID); err == nil {
			managerIDs := make([]int, 0)
			for _, m := range members {
				if _guild.MemberRole(m.Role).CanManage() {
					managerIDs = append(managerIDs, m.KingdomID)
				}
			}
			h.publishUpdates(r, managerIDs)
		}
	}
}

func (h *handler) handleRevokeInvitation() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")
		sse := datastar.NewSSE(w, r)

		invitationID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("invalid invitation id"))))
			return
		}

		g, viewerRole, err := h.getGuildAndViewerRole(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("guild not found"))))
			return
		}
		if err != nil {
			log.Printf("guild revoke invitation: get guild: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}
		if !viewerRole.CanManage() {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("not authorized"))))
			return
		}

		invitedKingdomID, err := h.queries.RevokeGuildInvitation(r.Context(), db.RevokeGuildInvitationParams{
			ID:      invitationID,
			GuildID: g.ID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Already gone; refresh the actor's page to reflect current state.
				h.publishUpdates(r, []int{kingdom.ID})
				return
			}
			log.Printf("guild revoke invitation: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		// Publish to the invited kingdom and all managers so their UI refreshes.
		toNotify := []int{invitedKingdomID}
		if members, err := h.queries.ListGuildMembersWithNames(r.Context(), g.ID); err == nil {
			for _, m := range members {
				if _guild.MemberRole(m.Role).CanManage() {
					toNotify = append(toNotify, m.KingdomID)
				}
			}
		}
		h.publishUpdates(r, toNotify)
	}
}
