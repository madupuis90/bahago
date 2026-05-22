package guild

import (
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

func (h *handler) handleRequestJoin() http.HandlerFunc {
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
			log.Printf("guild request join: get guild: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}
		if !_guild.GuildStatus(g.Status).IsActive() {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("this guild is not accepting requests"))))
			return
		}

		// Server-side guard: reject if the kingdom is already committed to any guild.
		if _, kmErr := h.queries.GetKingdomGuildMembership(r.Context(), kingdom.ID); kmErr == nil {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("you are already committed to a guild"))))
			return
		} else if !errors.Is(kmErr, pgx.ErrNoRows) {
			log.Printf("guild request join: check membership: %v", kmErr)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		_, err = h.queries.RequestJoinIfNotFull(r.Context(), db.RequestJoinIfNotFullParams{
			GuildID:   g.ID,
			KingdomID: kingdom.ID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				sse.PatchElementGostar(guildAlert(AlertError(errors.New("this guild is full"))))
				return
			}
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
				sse.PatchElementGostar(guildAlert(AlertError(errors.New("you already have a pending request to this guild"))))
				return
			}
			log.Printf("guild request join: create membership: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		// Notify guild members that a new join request has arrived.
		if members, err := h.queries.ListGuildMembersWithNames(r.Context(), g.ID); err == nil {
			managerIDs := make([]int, 0)
			for _, m := range members {
				if _guild.MemberRole(m.Role).CanManage() {
					managerIDs = append(managerIDs, m.KingdomID)
				}
			}
			h.sendNotifications(r, kingdom.ID, managerIDs,
				"New Join Request",
				kingdom.Name+" has requested to join "+g.Name+".",
				slugURL(routes.GuildManagePath, slug), "Manage Guild",
			)
		}

		h.publishUpdates(r, []int{kingdom.ID})
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
		h.publishUpdates(r, toNotify)
	}
}

func (h *handler) handleApprove() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")
		sse := datastar.NewSSE(w, r)

		membershipID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("invalid membership id"))))
			return
		}

		g, viewerRole, err := h.getGuildAndViewerRole(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("guild not found"))))
			return
		}
		if err != nil {
			log.Printf("guild approve: get guild: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}
		if !viewerRole.CanManage() {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("not authorized"))))
			return
		}

		approvedKingdomID, err := h.queries.ApproveMembershipIfNotFull(r.Context(), db.ApproveMembershipIfNotFullParams{
			MembershipID: membershipID,
			GuildID:      g.ID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				sse.PatchElementGostar(guildAlert(AlertError(errors.New("guild is full or request no longer exists"))))
				return
			}
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
				sse.PatchElementGostar(guildAlert(AlertError(errors.New("this kingdom is already committed to another guild"))))
				return
			}
			log.Printf("guild approve: approve membership: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
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
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("invalid membership id"))))
			return
		}

		g, viewerRole, err := h.getGuildAndViewerRole(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("guild not found"))))
			return
		}
		if err != nil {
			log.Printf("guild reject: get guild: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}
		if !viewerRole.CanManage() {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("not authorized"))))
			return
		}

		membership, err := h.queries.GetMembershipByID(r.Context(), db.GetMembershipByIDParams{
			ID:      membershipID,
			GuildID: g.ID,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				sse.PatchElementGostar(guildAlert(AlertError(errors.New("request not found"))))
				return
			}
			log.Printf("guild reject: get membership: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		if err := h.queries.RejectMembership(r.Context(), db.RejectMembershipParams{
			MembershipID: membershipID,
			GuildID:      g.ID,
		}); err != nil {
			log.Printf("guild reject: reject membership: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
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
