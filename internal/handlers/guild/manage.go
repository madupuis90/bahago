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
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	_guild "bahago/internal/guild"
	. "bahago/internal/layout"
	"bahago/internal/routes"
)

// ── Input structs ─────────────────────────────────────────────────────────────

type editDescriptionSignals struct {
	GuildDescription string `json:"guild_description"`
}

type transferLeaderSignals struct {
	TargetKingdomID int `json:"target_kingdom_id"`
}

type inviteSignals struct {
	InviteKingdomName string `json:"invite_kingdom_name"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handleManage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")

		guild, members, viewerRole, err := h.loadGuildAndMembership(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			http.Error(w, "guild not found", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("guild manage: load: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !viewerRole.CanManage() {
			http.Redirect(w, r, slugURL(routes.GuildViewPath, slug), http.StatusFound)
			return
		}

		pending, err := h.queries.ListPendingRequests(r.Context(), guild.ID)
		if err != nil {
			log.Printf("guild manage: list pending: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		invitations, err := h.queries.ListGuildInvitations(r.Context(), guild.ID)
		if err != nil {
			log.Printf("guild manage: list invitations: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		KingdomLayout(r, "Manage "+guild.Name, r.URL.Path, kingdom,
			guildManageContent(guild, members, viewerRole, pending, invitations),
		).Render(w)
	}
}

func (h *handler) handleManageRefresh() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")

		ch, cleanup := h.hub.Subscribe(kingdom.ID)
		defer cleanup()

		sse := datastar.NewSSE(w, r)
		for {
			select {
			case <-r.Context().Done():
				return
			case k := <-ch:
				guild, members, viewerRole, err := h.loadGuildAndMembership(r, slug, k.ID)
				if errors.Is(err, errGuildNotFound) {
					if err := sse.Redirect(routes.GuildPath); err != nil {
						log.Printf("guild manage refresh: redirect: %v", err)
					}
					return
				}
				if err != nil {
					log.Printf("guild manage refresh: load: %v", err)
					return
				}
				if !viewerRole.CanManage() {
					if err := sse.Redirect(slugURL(routes.GuildViewPath, slug)); err != nil {
						log.Printf("guild manage refresh: redirect: %v", err)
					}
					return
				}
				pending, err := h.queries.ListPendingRequests(r.Context(), guild.ID)
				if err != nil {
					log.Printf("guild manage refresh: list pending: %v", err)
					return
				}
				invitations, err := h.queries.ListGuildInvitations(r.Context(), guild.ID)
				if err != nil {
					log.Printf("guild manage refresh: list invitations: %v", err)
					return
				}
				if err := sse.PatchElementGostar(MainContent(guildManageContent(guild, members, viewerRole, pending, invitations))); err != nil {
					log.Printf("guild manage refresh: patch: %v", err)
					return
				}
			}
		}
	}
}

func (h *handler) handleApprove() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")
		sse := datastar.NewSSE(w, r)

		membershipID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			sse.PatchElementGostar(guildErrorComponent(errors.New("invalid membership id")))
			return
		}

		g, viewerRole, err := h.getGuildAndViewerRole(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			sse.PatchElementGostar(guildErrorComponent(errors.New("guild not found")))
			return
		}
		if err != nil {
			log.Printf("guild approve: get guild: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}
		if !viewerRole.CanManage() {
			sse.PatchElementGostar(guildErrorComponent(errors.New("not authorized")))
			return
		}

		approvedKingdomID, err := h.queries.ApproveMembershipIfNotFull(r.Context(), db.ApproveMembershipIfNotFullParams{
			MembershipID: membershipID,
			GuildID:      g.ID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				sse.PatchElementGostar(guildErrorComponent(errors.New("guild is full or request no longer exists")))
				return
			}
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
				sse.PatchElementGostar(guildErrorComponent(errors.New("this kingdom is already committed to another guild")))
				return
			}
			log.Printf("guild approve: approve membership: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}

		// Notify applicant via a message from the approving officer/leader's kingdom.
		h.sendNotifications(r, kingdom.ID, []int{approvedKingdomID},
			"Guild Application Accepted",
			fmt.Sprintf("Your request to join %s has been accepted. Welcome!", g.Name),
			slugURL(routes.GuildViewPath, slug), "Visit Guild",
		)

		h.publishUpdates(r, []int{kingdom.ID})
	}
}

func (h *handler) handleReject() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")
		sse := datastar.NewSSE(w, r)

		membershipID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			sse.PatchElementGostar(guildErrorComponent(errors.New("invalid membership id")))
			return
		}

		g, viewerRole, err := h.getGuildAndViewerRole(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			sse.PatchElementGostar(guildErrorComponent(errors.New("guild not found")))
			return
		}
		if err != nil {
			log.Printf("guild reject: get guild: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}
		if !viewerRole.CanManage() {
			sse.PatchElementGostar(guildErrorComponent(errors.New("not authorized")))
			return
		}

		membership, err := h.queries.GetMembershipByID(r.Context(), db.GetMembershipByIDParams{
			ID:      membershipID,
			GuildID: g.ID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				sse.PatchElementGostar(guildErrorComponent(errors.New("request not found")))
				return
			}
			log.Printf("guild reject: get membership: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}

		if err := h.queries.RejectMembership(r.Context(), db.RejectMembershipParams{
			MembershipID: membershipID,
			GuildID:      g.ID,
		}); err != nil {
			log.Printf("guild reject: reject membership: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}

		h.sendNotifications(r, kingdom.ID, []int{membership.KingdomID},
			"Guild Application Declined",
			fmt.Sprintf("Your request to join %s has been declined.", g.Name),
			"", "",
		)

		h.publishUpdates(r, []int{kingdom.ID})
	}
}

func (h *handler) handleRemove() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")
		sse := datastar.NewSSE(w, r)

		targetKingdomID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			sse.PatchElementGostar(guildErrorComponent(errors.New("invalid kingdom id")))
			return
		}

		g, viewerRole, err := h.getGuildAndViewerRole(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			sse.PatchElementGostar(guildErrorComponent(errors.New("guild not found")))
			return
		}
		if err != nil {
			log.Printf("guild remove: get guild: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}
		if !viewerRole.CanManage() {
			sse.PatchElementGostar(guildErrorComponent(errors.New("not authorized")))
			return
		}

		target, err := h.queries.GetMembershipByKingdomAndGuild(r.Context(), db.GetMembershipByKingdomAndGuildParams{
			KingdomID: targetKingdomID,
			GuildID:   g.ID,
		})
		if err != nil {
			log.Printf("guild remove: check target role: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}
		if !viewerRole.CanRemoveTarget(_guild.MemberRole(target.Role)) {
			sse.PatchElementGostar(guildErrorComponent(errors.New("officers can only remove regular members")))
			return
		}

		if err := h.queries.RemoveMembership(r.Context(), db.RemoveMembershipParams{
			KingdomID: targetKingdomID,
			GuildID:   g.ID,
		}); err != nil {
			log.Printf("guild remove: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}

		h.sendNotifications(r, kingdom.ID, []int{targetKingdomID},
			"Removed from "+g.Name,
			"You have been removed from "+g.Name+".",
			"", "",
		)

		h.publishUpdates(r, []int{kingdom.ID})
	}
}

func (h *handler) handleLeave() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")
		sse := datastar.NewSSE(w, r)

		g, viewerRole, err := h.getGuildAndViewerRole(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			sse.PatchElementGostar(guildErrorComponent(errors.New("guild not found")))
			return
		}
		if err != nil {
			log.Printf("guild leave: get guild: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}
		if viewerRole != _guild.RoleMember && viewerRole != _guild.RoleOfficer {
			msg := "permission denied"
			if viewerRole == _guild.RoleLeader {
				msg = "the guild leader must transfer leadership before leaving"
			}
			sse.PatchElementGostar(guildErrorComponent(errors.New(msg)))
			return
		}

		if err := h.queries.RemoveMembership(r.Context(), db.RemoveMembershipParams{
			KingdomID: kingdom.ID,
			GuildID:   g.ID,
		}); err != nil {
			log.Printf("guild leave: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}

		// Notify members managers that a member has left.
		if members, err := h.queries.ListGuildMembersWithNames(r.Context(), g.ID); err == nil {
			managerIDs := make([]int, 0)
			for _, m := range members {
				if _guild.MemberRole(m.Role).CanManage() {
					managerIDs = append(managerIDs, m.KingdomID)
				}
			}
			h.sendNotifications(r, kingdom.ID, managerIDs,
				"Member Left",
				kingdom.Name+" has left "+g.Name+".",
				slugURL(routes.GuildViewPath, slug), "Visit Guild",
			)
		}

		if err := sse.Redirect(routes.GuildPath); err != nil {
			log.Printf("guild leave: redirect: %v", err)
		}
	}
}

func (h *handler) handlePromote() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")
		sse := datastar.NewSSE(w, r)

		targetKingdomID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			sse.PatchElementGostar(guildErrorComponent(errors.New("invalid kingdom id")))
			return
		}

		g, members, viewerRole, err := h.loadGuildAndMembership(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			sse.PatchElementGostar(guildErrorComponent(errors.New("guild not found")))
			return
		}
		if err != nil {
			log.Printf("guild promote: load: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}
		if !viewerRole.IsLeader() {
			sse.PatchElementGostar(guildErrorComponent(errors.New("only the guild leader can promote officers")))
			return
		}

		// Enforce the 4-officer cap.
		officerCount := 0
		for _, m := range members {
			if _guild.MemberRole(m.Role) == _guild.RoleOfficer {
				officerCount++
			}
		}
		if officerCount >= 4 {
			sse.PatchElementGostar(guildErrorComponent(errors.New("a guild can have at most 4 officers")))
			return
		}

		if err := h.queries.SetMembershipRole(r.Context(), db.SetMembershipRoleParams{
			Role:      string(_guild.RoleOfficer),
			KingdomID: targetKingdomID,
			GuildID:   g.ID,
		}); err != nil {
			log.Printf("guild promote: set role: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}

		h.sendNotifications(r, kingdom.ID, []int{targetKingdomID},
			"Promoted to Officer",
			"You have been promoted to Officer in "+g.Name+".",
			slugURL(routes.GuildViewPath, slug), "Visit Guild",
		)

		h.publishUpdates(r, []int{kingdom.ID})
	}
}

func (h *handler) handleDemote() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")
		sse := datastar.NewSSE(w, r)

		targetKingdomID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			sse.PatchElementGostar(guildErrorComponent(errors.New("invalid kingdom id")))
			return
		}

		g, viewerRole, err := h.getGuildAndViewerRole(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			sse.PatchElementGostar(guildErrorComponent(errors.New("guild not found")))
			return
		}
		if err != nil {
			log.Printf("guild demote: get guild: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}
		if !viewerRole.CanManage() {
			sse.PatchElementGostar(guildErrorComponent(errors.New("not authorized")))
			return
		}

		demoteTarget, err := h.queries.GetMembershipByKingdomAndGuild(r.Context(), db.GetMembershipByKingdomAndGuildParams{
			KingdomID: targetKingdomID,
			GuildID:   g.ID,
		})
		if err != nil {
			log.Printf("guild demote: check target role: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}
		if !viewerRole.CanRemoveTarget(_guild.MemberRole(demoteTarget.Role)) {
			sse.PatchElementGostar(guildErrorComponent(errors.New("officers cannot demote other officers")))
			return
		}

		if err := h.queries.SetMembershipRole(r.Context(), db.SetMembershipRoleParams{
			Role:      string(_guild.RoleMember),
			KingdomID: targetKingdomID,
			GuildID:   g.ID,
		}); err != nil {
			log.Printf("guild demote: set role: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}

		h.sendNotifications(r, kingdom.ID, []int{targetKingdomID},
			"Demoted to Member",
			"You have been demoted to Member in "+g.Name+".",
			slugURL(routes.GuildViewPath, slug), "Visit Guild",
		)

		h.publishUpdates(r, []int{kingdom.ID})
	}
}

func (h *handler) handleTransferLeadership() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")

		input := &transferLeaderSignals{}
		if err := datastar.ReadSignals(r, input); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(guildErrorComponent(errors.New("invalid request")))
			return
		}
		sse := datastar.NewSSE(w, r)

		g, viewerRole, err := h.getGuildAndViewerRole(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			sse.PatchElementGostar(guildErrorComponent(errors.New("guild not found")))
			return
		}
		if err != nil {
			log.Printf("guild transfer: get guild: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}
		if !viewerRole.IsLeader() {
			sse.PatchElementGostar(guildErrorComponent(errors.New("only the guild leader can transfer leadership")))
			return
		}

		if input.TargetKingdomID == 0 || input.TargetKingdomID == kingdom.ID {
			sse.PatchElementGostar(guildErrorComponent(errors.New("please select a valid member to transfer leadership to")))
			return
		}

		targetMembership, err := h.queries.GetMembershipByKingdomAndGuild(r.Context(), db.GetMembershipByKingdomAndGuildParams{
			KingdomID: input.TargetKingdomID,
			GuildID:   g.ID,
		})
		if err != nil {
			sse.PatchElementGostar(guildErrorComponent(errors.New("target is not a member of this guild")))
			return
		}
		if role := _guild.MemberRole(targetMembership.Role); role != _guild.RoleMember && role != _guild.RoleOfficer {
			sse.PatchElementGostar(guildErrorComponent(errors.New("leadership can only be transferred to a full member or officer")))
			return
		}

		if err := h.queries.TransferLeadership(r.Context(), db.TransferLeadershipParams{
			NewLeaderKingdomID: input.TargetKingdomID,
			GuildID:            g.ID,
		}); err != nil {
			log.Printf("guild transfer leadership: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}

		h.sendNotifications(r, kingdom.ID, []int{input.TargetKingdomID},
			"Guild Leadership Transferred",
			"You are now the leader of "+g.Name+".",
			slugURL(routes.GuildViewPath, slug), "Visit Guild",
		)

		if err := sse.Redirect(slugURL(routes.GuildViewPath, slug)); err != nil {
			log.Printf("guild transfer: redirect: %v", err)
		}
	}
}

func (h *handler) handleDisband() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")
		sse := datastar.NewSSE(w, r)

		g, viewerRole, err := h.getGuildAndViewerRole(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			sse.PatchElementGostar(guildErrorComponent(errors.New("guild not found")))
			return
		}
		if err != nil {
			log.Printf("guild disband: get guild: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}
		if !viewerRole.IsLeader() {
			sse.PatchElementGostar(guildErrorComponent(errors.New("only the guild leader can disband the guild")))
			return
		}

		// Collect members to notify before disbanding (they'll be gone after).
		members, err := h.queries.ListGuildMembersWithNames(r.Context(), g.ID)
		if err != nil {
			log.Printf("guild disband: list members: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}
		memberIDs := make([]int, 0, len(members))
		for _, m := range members {
			if m.KingdomID != kingdom.ID && _guild.MemberRole(m.Role).IsActiveMember() {
				memberIDs = append(memberIDs, m.KingdomID)
			}
		}

		if err := h.queries.DisbandGuild(r.Context(), g.ID); err != nil {
			log.Printf("guild disband: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}

		h.sendNotifications(r, kingdom.ID, memberIDs,
			"Guild Disbanded",
			g.Name+" has been disbanded.",
			"", "",
		)

		if err := sse.Redirect(routes.GuildPath); err != nil {
			log.Printf("guild disband: redirect: %v", err)
		}
	}
}

func (h *handler) handleEditDescription() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")

		input := &editDescriptionSignals{}
		if err := datastar.ReadSignals(r, input); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(guildErrorComponent(errors.New("invalid request")))
			return
		}
		sse := datastar.NewSSE(w, r)

		description := strings.TrimSpace(input.GuildDescription)
		if len(description) > 500 {
			sse.PatchElementGostar(guildErrorComponent(errors.New("description cannot exceed 500 characters")))
			return
		}

		g, viewerRole, err := h.getGuildAndViewerRole(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			sse.PatchElementGostar(guildErrorComponent(errors.New("guild not found")))
			return
		}
		if err != nil {
			log.Printf("guild edit description: get guild: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}
		if !viewerRole.IsLeader() {
			sse.PatchElementGostar(guildErrorComponent(errors.New("only the guild leader can edit the description")))
			return
		}

		if err := h.queries.UpdateGuildDescription(r.Context(), db.UpdateGuildDescriptionParams{
			Description: description,
			ID:          g.ID,
		}); err != nil {
			log.Printf("guild edit description: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}

		h.publishUpdates(r, []int{kingdom.ID})
	}
}

// ── Page components ───────────────────────────────────────────────────────────

func (h *handler) handleSendInvitation() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")

		input := &inviteSignals{}
		if err := datastar.ReadSignals(r, input); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(guildErrorComponent(errors.New("invalid request")))
			return
		}
		sse := datastar.NewSSE(w, r)

		name := strings.TrimSpace(input.InviteKingdomName)
		if name == "" {
			sse.PatchElementGostar(guildErrorComponent(errors.New("please enter a kingdom name")))
			return
		}

		g, viewerRole, err := h.getGuildAndViewerRole(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			sse.PatchElementGostar(guildErrorComponent(errors.New("guild not found")))
			return
		}
		if err != nil {
			log.Printf("guild send invitation: get guild: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}
		if !viewerRole.CanManage() {
			sse.PatchElementGostar(guildErrorComponent(errors.New("not authorized")))
			return
		}

		target, err := h.queries.GetKingdomByName(r.Context(), name)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				sse.PatchElementGostar(guildErrorComponent(fmt.Errorf("kingdom %q not found", name)))
				return
			}
			log.Printf("guild send invitation: get kingdom: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}

		if target.ID == kingdom.ID {
			sse.PatchElementGostar(guildErrorComponent(errors.New("you cannot invite yourself")))
			return
		}

		// Reject if the kingdom already has any membership row for this guild.
		if existing, err := h.queries.GetMembershipByKingdomAndGuild(r.Context(), db.GetMembershipByKingdomAndGuildParams{
			KingdomID: target.ID,
			GuildID:   g.ID,
		}); err == nil {
			switch _guild.MemberRole(existing.Role) {
			case _guild.RoleInvited:
				sse.PatchElementGostar(guildErrorComponent(fmt.Errorf("%s has already been invited to this guild", target.Name)))
			case _guild.RolePendingApproval:
				sse.PatchElementGostar(guildErrorComponent(fmt.Errorf("%s already has a pending join request — approve it instead", target.Name)))
			default:
				sse.PatchElementGostar(guildErrorComponent(fmt.Errorf("%s is already a member of this guild", target.Name)))
			}
			return
		} else if !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("guild send invitation: check membership: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}

		// Reject if the target is already committed to a different guild.
		if _, err := h.queries.GetKingdomGuildMembership(r.Context(), target.ID); err == nil {
			sse.PatchElementGostar(guildErrorComponent(fmt.Errorf("%s is already a member of another guild", target.Name)))
			return
		} else if !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("guild send invitation: check target commitment: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}

		if err := h.queries.CreateGuildInvitation(r.Context(), db.CreateGuildInvitationParams{
			GuildID:   g.ID,
			KingdomID: target.ID,
		}); err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
				sse.PatchElementGostar(guildErrorComponent(fmt.Errorf("%s has already been invited to this guild", target.Name)))
				return
			}
			log.Printf("guild send invitation: create: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
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
			sse.PatchElementGostar(guildErrorComponent(errors.New("invalid invitation id")))
			return
		}

		g, viewerRole, err := h.getGuildAndViewerRole(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			sse.PatchElementGostar(guildErrorComponent(errors.New("guild not found")))
			return
		}
		if err != nil {
			log.Printf("guild revoke invitation: get guild: %v", err)
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
			return
		}
		if !viewerRole.CanManage() {
			sse.PatchElementGostar(guildErrorComponent(errors.New("not authorized")))
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
			sse.PatchElementGostar(guildErrorComponent(errors.New("internal error")))
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

func guildManageContent(g db.Guild, members []db.ListGuildMembersWithNamesRow, viewerRole _guild.MemberRole, pending []db.ListPendingRequestsRow, invitations []db.ListGuildInvitationsRow) Node {
	slug := g.Slug
	isLeader := viewerRole.IsLeader()

	// Build member options for transfer leadership select.
	eligibleMembers := make([]db.ListGuildMembersWithNamesRow, 0)
	for _, m := range members {
		if r := _guild.MemberRole(m.Role); r == _guild.RoleMember || r == _guild.RoleOfficer {
			eligibleMembers = append(eligibleMembers, m)
		}
	}

	return Div(
		H1(Class("page-title"), Text("Manage "+g.Name)),
		Div(ds.Init(GetSSENoSignals("%s", slugURL(routes.GuildManageRefreshPath, slug)))),
		guildErrorComponent(nil),
		A(Href(slugURL(routes.GuildViewPath, slug)), Text("← Back to guild page")),

		// ── Join Requests section
		Iff(len(pending) > 0, func() Node {
			return Div(Class("guild-manage-section panel"),
				P(Class("panel-title"), Text("Join Requests")),
				Table(Class("guild-member-table"),
					THead(Tr(
						Th(Text("Kingdom")),
						Th(Text("Actions")),
					)),
					TBody(Map(pending, func(p db.ListPendingRequestsRow) Node {
						return Tr(
							Td(Text(p.KingdomName)),
							Td(
								Button(Class("btn"), ds.On("click", datastar.PostSSE("%s", memberActionURL(routes.GuildApproveMemberPath, slug, p.ID))), Text("Approve")),
								Button(Class("btn"), ds.On("click", datastar.PostSSE("%s", memberActionURL(routes.GuildRejectMemberPath, slug, p.ID))), Text("Reject")),
							),
						)
					})),
				),
			)
		}),

		// ── Members section
		Div(Class("guild-manage-section panel"),
			P(Class("panel-title"), Text("Members")),
			Table(Class("guild-member-table"),
				THead(Tr(
					Th(Text("Kingdom")),
					Th(Text("Role")),
					Th(Text("Actions")),
				)),
				TBody(Map(members, func(m db.ListGuildMembersWithNamesRow) Node {
					if _guild.MemberRole(m.Role) == _guild.RoleLeader {
						return Tr(
							Td(Text(m.KingdomName)),
							Td(Text("Leader")),
							Td(),
						)
					}
					canRemove := viewerRole.CanRemoveTarget(_guild.MemberRole(m.Role))
					return Tr(
						Td(Text(m.KingdomName)),
						Td(Text(_guild.MemberRole(m.Role).Display())),
						Td(
							If(isLeader && _guild.MemberRole(m.Role) == _guild.RoleMember,
								Button(Class("btn"), ds.On("click", datastar.PostSSE("%s", memberActionURL(routes.GuildPromotePath, slug, m.KingdomID))), Text("Promote")),
							),
							If(isLeader && _guild.MemberRole(m.Role) == _guild.RoleOfficer,
								Button(Class("btn"), ds.On("click", datastar.PostSSE("%s", memberActionURL(routes.GuildDemotePath, slug, m.KingdomID))), Text("Demote")),
							),
							If(canRemove,
								Button(Class("btn btn--danger"), ds.On("click", datastar.PostSSE("%s", memberActionURL(routes.GuildRemoveMemberPath, slug, m.KingdomID))), Text("Remove")),
							),
						),
					)
				})),
			),
		),

		// ── Invite section
		Div(Class("guild-manage-section panel"),
			P(Class("panel-title"), Text("Invite a Kingdom")),
			ds.Signals(map[string]any{"invite_kingdom_name": ""}),
			Div(Class("form-fields"),
				Label(For("guild-invite-input"), Text("Kingdom Name")),
				Input(ID("guild-invite-input"), Type("text"), ds.Bind("invite_kingdom_name"),
					Placeholder("Kingdom name"),
				),
			),
			Button(Class("btn"),
				ds.On("click", datastar.PostSSE("%s", slugURL(routes.GuildInvitePath, slug))),
				Text("Send Invitation"),
			),
		),

		// ── Pending invitations
		Iff(len(invitations) > 0, func() Node {
			return Div(Class("guild-manage-section panel"),
				P(Class("panel-title"), Text("Pending Invitations")),
				Table(Class("guild-member-table"),
					THead(Tr(
						Th(Text("Kingdom")),
						Th(Text("Actions")),
					)),
					TBody(Map(invitations, func(inv db.ListGuildInvitationsRow) Node {
						return Tr(
							Td(Text(inv.KingdomName)),
							Td(
								Button(Class("btn btn--danger"),
									ds.On("click", datastar.PostSSE("%s", memberActionURL(routes.GuildInvitationRevokePath, slug, inv.ID))),
									Text("Revoke"),
								),
							),
						)
					})),
				),
			)
		}),

		// ── Leader-only section
		If(isLeader, guildLeaderActions(g, eligibleMembers)),
		If(isLeader,
			Div(Class("guild-manage-section panel"),
				P(Class("panel-title"), Text("Guild Settings")),
				A(Href(slugURL(routes.GuildSettingsPath, slug)), Class("btn"), Text("Manage Settings")),
			),
		),
	)
}

func guildLeaderActions(g db.Guild, eligibleMembers []db.ListGuildMembersWithNamesRow) Node {
	slug := g.Slug
	editDescURL := slugURL(routes.GuildEditDescriptionPath, slug)
	transferURL := slugURL(routes.GuildTransferLeadershipPath, slug)
	disbandURL := slugURL(routes.GuildDisbandPath, slug)
	return Div(Class("guild-manage-section panel"),
		P(Class("panel-title"), Text("Leader Actions")),

		// Edit description
		ds.Signals(map[string]any{
			"guild_description": g.Description,
		}),
		Div(Class("form-fields"),
			Label(For("guild-desc-edit"), Text("Description")),
			El("textarea", ID("guild-desc-edit"), ds.Bind("guild_description"),
				Attr("rows", "4"), MaxLength("500"),
			),
			Button(Class("btn"),
				ds.On("click", datastar.PostSSE("%s", editDescURL)),
				Text("Save Description"),
			),
		),

		// Transfer leadership
		Iff(len(eligibleMembers) > 0, func() Node {
			return Div(Class("guild-transfer"),
				ds.Signals(map[string]any{
					"target_kingdom_id": eligibleMembers[0].KingdomID,
				}),
				Label(For("guild-transfer-select"), Text("Transfer Leadership to")),
				Select(ID("guild-transfer-select"),
					ds.Bind("target_kingdom_id"),
					Map(eligibleMembers, func(m db.ListGuildMembersWithNamesRow) Node {
						return Option(Value(strconv.Itoa(m.KingdomID)), Text(m.KingdomName))
					}),
				),
				Button(Class("btn"),
					ds.On("click", datastar.PostSSE("%s", transferURL)),
					Text("Transfer Leadership"),
				),
			)
		}),

		// Disband
		Button(Class("btn btn--danger"),
			ds.On("click", datastar.PostSSE("%s", disbandURL)),
			Text("Disband Guild"),
		),
	)
}
