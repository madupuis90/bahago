package guild

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/starfederation/datastar-go/datastar"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	_guild "bahago/internal/guild"
	"bahago/internal/hub"
	. "bahago/internal/ui"
	"bahago/internal/router"
	"bahago/internal/routes"
)

// ── Slug generation ───────────────────────────────────────────────────────────

var (
	reNonSlugChar = regexp.MustCompile(`[^a-z0-9-]+`)
	reMultiHyphen = regexp.MustCompile(`-{2,}`)
)

func generateSlug(name string) string {
	// NFKD decomposition: splits characters + diacritics into base + combining marks,
	// then removes the combining marks (accents).
	t := transform.Chain(norm.NFKD, runes.Remove(runes.In(unicode.Mn)))
	result, _, _ := transform.String(t, name)
	result = strings.ToLower(result)
	result = strings.ReplaceAll(result, "'", "")
	result = reNonSlugChar.ReplaceAllString(result, "-")
	result = reMultiHyphen.ReplaceAllString(result, "-")
	result = strings.Trim(result, "-")
	return result
}

// slugURL substitutes {slug} into a route path constant.
// memberActionURL substitutes both {slug} and {id} into a route path constant.
func slugURL(pattern, slug string) string {
	return strings.ReplaceAll(pattern, "{slug}", slug)
}

func memberActionURL(pattern, slug string, id int) string {
	s := strings.ReplaceAll(pattern, "{slug}", slug)
	return strings.ReplaceAll(s, "{id}", strconv.Itoa(id))
}

// ── Route registration ────────────────────────────────────────────────────────

func RegisterRoutes(r router.Router, queries db.Querier, pool *pgxpool.Pool, tickHub *hub.Hub) {
	h := &handler{queries: queries, pool: pool, hub: tickHub}
	r.HandleFunc("GET "+routes.GuildListPath, h.handleList())
	r.HandleFunc("GET "+routes.GuildPath, h.handleLanding())
	r.HandleFunc("GET "+routes.GuildRefreshPath, h.handleLandingRefresh())
	r.HandleFunc("GET "+routes.GuildNewPath, h.handleNewPage())
	r.HandleFunc("GET "+routes.GuildViewPath, h.handleView())
	r.HandleFunc("GET "+routes.GuildViewRefreshPath, h.handleViewRefresh())
	r.HandleFunc("GET "+routes.GuildManagePath, h.handleManage())
	r.HandleFunc("GET "+routes.GuildManageRefreshPath, h.handleManageRefresh())
	r.HandleFunc("POST "+routes.GuildCreatePath, h.handleCreate())
	r.HandleFunc("POST "+routes.GuildSupportPath, h.handleSupport())
	r.HandleFunc("POST "+routes.GuildWithdrawSupportPath, h.handleWithdrawSupport())
	r.HandleFunc("POST "+routes.GuildCancelProposalPath, h.handleCancelProposal())
	r.HandleFunc("POST "+routes.GuildRequestJoinPath, h.handleRequestJoin())
	r.HandleFunc("POST "+routes.GuildCancelRequestPath, h.handleCancelRequest())
	r.HandleFunc("POST "+routes.GuildApproveMemberPath, h.handleApprove())
	r.HandleFunc("POST "+routes.GuildRejectMemberPath, h.handleReject())
	r.HandleFunc("POST "+routes.GuildRemoveMemberPath, h.handleRemove())
	r.HandleFunc("POST "+routes.GuildPromotePath, h.handlePromote())
	r.HandleFunc("POST "+routes.GuildDemotePath, h.handleDemote())
	r.HandleFunc("POST "+routes.GuildTransferLeadershipPath, h.handleTransferLeadership())
	r.HandleFunc("POST "+routes.GuildLeavePath, h.handleLeave())
	r.HandleFunc("POST "+routes.GuildDisbandPath, h.handleDisband())
	r.HandleFunc("POST "+routes.GuildEditDescriptionPath, h.handleEditDescription())
	r.HandleFunc("POST "+routes.GuildInvitePath, h.handleSendInvitation())
	r.HandleFunc("POST "+routes.GuildInvitationRevokePath, h.handleRevokeInvitation())
	r.HandleFunc("POST "+routes.GuildInvitationAcceptPath, h.handleAcceptInvitation())
	r.HandleFunc("POST "+routes.GuildInvitationDeclinePath, h.handleDeclineInvitation())
	r.HandleFunc("GET "+routes.GuildSettingsPath, h.handleSettings())
	r.HandleFunc("POST "+routes.GuildSettingsSavePath, h.handleSettingsSave())
}

type handler struct {
	queries db.Querier
	pool    *pgxpool.Pool
	hub     *hub.Hub
}

// ── Input structs ─────────────────────────────────────────────────────────────

type createGuildSignals struct {
	GuildName        string `json:"guild_name"`
	GuildDescription string `json:"guild_description"`
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handleList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		guilds, err := h.queries.ListActiveGuilds(r.Context())
		if err != nil {
			log.Printf("guild list: active query: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		pending, err := h.queries.ListPendingGuilds(r.Context())
		if err != nil {
			log.Printf("guild list: pending query: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		KingdomLayout(r, "Guilds", r.URL.Path, kingdom, guildListContent(guilds, pending)).Render(w)
	}
}

func (h *handler) handleLanding() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		membership, err := h.queries.GetKingdomGuildMembership(r.Context(), kingdom.ID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("guild landing: get membership: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err == nil {
			http.Redirect(w, r, slugURL(routes.GuildViewPath, membership.GuildSlug), http.StatusFound)
			return
		}
		invitations, err := h.queries.ListKingdomInvitations(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("guild landing: list invitations: %v", err)
			invitations = nil
		}
		KingdomLayout(r, "Guild", r.URL.Path, kingdom, guildLandingContent(invitations)).Render(w)
	}
}

func (h *handler) handleLandingRefresh() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		ch, cleanup := h.hub.Subscribe(kingdom.ID)
		defer cleanup()

		sse := datastar.NewSSE(w, r)
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ch:
				// If the kingdom has joined a guild since the page loaded, redirect them.
				membership, err := h.queries.GetKingdomGuildMembership(r.Context(), kingdom.ID)
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					log.Printf("guild landing refresh: get membership: %v", err)
					return
				}
				if err == nil {
					if err := sse.Redirect(slugURL(routes.GuildViewPath, membership.GuildSlug)); err != nil {
						log.Printf("guild landing refresh: redirect: %v", err)
					}
					return
				}
				invitations, err := h.queries.ListKingdomInvitations(r.Context(), kingdom.ID)
				if err != nil {
					log.Printf("guild landing refresh: list invitations: %v", err)
					return
				}
				if err := sse.PatchElementGostar(MainContent(guildLandingContent(invitations))); err != nil {
					log.Printf("guild landing refresh: patch: %v", err)
					return
				}
			}
		}
	}
}

func (h *handler) handleNewPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		_, err := h.queries.GetKingdomGuildMembership(r.Context(), kingdom.ID)
		if err == nil {
			http.Redirect(w, r, routes.GuildPath, http.StatusFound)
			return
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			log.Printf("guild new: get membership: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		KingdomLayout(r, "Create Guild", r.URL.Path, kingdom, guildNewContent()).Render(w)
	}
}

func (h *handler) handleCreate() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		input := &createGuildSignals{}
		if err := datastar.ReadSignals(r, input); err != nil {
			log.Printf("guild create: read signals: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(errors.New("invalid request"))))
			return
		}

		sse := datastar.NewSSE(w, r)

		var errs []error
		name := strings.TrimSpace(input.GuildName)
		if len(name) < 5 || len(name) > 60 {
			errs = append(errs, errors.New("guild name must be between 5 and 60 characters"))
		}
		description := strings.TrimSpace(input.GuildDescription)
		if len(description) > 500 {
			errs = append(errs, errors.New("description cannot exceed 500 characters"))
		}
		if len(errs) > 0 {
			sse.PatchElementGostar(guildAlert(AlertError(errs...)))
			return
		}

		slug := generateSlug(name)
		if slug == "" {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("guild name must contain at least one letter or number"))))
			return
		}

		guild, err := h.queries.CreateGuild(r.Context(), db.CreateGuildParams{
			Name:             name,
			Slug:             slug,
			Description:      description,
			FounderKingdomID: kingdom.ID,
		})
		if err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
				var msg string
				switch pgErr.ConstraintName {
				case "guilds_name_unique", "guilds_slug_unique":
					msg = "a guild with this name already exists"
				default:
					msg = "you are already committed to a guild"
				}
				sse.PatchElementGostar(guildAlert(AlertError(errors.New(msg))))
				return
			}
			log.Printf("guild create: insert: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		if err := sse.Redirect(slugURL(routes.GuildViewPath, guild.Slug)); err != nil {
			log.Printf("guild create: redirect: %v", err)
		}
	}
}

func (h *handler) handleView() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")

		guild, members, viewerRole, err := h.loadGuildAndMembership(r, slug, kingdom.ID)
		if errors.Is(err, errGuildNotFound) {
			http.Error(w, "guild not found", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("guild view: load: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		var invitationID int
		if viewerRole.CanReceiveInvitation() {
			if id, err := h.queries.GetKingdomGuildInvitation(r.Context(), db.GetKingdomGuildInvitationParams{
				KingdomID: kingdom.ID,
				GuildID:   guild.ID,
			}); err == nil {
				invitationID = id
			}
		}

		KingdomLayout(r, guild.Name, r.URL.Path, kingdom,
			guildViewContent(guild, members, viewerRole, invitationID),
		).Render(w)
	}
}

func (h *handler) handleViewRefresh() http.HandlerFunc {
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
						log.Printf("guild view refresh: redirect: %v", err)
					}
					return
				}
				if err != nil {
					log.Printf("guild view refresh: load: %v", err)
					return
				}
				var invitationID int
				if viewerRole.CanReceiveInvitation() {
					if id, idErr := h.queries.GetKingdomGuildInvitation(r.Context(), db.GetKingdomGuildInvitationParams{
						KingdomID: k.ID,
						GuildID:   guild.ID,
					}); idErr == nil {
						invitationID = id
					}
				}
				if err := sse.PatchElementGostar(MainContent(guildViewContent(guild, members, viewerRole, invitationID))); err != nil {
					log.Printf("guild view refresh: patch: %v", err)
					return
				}
			}
		}
	}
}

func (h *handler) handleSupport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")
		sse := datastar.NewSSE(w, r)

		tx, err := h.pool.BeginTx(r.Context(), pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			log.Printf("guild support: begin tx: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}
		defer tx.Rollback(r.Context()) //nolint:errcheck

		txq := db.New(tx)

		g, err := txq.GetGuildBySlug(r.Context(), slug)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				sse.PatchElementGostar(guildAlert(AlertError(errors.New("guild not found"))))
				return
			}
			log.Printf("guild support: get guild: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}
		if !_guild.GuildStatus(g.Status).IsPending() {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("this guild is no longer accepting support"))))
			return
		}

		if err := txq.CreateGuildMembership(r.Context(), db.CreateGuildMembershipParams{
			GuildID:   g.ID,
			KingdomID: kingdom.ID,
			Role:      string(_guild.RoleSupporter),
		}); err != nil {
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
				sse.PatchElementGostar(guildAlert(AlertError(errors.New("you are already committed to a guild"))))
				return
			}
			log.Printf("guild support: create membership: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		if err := txq.CancelOtherPendingRequests(r.Context(), db.CancelOtherPendingRequestsParams{
			KingdomID: kingdom.ID,
			GuildID:   g.ID,
		}); err != nil {
			log.Printf("guild support: cancel pending requests: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		count, err := txq.CountGuildSupporters(r.Context(), g.ID)
		if err != nil {
			log.Printf("guild support: count supporters: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		if count >= 5 {
			if err := txq.ActivateGuild(r.Context(), g.ID); err != nil {
				log.Printf("guild support: activate guild: %v", err)
				sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
				return
			}
		}

		if err := tx.Commit(r.Context()); err != nil {
			log.Printf("guild support: commit: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		// Push a live refresh to all supporters so their guild view reflects the new state.
		if supporters, err := h.queries.ListGuildMembersWithNames(r.Context(), g.ID); err == nil {
			supporterIDs := make([]int, 0, len(supporters))
			for _, m := range supporters {
				if m.KingdomID != kingdom.ID {
					supporterIDs = append(supporterIDs, m.KingdomID)
				}
			}
			h.publishUpdates(r, supporterIDs)
		}

		if err := sse.Redirect(slugURL(routes.GuildViewPath, slug)); err != nil {
			log.Printf("guild support: redirect: %v", err)
		}
	}
}

func (h *handler) handleWithdrawSupport() http.HandlerFunc {
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
			log.Printf("guild withdraw support: get guild: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		if err := h.queries.WithdrawSupport(r.Context(), db.WithdrawSupportParams{
			KingdomID: kingdom.ID,
			GuildID:   g.ID,
		}); err != nil {
			log.Printf("guild withdraw support: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		toNotify := []int{kingdom.ID}
		if remaining, err := h.queries.ListGuildMembersWithNames(r.Context(), g.ID); err == nil {
			for _, m := range remaining {
				toNotify = append(toNotify, m.KingdomID)
			}
		}
		h.publishUpdates(r, toNotify)
	}
}

func (h *handler) handleCancelProposal() http.HandlerFunc {
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
			log.Printf("guild cancel proposal: get guild: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		membership, err := h.queries.GetMembershipByKingdomAndGuild(r.Context(), db.GetMembershipByKingdomAndGuildParams{
			KingdomID: kingdom.ID,
			GuildID:   g.ID,
		})
		if err != nil || _guild.MemberRole(membership.Role) != _guild.RoleApplicant {
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("not authorized"))))
			return
		}

		if err := h.queries.CancelProposal(r.Context(), g.ID); err != nil {
			log.Printf("guild cancel proposal: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		if err := sse.Redirect(routes.GuildPath); err != nil {
			log.Printf("guild cancel proposal: redirect: %v", err)
		}
	}
}

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

// ── Helpers ───────────────────────────────────────────────────────────────────

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

var errGuildNotFound = errors.New("guild not found")

func (h *handler) loadGuildAndMembership(r *http.Request, slug string, kingdomID int) (db.Guild, []db.ListGuildMembersWithNamesRow, _guild.MemberRole, error) {
	g, err := h.queries.GetGuildBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Guild{}, nil, _guild.RoleNone, errGuildNotFound
		}
		return db.Guild{}, nil, _guild.RoleNone, fmt.Errorf("get guild: %w", err)
	}

	members, err := h.queries.ListGuildMembersWithNames(r.Context(), g.ID)
	if err != nil {
		return db.Guild{}, nil, _guild.RoleNone, fmt.Errorf("list members: %w", err)
	}

	membership, err := h.queries.GetMembershipByKingdomAndGuild(r.Context(), db.GetMembershipByKingdomAndGuildParams{
		KingdomID: kingdomID,
		GuildID:   g.ID,
	})
	viewerRole := _guild.RoleNone
	if err == nil {
		viewerRole = _guild.MemberRole(membership.Role)
	}

	// Promote to _guild.RoleInOtherGuild when the kingdom has no role in this guild but
	// is committed elsewhere — view functions use this instead of a separate bool.
	if viewerRole == _guild.RoleNone {
		if km, err := h.queries.GetKingdomGuildMembership(r.Context(), kingdomID); err == nil && km.GuildID != g.ID {
			viewerRole = _guild.RoleInOtherGuild
		}
	}

	return g, members, viewerRole, nil
}

func (h *handler) getGuildAndViewerRole(r *http.Request, slug string, kingdomID int) (db.Guild, _guild.MemberRole, error) {
	g, err := h.queries.GetGuildBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Guild{}, _guild.RoleNone, errGuildNotFound
		}
		return db.Guild{}, _guild.RoleNone, fmt.Errorf("get guild: %w", err)
	}
	membership, err := h.queries.GetMembershipByKingdomAndGuild(r.Context(), db.GetMembershipByKingdomAndGuildParams{
		KingdomID: kingdomID,
		GuildID:   g.ID,
	})
	if err != nil {
		return g, _guild.RoleNone, nil
	}
	return g, _guild.MemberRole(membership.Role), nil
}

// publishUpdates fetches the kingdoms for the given IDs and publishes each to the hub,
// triggering live page refreshes for those kingdoms. Errors are logged but not propagated.
func (h *handler) publishUpdates(r *http.Request, kingdomIDs []int) {
	if len(kingdomIDs) == 0 {
		return
	}
	kingdoms, err := h.queries.GetKingdomsByIDs(r.Context(), kingdomIDs)
	if err != nil {
		log.Printf("guild publish: fetch kingdoms: %v", err)
		return
	}
	for _, k := range kingdoms {
		h.hub.Publish(k)
	}
}

// sendNotifications sends an in-game message from one kingdom to one or more recipients
// and immediately publishes each recipient's kingdom to the hub for instant delivery.
// Errors are logged but not propagated — notification failure is non-fatal.
func (h *handler) sendNotifications(r *http.Request, fromKingdomID int, toKingdomIDs []int, subject, body, actionURL, actionText string) {
	if len(toKingdomIDs) == 0 {
		return
	}
	if err := h.queries.BulkCreateMessages(r.Context(), db.BulkCreateMessagesParams{
		FromKingdomID: fromKingdomID,
		ToKingdomIds:  toKingdomIDs,
		Subject:       subject,
		Body:          body,
		ActionUrl:     actionURL,
		ActionText:    actionText,
	}); err != nil {
		log.Printf("guild notification: send message: %v", err)
		return
	}
	h.publishUpdates(r, toKingdomIDs)
}

// ── Page components ───────────────────────────────────────────────────────────

func guildAlert(inner Node) Node { return AlertContainer("guild-alert", inner) }

func guildLandingContent(invitations []db.ListKingdomInvitationsRow) Node {
	return Div(
		H1(Class("page-title"), Text("Guild")),
		Div(ds.Init(GetSSENoSignals(routes.GuildRefreshPath))),
		Iff(len(invitations) > 0, func() Node {
			return Div(Class("guild-invitations panel"),
				P(Class("panel-title"), Text("Your Invitations")),
				Table(Class("guild-member-table"),
					THead(Tr(
						Th(Text("Guild")),
						Th(Text("Actions")),
					)),
					TBody(Map(invitations, func(inv db.ListKingdomInvitationsRow) Node {
						return Tr(
							Td(A(Href(slugURL(routes.GuildViewPath, inv.GuildSlug)), Text(inv.GuildName))),
							Td(
								Button(Class("btn"),
									ds.On("click", datastar.PostSSE("%s", memberActionURL(routes.GuildInvitationAcceptPath, inv.GuildSlug, inv.ID))),
									Text("Accept"),
								),
								Button(Class("btn btn--danger"),
									ds.On("click", datastar.PostSSE("%s", memberActionURL(routes.GuildInvitationDeclinePath, inv.GuildSlug, inv.ID))),
									Text("Decline"),
								),
							),
						)
					})),
				),
			)
		}),
		Div(Class("guild-landing"),
			Div(Class("panel"),
				P(Class("panel-title"), Text("Looking for a guild?")),
				P(Text("Browse active guilds and find one to join.")),
				A(Href(routes.GuildListPath), Class("btn"), Text("Browse Guilds")),
			),
			Div(Class("panel"),
				P(Class("panel-title"), Text("Start your own")),
				P(Text("Submit an application and have 4 other kingdoms support it to bring your guild into being.")),
				A(Href(routes.GuildNewPath), Class("btn"), Text("Guild Application")),
			),
		),
		guildAlert(nil),
	)
}

func guildNewContent() Node {
	return Div(
		H1(Class("page-title"), Text("Guild Application")),
		Div(Class("guild-new panel"),
			Div(Class("form-fields"),
				Label(For("guild-name-input"), Text("Guild Name")),
				Input(ID("guild-name-input"), Type("text"), ds.Bind("guild_name"),
					Placeholder("e.g. La table ronde"),
					MinLength("5"), MaxLength("60"),
				),
				Label(For("guild-desc-input"), Text("Description")),
				El("textarea", ID("guild-desc-input"), ds.Bind("guild_description"),
					Placeholder("Describe your guild..."),
					Attr("rows", "4"), MaxLength("500"),
				),
			),
			Button(Class("btn"),
				ds.On("click", datastar.PostSSE(routes.GuildCreatePath)),
				Text("Submit Application"),
			),
			P(Class("guild-application-note"), Text("4 other kingdoms must support the application before the guild is officially founded.")),
			guildAlert(nil),
		),
	)
}

func guildViewContent(g db.Guild, members []db.ListGuildMembersWithNamesRow, viewerRole _guild.MemberRole, invitationID int) Node {
	isPending := _guild.GuildStatus(g.Status).IsPending()

	titleSuffix := ""
	if isPending {
		titleSuffix = " (Application)"
	}

	supportCount := 0
	activeCount := 0
	for _, m := range members {
		if _guild.MemberRole(m.Role).IsApplicationPhase() {
			supportCount++
		}
		if _guild.MemberRole(m.Role).IsActiveMember() {
			activeCount++
		}
	}

	return Div(
		H1(Class("page-title"), Text(g.Name+titleSuffix)),
		Div(ds.Init(GetSSENoSignals("%s", slugURL(routes.GuildViewRefreshPath, g.Slug)))),
		Div(Class("guild-view panel"),
			If(g.Description != "", P(Class("guild-description"), Text(g.Description))),
			If(isPending,
				P(Class("guild-support-progress"), Text(fmt.Sprintf("%d/5 kingdoms have supported the application", supportCount))),
			),
			If(!isPending,
				P(Class("guild-member-count"), Text(fmt.Sprintf("%d/20 members", activeCount))),
			),
			guildMemberTable(members),
			guildActionButtons(g, viewerRole, invitationID),
		),
		guildAlert(nil),
	)
}

func guildMemberTable(members []db.ListGuildMembersWithNamesRow) Node {
	if len(members) == 0 {
		return nil
	}
	return Table(Class("guild-member-table"),
		THead(Tr(
			Th(Text("Kingdom")),
			Th(Text("Role")),
		)),
		TBody(Map(members, func(m db.ListGuildMembersWithNamesRow) Node {
			return Tr(
				Td(Text(m.KingdomName)),
				Td(Text(_guild.MemberRole(m.Role).Display())),
			)
		})),
	)
}

func guildActionButtons(g db.Guild, viewerRole _guild.MemberRole, invitationID int) Node {
	slug := g.Slug
	guildStatus := _guild.GuildStatus(g.Status)
	isPending := guildStatus.IsPending()
	isActive := guildStatus.IsActive()

	return Div(Class("guild-actions"),
		// Application-phase actions
		If(isPending && viewerRole == _guild.RoleNone,
			Button(Class("btn"),
				ds.On("click", datastar.PostSSE("%s", slugURL(routes.GuildSupportPath, slug))),
				Text("Support Application"),
			),
		),
		If(isPending && viewerRole == _guild.RoleSupporter,
			Button(Class("btn btn--danger"),
				ds.On("click", datastar.PostSSE("%s", slugURL(routes.GuildWithdrawSupportPath, slug))),
				Text("Withdraw Support"),
			),
		),
		If(isPending && viewerRole == _guild.RoleApplicant,
			Button(Class("btn btn--danger"),
				ds.On("click", datastar.PostSSE("%s", slugURL(routes.GuildCancelProposalPath, slug))),
				Text("Withdraw Application"),
			),
		),

		// Active guild actions for non-members
		If(isActive && viewerRole == _guild.RoleNone && invitationID == 0,
			Button(Class("btn"),
				ds.On("click", datastar.PostSSE("%s", slugURL(routes.GuildRequestJoinPath, slug))),
				Text("Request to Join"),
			),
		),
		If(isActive && invitationID != 0,
			Div(Class("guild-invitation-notice"),
				P(Text("You have been invited to join this guild.")),
				Button(Class("btn"),
					ds.On("click", datastar.PostSSE("%s", memberActionURL(routes.GuildInvitationAcceptPath, slug, invitationID))),
					Text("Accept Invitation"),
				),
				Button(Class("btn btn--danger"),
					ds.On("click", datastar.PostSSE("%s", memberActionURL(routes.GuildInvitationDeclinePath, slug, invitationID))),
					Text("Decline"),
				),
			),
		),
		If(isActive && viewerRole == _guild.RolePendingApproval,
			Button(Class("btn"),
				ds.On("click", datastar.PostSSE("%s", slugURL(routes.GuildCancelRequestPath, slug))),
				Text("Cancel Request"),
			),
		),

		// Active guild: member/officer can leave
		If(isActive && viewerRole.CanLeave(),
			Button(Class("btn btn--danger"),
				ds.On("click", datastar.PostSSE("%s", slugURL(routes.GuildLeavePath, slug))),
				Text("Leave Guild"),
			),
		),

		// Manage button for leader/officer
		If(isActive && viewerRole.CanManage(),
			A(Href(slugURL(routes.GuildManagePath, slug)), Class("btn"), Text("Manage Guild")),
		),
	)
}

func guildListContent(guilds []db.ListActiveGuildsRow, pending []db.ListPendingGuildsRow) Node {
	return Div(
		H1(Class("page-title"), Text("Guilds")),
		Div(Class("guilds-list panel"),
			P(Class("panel-title"), Text("Active Guilds")),
			Iff(len(guilds) == 0, func() Node {
				return P(Text("No active guilds yet."))
			}),
			Iff(len(guilds) > 0, func() Node {
				return Table(Class("table"),
					THead(
						Tr(
							Th(Text("Guild")),
							Th(Text("Leader")),
							Th(Text("Members")),
						),
					),
					TBody(
						Map(guilds, func(g db.ListActiveGuildsRow) Node {
							leaderName := "—"
							if g.LeaderName.Valid {
								leaderName = g.LeaderName.String
							}
							return Tr(
								Td(A(Href(slugURL(routes.GuildViewPath, g.Slug)), Text(g.Name))),
								Td(Text(leaderName)),
								Td(Text(fmt.Sprintf("%d / 20", g.MemberCount))),
							)
						}),
					),
				)
			}),
		),
		Iff(len(pending) > 0, func() Node {
			return Div(Class("guilds-applications panel"),
				P(Class("panel-title"), Text("Guild Applications")),
				Table(Class("table"),
					THead(
						Tr(
							Th(Text("Guild")),
							Th(Text("Founder")),
							Th(Text("Supporters")),
							Th(Text("Expires")),
						),
					),
					TBody(
						Map(pending, func(g db.ListPendingGuildsRow) Node {
							founderName := "—"
							if g.FounderName.Valid {
								founderName = g.FounderName.String
							}
							return Tr(
								Td(A(Href(slugURL(routes.GuildViewPath, g.Slug)), Text(g.Name))),
								Td(Text(founderName)),
								Td(Text(fmt.Sprintf("%d / 5", g.SupporterCount))),
								Td(Text(formatExpiry(g.ExpiresAt))),
							)
						}),
					),
				),
			)
		}),
	)
}

func formatExpiry(t time.Time) string {
	d := int(time.Until(t).Hours() / 24)
	switch {
	case d <= 0:
		return "today"
	case d == 1:
		return "1 day"
	default:
		return fmt.Sprintf("%d days", d)
	}
}
