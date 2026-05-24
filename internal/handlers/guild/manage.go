package guild

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	_guild "bahago/internal/guild"
	. "bahago/internal/ui"
	"bahago/internal/routes"
)

// ── Input structs ─────────────────────────────────────────────────────────────

type editDescriptionSignals struct {
	GuildDescription string `json:"guild_description"`
}

type transferLeaderSignals struct {
	TargetKingdomID int `json:"target_kingdom_id"`
}

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrInvalidKingdomID             = errors.New("invalid kingdom id")
	ErrCannotRemoveTarget           = errors.New("officers can only remove regular members")
	ErrLeaderMustTransferFirst      = errors.New("the guild leader must transfer leadership before leaving")
	ErrCannotLeave                  = errors.New("permission denied")
	ErrOnlyLeaderCanPromote         = errors.New("only the guild leader can promote officers")
	ErrOfficerCapReached            = errors.New("a guild can have at most 4 officers")
	ErrCannotDemoteOfficer          = errors.New("officers cannot demote other officers")
	ErrInvalidTransferTarget        = errors.New("please select a valid member to transfer leadership to")
	ErrOnlyLeaderCanTransfer        = errors.New("only the guild leader can transfer leadership")
	ErrTargetNotMember              = errors.New("target is not a member of this guild")
	ErrTargetCannotBeLeader         = errors.New("leadership can only be transferred to a full member or officer")
	ErrOnlyLeaderCanDisband         = errors.New("only the guild leader can disband the guild")
	ErrOnlyLeaderCanEditDescription = errors.New("only the guild leader can edit the description")
)

// ── Validation ────────────────────────────────────────────────────────────────

func validateKingdomID(s string) (int, error) {
	id, err := strconv.Atoi(s)
	if err != nil || id <= 0 {
		return 0, ErrInvalidKingdomID
	}
	return id, nil
}

func validateTransferInput(in *transferLeaderSignals, actorKingdomID int) []error {
	var errs []error
	if in.TargetKingdomID == 0 || in.TargetKingdomID == actorKingdomID {
		errs = append(errs, ErrInvalidTransferTarget)
	}
	return errs
}

func validateEditDescription(in *editDescriptionSignals) []error {
	var errs []error
	if len(strings.TrimSpace(in.GuildDescription)) > 500 {
		errs = append(errs, ErrDescriptionTooLong)
	}
	return errs
}

// ── Page handlers ─────────────────────────────────────────────────────────────

func (h *handler) handleManage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")

		guild, members, viewerRole, err := h.loadGuildAndMembership(r.Context(), slug, kingdom.ID)
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
				guild, members, viewerRole, err := h.loadGuildAndMembership(r.Context(), slug, k.ID)
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

// ── Action handlers ───────────────────────────────────────────────────────────

func (h *handler) handleRemove() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")

		targetKingdomID, err := validateKingdomID(r.PathValue("id"))
		if err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(err)))
			return
		}

		guildName, err := h.removeMember(r.Context(), kingdom.ID, slug, targetKingdomID)
		if err != nil {
			if isRemoveUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(err)))
				return
			}
			log.Printf("guild remove: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		h.sendNotifications(r.Context(), kingdom.ID, []int{targetKingdomID},
			"Removed from "+guildName,
			"You have been removed from "+guildName+".",
			"", "",
		)

		h.publishUpdates(r.Context(), []int{kingdom.ID})
	}
}

func (h *handler) handleLeave() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")
		sse := datastar.NewSSE(w, r)

		guildName, managerIDs, err := h.leaveGuild(r.Context(), kingdom.ID, slug)
		if err != nil {
			if isLeaveUserError(err) {
				sse.PatchElementGostar(guildAlert(AlertError(err)))
				return
			}
			log.Printf("guild leave: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		h.sendNotifications(r.Context(), kingdom.ID, managerIDs,
			"Member Left",
			kingdom.Name+" has left "+guildName+".",
			slugURL(routes.GuildViewPath, slug), "Visit Guild",
		)

		if err := sse.Redirect(routes.GuildPath); err != nil {
			log.Printf("guild leave: redirect: %v", err)
		}
	}
}

func (h *handler) handlePromote() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")

		targetKingdomID, err := validateKingdomID(r.PathValue("id"))
		if err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(err)))
			return
		}

		guildName, err := h.promoteToOfficer(r.Context(), kingdom.ID, slug, targetKingdomID)
		if err != nil {
			if isPromoteUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(err)))
				return
			}
			log.Printf("guild promote: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		h.sendNotifications(r.Context(), kingdom.ID, []int{targetKingdomID},
			"Promoted to Officer",
			"You have been promoted to Officer in "+guildName+".",
			slugURL(routes.GuildViewPath, slug), "Visit Guild",
		)

		h.publishUpdates(r.Context(), []int{kingdom.ID})
	}
}

func (h *handler) handleDemote() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")

		targetKingdomID, err := validateKingdomID(r.PathValue("id"))
		if err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(err)))
			return
		}

		guildName, err := h.demoteFromOfficer(r.Context(), kingdom.ID, slug, targetKingdomID)
		if err != nil {
			if isDemoteUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(err)))
				return
			}
			log.Printf("guild demote: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		h.sendNotifications(r.Context(), kingdom.ID, []int{targetKingdomID},
			"Demoted to Member",
			"You have been demoted to Member in "+guildName+".",
			slugURL(routes.GuildViewPath, slug), "Visit Guild",
		)

		h.publishUpdates(r.Context(), []int{kingdom.ID})
	}
}

func (h *handler) handleTransferLeadership() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")

		input := &transferLeaderSignals{}
		if err := datastar.ReadSignals(r, input); err != nil {
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(errors.New("invalid request"))))
			return
		}

		if errs := validateTransferInput(input, kingdom.ID); len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(errs...)))
			return
		}

		sse := datastar.NewSSE(w, r)

		guildName, err := h.transferLeadership(r.Context(), kingdom.ID, slug, input.TargetKingdomID)
		if err != nil {
			if isTransferLeadershipUserError(err) {
				sse.PatchElementGostar(guildAlert(AlertError(err)))
				return
			}
			log.Printf("guild transfer leadership: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		h.sendNotifications(r.Context(), kingdom.ID, []int{input.TargetKingdomID},
			"Guild Leadership Transferred",
			"You are now the leader of "+guildName+".",
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

		guildName, memberIDs, err := h.disbandGuild(r.Context(), kingdom.ID, slug)
		if err != nil {
			if isDisbandUserError(err) {
				sse.PatchElementGostar(guildAlert(AlertError(err)))
				return
			}
			log.Printf("guild disband: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		h.sendNotifications(r.Context(), kingdom.ID, memberIDs,
			"Guild Disbanded",
			guildName+" has been disbanded.",
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
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(errors.New("invalid request"))))
			return
		}

		if errs := validateEditDescription(input); len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(errs...)))
			return
		}

		sse := datastar.NewSSE(w, r)

		if err := h.updateGuildDescription(r.Context(), kingdom.ID, slug, input); err != nil {
			if isEditDescriptionUserError(err) {
				sse.PatchElementGostar(guildAlert(AlertError(err)))
				return
			}
			log.Printf("guild edit description: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		h.publishUpdates(r.Context(), []int{kingdom.ID})
	}
}

// ── Orchestration ─────────────────────────────────────────────────────────────

// removeMember removes the target kingdom's membership from the guild after
// verifying the actor has the right to remove a member of that role. Returns
// the guild name so the caller can format the notification message.
func (h *handler) removeMember(ctx context.Context, actorKingdomID int, slug string, targetKingdomID int) (string, error) {
	g, viewerRole, err := h.getGuildAndViewerRole(ctx, slug, actorKingdomID)
	if errors.Is(err, ErrGuildNotFound) {
		return "", ErrGuildNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get guild: %w", err)
	}
	if !viewerRole.CanManage() {
		return "", ErrNotAuthorized
	}

	target, err := h.queries.GetMembershipByKingdomAndGuild(ctx, db.GetMembershipByKingdomAndGuildParams{
		KingdomID: targetKingdomID,
		GuildID:   g.ID,
	})
	if err != nil {
		return "", fmt.Errorf("check target role: %w", err)
	}
	if !viewerRole.CanRemoveTarget(_guild.MemberRole(target.Role)) {
		return "", ErrCannotRemoveTarget
	}

	if err := h.queries.RemoveMembership(ctx, db.RemoveMembershipParams{
		KingdomID: targetKingdomID,
		GuildID:   g.ID,
	}); err != nil {
		return "", fmt.Errorf("remove membership: %w", err)
	}
	return g.Name, nil
}

// leaveGuild removes the actor's own membership. Leaders must transfer first.
// Returns the guild name and the list of manager IDs so the caller can fan out
// notifications.
func (h *handler) leaveGuild(ctx context.Context, kingdomID int, slug string) (string, []int, error) {
	g, viewerRole, err := h.getGuildAndViewerRole(ctx, slug, kingdomID)
	if errors.Is(err, ErrGuildNotFound) {
		return "", nil, ErrGuildNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("get guild: %w", err)
	}
	if viewerRole == _guild.RoleLeader {
		return "", nil, ErrLeaderMustTransferFirst
	}
	if viewerRole != _guild.RoleMember && viewerRole != _guild.RoleOfficer {
		return "", nil, ErrCannotLeave
	}

	members, err := h.queries.ListGuildMembersWithNames(ctx, g.ID)
	if err != nil {
		return "", nil, fmt.Errorf("list members: %w", err)
	}
	managerIDs := make([]int, 0)
	for _, m := range members {
		if m.KingdomID != kingdomID && _guild.MemberRole(m.Role).CanManage() {
			managerIDs = append(managerIDs, m.KingdomID)
		}
	}

	if err := h.queries.RemoveMembership(ctx, db.RemoveMembershipParams{
		KingdomID: kingdomID,
		GuildID:   g.ID,
	}); err != nil {
		return "", nil, fmt.Errorf("remove membership: %w", err)
	}
	return g.Name, managerIDs, nil
}

// promoteToOfficer promotes a member to officer. Leader-only, with a 4-officer
// cap enforced by counting current officers in the membership list.
func (h *handler) promoteToOfficer(ctx context.Context, actorKingdomID int, slug string, targetKingdomID int) (string, error) {
	g, members, viewerRole, err := h.loadGuildAndMembership(ctx, slug, actorKingdomID)
	if errors.Is(err, ErrGuildNotFound) {
		return "", ErrGuildNotFound
	}
	if err != nil {
		return "", fmt.Errorf("load guild: %w", err)
	}
	if !viewerRole.IsLeader() {
		return "", ErrOnlyLeaderCanPromote
	}

	officerCount := 0
	for _, m := range members {
		if _guild.MemberRole(m.Role) == _guild.RoleOfficer {
			officerCount++
		}
	}
	if officerCount >= 4 {
		return "", ErrOfficerCapReached
	}

	if err := h.queries.SetMembershipRole(ctx, db.SetMembershipRoleParams{
		Role:      string(_guild.RoleOfficer),
		KingdomID: targetKingdomID,
		GuildID:   g.ID,
	}); err != nil {
		return "", fmt.Errorf("set role: %w", err)
	}
	return g.Name, nil
}

// demoteFromOfficer demotes a member to the regular member role. Officers can
// demote regular members; only the leader can demote other officers (enforced
// by CanRemoveTarget).
func (h *handler) demoteFromOfficer(ctx context.Context, actorKingdomID int, slug string, targetKingdomID int) (string, error) {
	g, viewerRole, err := h.getGuildAndViewerRole(ctx, slug, actorKingdomID)
	if errors.Is(err, ErrGuildNotFound) {
		return "", ErrGuildNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get guild: %w", err)
	}
	if !viewerRole.CanManage() {
		return "", ErrNotAuthorized
	}

	target, err := h.queries.GetMembershipByKingdomAndGuild(ctx, db.GetMembershipByKingdomAndGuildParams{
		KingdomID: targetKingdomID,
		GuildID:   g.ID,
	})
	if err != nil {
		return "", fmt.Errorf("check target role: %w", err)
	}
	if !viewerRole.CanRemoveTarget(_guild.MemberRole(target.Role)) {
		return "", ErrCannotDemoteOfficer
	}

	if err := h.queries.SetMembershipRole(ctx, db.SetMembershipRoleParams{
		Role:      string(_guild.RoleMember),
		KingdomID: targetKingdomID,
		GuildID:   g.ID,
	}); err != nil {
		return "", fmt.Errorf("set role: %w", err)
	}
	return g.Name, nil
}

// transferLeadership hands leadership of the guild to a target member or
// officer. Leader-only.
func (h *handler) transferLeadership(ctx context.Context, actorKingdomID int, slug string, targetKingdomID int) (string, error) {
	g, viewerRole, err := h.getGuildAndViewerRole(ctx, slug, actorKingdomID)
	if errors.Is(err, ErrGuildNotFound) {
		return "", ErrGuildNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get guild: %w", err)
	}
	if !viewerRole.IsLeader() {
		return "", ErrOnlyLeaderCanTransfer
	}

	target, err := h.queries.GetMembershipByKingdomAndGuild(ctx, db.GetMembershipByKingdomAndGuildParams{
		KingdomID: targetKingdomID,
		GuildID:   g.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrTargetNotMember
		}
		return "", fmt.Errorf("get target membership: %w", err)
	}
	if role := _guild.MemberRole(target.Role); role != _guild.RoleMember && role != _guild.RoleOfficer {
		return "", ErrTargetCannotBeLeader
	}

	if err := h.queries.TransferLeadership(ctx, db.TransferLeadershipParams{
		NewLeaderKingdomID: targetKingdomID,
		GuildID:            g.ID,
	}); err != nil {
		return "", fmt.Errorf("transfer leadership: %w", err)
	}
	return g.Name, nil
}

// disbandGuild dissolves the guild after collecting the IDs of active members
// (other than the leader) so the caller can notify them.
func (h *handler) disbandGuild(ctx context.Context, actorKingdomID int, slug string) (string, []int, error) {
	g, viewerRole, err := h.getGuildAndViewerRole(ctx, slug, actorKingdomID)
	if errors.Is(err, ErrGuildNotFound) {
		return "", nil, ErrGuildNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("get guild: %w", err)
	}
	if !viewerRole.IsLeader() {
		return "", nil, ErrOnlyLeaderCanDisband
	}

	members, err := h.queries.ListGuildMembersWithNames(ctx, g.ID)
	if err != nil {
		return "", nil, fmt.Errorf("list members: %w", err)
	}
	memberIDs := make([]int, 0, len(members))
	for _, m := range members {
		if m.KingdomID != actorKingdomID && _guild.MemberRole(m.Role).IsActiveMember() {
			memberIDs = append(memberIDs, m.KingdomID)
		}
	}

	if err := h.queries.DisbandGuild(ctx, g.ID); err != nil {
		return "", nil, fmt.Errorf("disband: %w", err)
	}
	return g.Name, memberIDs, nil
}

// updateGuildDescription saves a new description for the guild. Leader-only.
func (h *handler) updateGuildDescription(ctx context.Context, actorKingdomID int, slug string, input *editDescriptionSignals) error {
	g, viewerRole, err := h.getGuildAndViewerRole(ctx, slug, actorKingdomID)
	if errors.Is(err, ErrGuildNotFound) {
		return ErrGuildNotFound
	}
	if err != nil {
		return fmt.Errorf("get guild: %w", err)
	}
	if !viewerRole.IsLeader() {
		return ErrOnlyLeaderCanEditDescription
	}

	description := strings.TrimSpace(input.GuildDescription)
	if err := h.queries.UpdateGuildDescription(ctx, db.UpdateGuildDescriptionParams{
		Description: description,
		ID:          g.ID,
	}); err != nil {
		return fmt.Errorf("update description: %w", err)
	}
	return nil
}

// ── User-error predicates ─────────────────────────────────────────────────────

func isRemoveUserError(err error) bool {
	return errors.Is(err, ErrGuildNotFound) ||
		errors.Is(err, ErrNotAuthorized) ||
		errors.Is(err, ErrCannotRemoveTarget)
}

func isLeaveUserError(err error) bool {
	return errors.Is(err, ErrGuildNotFound) ||
		errors.Is(err, ErrLeaderMustTransferFirst) ||
		errors.Is(err, ErrCannotLeave)
}

func isPromoteUserError(err error) bool {
	return errors.Is(err, ErrGuildNotFound) ||
		errors.Is(err, ErrOnlyLeaderCanPromote) ||
		errors.Is(err, ErrOfficerCapReached)
}

func isDemoteUserError(err error) bool {
	return errors.Is(err, ErrGuildNotFound) ||
		errors.Is(err, ErrNotAuthorized) ||
		errors.Is(err, ErrCannotDemoteOfficer)
}

func isTransferLeadershipUserError(err error) bool {
	return errors.Is(err, ErrGuildNotFound) ||
		errors.Is(err, ErrOnlyLeaderCanTransfer) ||
		errors.Is(err, ErrTargetNotMember) ||
		errors.Is(err, ErrTargetCannotBeLeader)
}

func isDisbandUserError(err error) bool {
	return errors.Is(err, ErrGuildNotFound) ||
		errors.Is(err, ErrOnlyLeaderCanDisband)
}

func isEditDescriptionUserError(err error) bool {
	return errors.Is(err, ErrGuildNotFound) ||
		errors.Is(err, ErrOnlyLeaderCanEditDescription)
}

// ── Page components ───────────────────────────────────────────────────────────

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
		guildAlert(nil),
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
