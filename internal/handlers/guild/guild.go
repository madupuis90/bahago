package guild

import (
	"context"
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
	"bahago/internal/router"
	"bahago/internal/routes"
	. "bahago/internal/ui"
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

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	// Validation sentinels.
	ErrGuildNameLength    = errors.New("guild name must be between 5 and 60 characters")
	ErrDescriptionTooLong = errors.New("description cannot exceed 500 characters")
	ErrGuildNameInvalid   = errors.New("guild name must contain at least one letter or number")

	// Orchestrator sentinels.
	ErrGuildNotFound   = errors.New("guild not found")
	ErrGuildNameTaken  = errors.New("a guild with this name already exists")
	ErrAlreadyInGuild  = errors.New("you are already committed to a guild")
	ErrGuildNotPending = errors.New("this guild is no longer accepting support")
	ErrNotAuthorized   = errors.New("not authorized")
	ErrInvalidID       = errors.New("invalid id")
)

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handleList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		activeGuilds, err := h.queries.ListActiveGuilds(r.Context())
		if err != nil {
			log.Printf("guild list: active query: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		pendingGuilds, err := h.queries.ListPendingGuilds(r.Context())
		if err != nil {
			log.Printf("guild list: pending query: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		KingdomLayout(r, "Guilds", r.URL.Path, kingdom, guildListContent(activeGuilds, pendingGuilds)).Render(w)
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

		if errs := validateCreateGuildInput(input); len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(errs...)))
			return
		}

		slug, err := h.createGuild(r.Context(), kingdom.ID, input)
		if err != nil {
			if isCreateGuildUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(err)))
				return
			}
			log.Printf("guild create: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		sse := datastar.NewSSE(w, r)
		if err := sse.Redirect(slugURL(routes.GuildViewPath, slug)); err != nil {
			log.Printf("guild create: redirect: %v", err)
		}
	}
}

// validateCreateGuildInput runs the field-level rules. Slug emptiness check
// lives in the orchestrator because it depends on generateSlug.
func validateCreateGuildInput(in *createGuildSignals) []error {
	var errs []error
	name := strings.TrimSpace(in.GuildName)
	if len(name) < 5 || len(name) > 60 {
		errs = append(errs, ErrGuildNameLength)
	}
	if len(strings.TrimSpace(in.GuildDescription)) > 500 {
		errs = append(errs, ErrDescriptionTooLong)
	}
	return errs
}

// createGuild generates the slug, inserts the guild row, and translates a
// unique-violation into one of two sentinels depending on which constraint
// fired (name/slug vs. founder-already-in-a-guild). Returns the slug for the
// caller's redirect.
func (h *handler) createGuild(ctx context.Context, founderKingdomID int, input *createGuildSignals) (string, error) {
	name := strings.TrimSpace(input.GuildName)
	description := strings.TrimSpace(input.GuildDescription)

	slug := generateSlug(name)
	if slug == "" {
		return "", ErrGuildNameInvalid
	}

	if _, err := h.queries.CreateGuild(ctx, db.CreateGuildParams{
		Name:             name,
		Slug:             slug,
		Description:      description,
		FounderKingdomID: founderKingdomID,
	}); err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
			switch pgErr.ConstraintName {
			case "guilds_name_unique", "guilds_slug_unique":
				return "", ErrGuildNameTaken
			default:
				return "", ErrAlreadyInGuild
			}
		}
		return "", fmt.Errorf("create guild: %w", err)
	}
	return slug, nil
}

func isCreateGuildUserError(err error) bool {
	return errors.Is(err, ErrGuildNameInvalid) ||
		errors.Is(err, ErrGuildNameTaken) ||
		errors.Is(err, ErrAlreadyInGuild)
}

func (h *handler) handleView() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)
		slug := r.PathValue("slug")

		guild, members, viewerRole, err := h.loadGuildAndMembership(r.Context(), slug, kingdom.ID)
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
				guild, members, viewerRole, err := h.loadGuildAndMembership(r.Context(), slug, k.ID)
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

		guildID, err := h.supportGuild(r.Context(), kingdom.ID, slug)
		if err != nil {
			if isSupportUserError(err) {
				sse.PatchElementGostar(guildAlert(AlertError(err)))
				return
			}
			log.Printf("guild support: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}

		// Push a live refresh to all supporters so their guild view reflects the new state.
		if supporters, err := h.queries.ListGuildMembersWithNames(r.Context(), guildID); err == nil {
			supporterIDs := make([]int, 0, len(supporters))
			for _, m := range supporters {
				if m.KingdomID != kingdom.ID {
					supporterIDs = append(supporterIDs, m.KingdomID)
				}
			}
			h.publishUpdates(r.Context(), supporterIDs)
		}

		if err := sse.Redirect(slugURL(routes.GuildViewPath, slug)); err != nil {
			log.Printf("guild support: redirect: %v", err)
		}
	}
}

// supportGuild adds the kingdom as a supporter of the pending guild inside a
// SERIALIZABLE transaction so the supporter-count read and the activation
// decision share the same snapshot. Returns the guild ID on success.
func (h *handler) supportGuild(ctx context.Context, kingdomID int, slug string) (int, error) {
	tx, err := h.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	txq := db.New(tx)

	g, err := txq.GetGuildBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrGuildNotFound
		}
		return 0, fmt.Errorf("get guild: %w", err)
	}
	if !_guild.GuildStatus(g.Status).IsPending() {
		return 0, ErrGuildNotPending
	}

	if err := txq.CreateGuildMembership(ctx, db.CreateGuildMembershipParams{
		GuildID:   g.ID,
		KingdomID: kingdomID,
		Role:      string(_guild.RoleSupporter),
	}); err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
			return 0, ErrAlreadyInGuild
		}
		return 0, fmt.Errorf("create membership: %w", err)
	}

	if err := txq.CancelOtherPendingRequests(ctx, db.CancelOtherPendingRequestsParams{
		KingdomID: kingdomID,
		GuildID:   g.ID,
	}); err != nil {
		return 0, fmt.Errorf("cancel pending requests: %w", err)
	}

	count, err := txq.CountGuildSupporters(ctx, g.ID)
	if err != nil {
		return 0, fmt.Errorf("count supporters: %w", err)
	}

	if count >= 5 {
		if err := txq.ActivateGuild(ctx, g.ID); err != nil {
			return 0, fmt.Errorf("activate guild: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}
	return g.ID, nil
}

func isSupportUserError(err error) bool {
	return errors.Is(err, ErrGuildNotFound) ||
		errors.Is(err, ErrGuildNotPending) ||
		errors.Is(err, ErrAlreadyInGuild)
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
		h.publishUpdates(r.Context(), toNotify)
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
		if err != nil {
			log.Printf("guild cancel proposal: get membership: %v", err)
			sse.PatchElementGostar(guildAlert(AlertError(errors.New("internal error"))))
			return
		}
		if _guild.MemberRole(membership.Role) != _guild.RoleApplicant {
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

// errGuildNotFound is an unexported alias kept for backwards-compatibility with
// existing handlers (handleView, handleViewRefresh) that switch on it directly.
// Prefer ErrGuildNotFound externally.
var errGuildNotFound = ErrGuildNotFound

func (h *handler) loadGuildAndMembership(ctx context.Context, slug string, kingdomID int) (db.Guild, []db.ListGuildMembersWithNamesRow, _guild.MemberRole, error) {
	g, err := h.queries.GetGuildBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Guild{}, nil, _guild.RoleNone, ErrGuildNotFound
		}
		return db.Guild{}, nil, _guild.RoleNone, fmt.Errorf("get guild: %w", err)
	}

	members, err := h.queries.ListGuildMembersWithNames(ctx, g.ID)
	if err != nil {
		return db.Guild{}, nil, _guild.RoleNone, fmt.Errorf("list members: %w", err)
	}

	membership, err := h.queries.GetMembershipByKingdomAndGuild(ctx, db.GetMembershipByKingdomAndGuildParams{
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
		if km, err := h.queries.GetKingdomGuildMembership(ctx, kingdomID); err == nil && km.GuildID != g.ID {
			viewerRole = _guild.RoleInOtherGuild
		}
	}

	return g, members, viewerRole, nil
}

func (h *handler) getGuildAndViewerRole(ctx context.Context, slug string, kingdomID int) (db.Guild, _guild.MemberRole, error) {
	g, err := h.queries.GetGuildBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Guild{}, _guild.RoleNone, ErrGuildNotFound
		}
		return db.Guild{}, _guild.RoleNone, fmt.Errorf("get guild: %w", err)
	}
	membership, err := h.queries.GetMembershipByKingdomAndGuild(ctx, db.GetMembershipByKingdomAndGuildParams{
		KingdomID: kingdomID,
		GuildID:   g.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return g, _guild.RoleNone, nil
		}
		return db.Guild{}, _guild.RoleNone, fmt.Errorf("get membership: %w", err)
	}
	return g, _guild.MemberRole(membership.Role), nil
}

// publishUpdates fetches the kingdoms for the given IDs and publishes each to the hub,
// triggering live page refreshes for those kingdoms. Errors are logged but not propagated.
func (h *handler) publishUpdates(ctx context.Context, kingdomIDs []int) {
	if len(kingdomIDs) == 0 {
		return
	}
	kingdoms, err := h.queries.GetKingdomsByIDs(ctx, kingdomIDs)
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
func (h *handler) sendNotifications(ctx context.Context, fromKingdomID int, toKingdomIDs []int, subject, body, actionURL, actionText string) {
	if len(toKingdomIDs) == 0 {
		return
	}
	if err := h.queries.BulkCreateMessages(ctx, db.BulkCreateMessagesParams{
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
	h.publishUpdates(ctx, toKingdomIDs)
}

// ── Page components ───────────────────────────────────────────────────────────

func guildAlert(inner Node) Node { return AlertContainer("guild-alert", inner) }

func guildLandingContent(invitations []db.ListKingdomInvitationsRow) Node {
	return Div(
		Div(Class("page-header"),
			Div(Class("page-header-kicker"), Text("Fellowship")),
			H1(Class("page-header-title"), Text("Guild Hall")),
		),
		Div(ds.Init(GetSSENoSignals(routes.GuildRefreshPath))),
		Iff(len(invitations) > 0, func() Node {
			return Div(Class("card"), Div(Class("card-inner"),
				Div(Class("section-header"),
					Div(Class("section-title"), Text("Your Invitations")),
					Div(Class("section-rule")),
				),
				Table(Class("table"),
					THead(Tr(
						Th(Text("Guild")),
						Th(Class("is-actions")),
					)),
					TBody(Map(invitations, func(inv db.ListKingdomInvitationsRow) Node {
						return Tr(
							Td(Span(Class("table-id-name"),
								A(Href(slugURL(routes.GuildViewPath, inv.GuildSlug)), Text(inv.GuildName)),
							)),
							Td(Class("is-actions"), Div(Class("table-actions"),
								Button(Class("btn btn--sm"),
									ds.On("click", datastar.PostSSE("%s", memberActionURL(routes.GuildInvitationAcceptPath, inv.GuildSlug, inv.ID))),
									Text("Accept"),
								),
								Button(Class("btn btn--sm btn--danger"),
									ds.On("click", datastar.PostSSE("%s", memberActionURL(routes.GuildInvitationDeclinePath, inv.GuildSlug, inv.ID))),
									Text("Decline"),
								),
							)),
						)
					})),
				),
			))
		}),
		Div(Class("doors"),
			Div(Class("door"), Div(Class("card"), Div(Class("card-inner door-inner"),
				Span(Class("door-crest"), Icon("star", 32, false)),
				Div(Class("door-title"), Text("Find a Fellowship")),
				P(Class("door-text"), Text("Browse active guilds and petition to join one.")),
				A(Href(routes.GuildListPath), Class("btn"), Text("Browse Guilds")),
			))),
			Div(Class("doors-seam"),
				Div(Class("seam-rule")),
				Span(Class("seam-or"), Text("or")),
				Div(Class("seam-rule")),
			),
			Div(Class("door"), Div(Class("card"), Div(Class("card-inner door-inner"),
				Span(Class("door-crest"), Icon("star", 32, false)),
				Div(Class("door-title"), Text("Found a Guild")),
				P(Class("door-text"), Text("Submit a founding charter and gather four seals of support.")),
				A(Href(routes.GuildNewPath), Class("btn"), Text("Draft a Charter")),
			))),
		),
		guildAlert(nil),
	)
}

func guildListContent(activeGuilds []db.ListActiveGuildsRow, pendingGuilds []db.ListPendingGuildsRow) Node {
	return Div(
		Div(Class("page-header"),
			Div(Class("page-header-kicker"), Text("Fellowship")),
			H1(Class("page-header-title"), Text("The Guild Roll")),
		),
		Div(Class("card"), Div(Class("card-inner"),
			Div(Class("section-header"),
				Div(Class("section-title"), Text("Active Fellowships")),
				Div(Class("section-rule")),
				Iff(len(activeGuilds) > 0, func() Node {
					return Span(Class("section-meta"), Text(fmt.Sprintf("%d guilds", len(activeGuilds))))
				}),
			),
			Iff(len(activeGuilds) == 0, func() Node {
				return Div(Class("empty-state"),
					Icon("star", 30, false),
					Div(Class("empty-state-title"), Text("The roll is empty")),
					Div(Class("empty-state-hint"), Text("No fellowship has yet been founded in the realm. Yours could be the first.")),
					A(Href(routes.GuildNewPath), Class("btn"), Text("Draft a Founding Charter")),
				)
			}),
			Iff(len(activeGuilds) > 0, func() Node {
				return Table(Class("table"),
					THead(Tr(
						Th(Text("Fellowship")),
						Th(Text("Leader")),
						Th(Class("is-num"), Text("Members")),
					)),
					TBody(Map(activeGuilds, func(g db.ListActiveGuildsRow) Node {
						leader := "—"
						if g.LeaderName.Valid {
							leader = g.LeaderName.String
						}
						return Tr(
							Td(Div(Class("table-id"),
								guildCrestSm(),
								Span(Class("table-id-name"),
									A(Href(slugURL(routes.GuildViewPath, g.Slug)), Text(g.Name)),
								),
							)),
							Td(Text(leader)),
							Td(Class("is-num"), Text(fmt.Sprintf("%d", g.MemberCount)),
								El("span", Class("num-cap"), Text(" / 20")),
							),
						)
					})),
				)
			}),
		)),
		Iff(len(pendingGuilds) > 0, func() Node {
			return Div(Class("card"), Div(Class("card-inner"),
				Div(Class("section-header"),
					Div(Class("section-title"), Text("Founding Charters")),
					Div(Class("section-rule")),
				),
				Table(Class("table"),
					THead(Tr(
						Th(Text("Charter")),
						Th(Text("Founder")),
						Th(Class("is-num"), Text("Seals")),
						Th(Text("Lapses")),
					)),
					TBody(Map(pendingGuilds, func(g db.ListPendingGuildsRow) Node {
						founder := "—"
						if g.FounderName.Valid {
							founder = g.FounderName.String
						}
						expiry := formatExpiry(g.ExpiresAt)
						expiryUrgent := int(time.Until(g.ExpiresAt).Hours()/24) <= 1
						return Tr(
							Td(Span(Class("table-id-name"),
								A(Href(slugURL(routes.GuildViewPath, g.Slug)), Text(g.Name)),
							)),
							Td(Text(founder)),
							Td(Class("is-num"),
								Text(fmt.Sprintf("%d", g.SupporterCount)),
								El("span", Class("num-cap"), Text(" / 5")),
							),
							Td(If(expiryUrgent,
								El("span", Class("is-urgent"), Text(expiry)),
							), If(!expiryUrgent,
								Text(expiry),
							)),
						)
					})),
				),
			))
		}),
	)
}

func guildCrestSm() Node {
	return El("span", Class("guild-crest guild-crest--sm guild-crest--empty"),
		Icon("star", 15, false),
	)
}

func guildNewContent() Node {
	return Div(
		Div(Class("page-header"),
			Div(Class("page-header-kicker"), Text("Fellowship")),
			H1(Class("page-header-title"), Text("Founding Charter")),
		),
		Div(Class("charter-grid"),
			Div(Class("card"), Div(Class("card-inner charter-form"),
				Div(Class("field-group"),
					Label(Class("field-label"), For("guild-name-input"), Text("Name of the Fellowship")),
					Input(ID("guild-name-input"), Class("field"), Type("text"),
						ds.Bind("guild_name"),
						Placeholder("e.g. La Table Ronde"),
						MinLength("5"), MaxLength("60"),
					),
				),
				Div(Class("field-group"),
					Label(Class("field-label"), For("guild-desc-input"), Text("The Charter")),
					Div(Class("field-hint"), Text("Describe the fellowship's purpose. Up to 500 characters.")),
					El("textarea", ID("guild-desc-input"), Class("field"),
						ds.Bind("guild_description"),
						Placeholder("Inscribe the charter of the fellowship…"),
						MaxLength("500"),
					),
				),
				Div(Class("charter-actions"),
					Button(Class("btn"),
						ds.On("click", datastar.PostSSE(routes.GuildCreatePath)),
						Text("Submit Charter"),
					),
					P(Class("charter-oath"), Text("Four kingdoms must affix their seal before the guild is founded.")),
				),
				guildAlert(nil),
			)),
			Div(Class("card"), Div(Class("card-inner"),
				Div(Class("card-header"), H2(Class("card-title"), Text("The Founding Rite"))),
				El("ol", Class("rite-list"),
					El("li", Div(Class("rite-body"),
						Div(Class("rite-name"), Text("Draft the Charter")),
						P(Class("rite-text"), Text("Set down the fellowship's name and purpose. The name must be unique across the realm.")),
					)),
					El("li", Div(Class("rite-body"),
						Div(Class("rite-name"), Text("Gather the Seals")),
						P(Class("rite-text"), Text("Four other kingdoms must pledge their seal. The charter then appears on the Guild Roll for others to find.")),
					)),
					El("li", Div(Class("rite-body"),
						Div(Class("rite-name"), Text("Founding")),
						P(Class("rite-text"), Text("At the fifth seal the guild is brought into being and you become its first leader.")),
					)),
				),
				P(Class("rite-note"), Text("An unfounded charter lapses after 30 days if it does not gather five seals.")),
			)),
		),
	)
}

func guildViewContent(g db.Guild, members []db.ListGuildMembersWithNamesRow, viewerRole _guild.MemberRole, invitationID int) Node {
	isPending := _guild.GuildStatus(g.Status).IsPending()
	supportCount, activeCount := 0, 0
	for _, m := range members {
		if _guild.MemberRole(m.Role).IsApplicationPhase() {
			supportCount++
		}
		if _guild.MemberRole(m.Role).IsActiveMember() {
			activeCount++
		}
	}
	standing := guildViewerStanding(g, viewerRole, invitationID, supportCount)
	swornRoles := viewerRole == _guild.RoleMember || viewerRole == _guild.RoleOfficer || viewerRole == _guild.RoleLeader
	standingAtFoot := swornRoles
	return Div(
		Div(ds.Init(GetSSENoSignals("%s", slugURL(routes.GuildViewRefreshPath, g.Slug)))),
		guildHead(g, isPending, activeCount),
		If(!standingAtFoot, standingNode(standing)),
		If(isPending, guildSealSection(members, supportCount, g)),
		If(!isPending, guildMemberSection(members, activeCount)),
		If(standingAtFoot, standingNode(standing)),
		guildAlert(nil),
	)
}

func guildHead(g db.Guild, isPending bool, activeCount int) Node {
	kicker := "Fellowship"
	if isPending {
		kicker = "Founding Charter"
	}
	sub := g.Description
	return Div(Class("guild-head"),
		El("span", Class("guild-crest guild-crest--lg guild-crest--empty"),
			Icon("star", 30, false),
		),
		Div(Class("page-header"),
			Div(Class("page-header-kicker"), Text(kicker)),
			H1(Class("page-header-title"), Text(g.Name)),
			If(sub != "", Div(Class("page-header-sub"), Text("“"+sub+"”"))),
		),
	)
}

func guildMemberSection(members []db.ListGuildMembersWithNamesRow, activeCount int) Node {
	return Div(
		Div(Class("section-header"),
			Div(Class("section-title"), Text("Sworn Members")),
			Div(Class("section-rule")),
			Span(Class("section-meta"), Text(fmt.Sprintf("%d of 20 kingdoms sworn", activeCount))),
		),
		Div(Class("card"), Div(Class("card-inner"),
			If(len(members) == 0,
				Div(Class("empty-state"),
					Icon("star", 30, false),
					Div(Class("empty-state-title"), Text("No members yet")),
				),
			),
			If(len(members) > 0,
				Table(Class("table"),
					THead(Tr(
						Th(Text("Kingdom")),
						Th(Text("Standing")),
					)),
					TBody(Map(members, func(m db.ListGuildMembersWithNamesRow) Node {
						return Tr(
							Td(Span(Class("table-id-name"), Text(m.KingdomName))),
							Td(guildRoleTag(_guild.MemberRole(m.Role))),
						)
					})),
				),
			),
		)),
	)
}

func guildSealSection(members []db.ListGuildMembersWithNamesRow, supportCount int, g db.Guild) Node {
	return Div(
		Div(Class("meter meter--support"), Style("margin: 0 2px 18px"),
			Div(Class("meter-top"),
				El("span", Class("meter-name"), Text("Seals of support")),
				El("span", Class("meter-eta"), Text(fmt.Sprintf("%d of 5 pledged", supportCount))),
			),
			Div(Class("meter-track"), Attr("style", fmt.Sprintf("--meter-steps:5")),
				Div(Class("meter-fill"), Style(fmt.Sprintf("width:%.0f%%", float64(supportCount)/5*100))),
				Div(Class("meter-ticks")),
			),
		),
		Div(Class("section-header"),
			Div(Class("section-title"), Text("Seals upon the Charter")),
			Div(Class("section-rule")),
		),
		Div(Class("card"), Div(Class("card-inner"),
			If(len(members) == 0,
				Div(Class("empty-state"),
					Div(Class("empty-state-title"), Text("No seals yet")),
					Div(Class("empty-state-hint"), Text("Be the first kingdom to lend your support.")),
				),
			),
			If(len(members) > 0,
				Table(Class("table"),
					THead(Tr(
						Th(Text("Kingdom")),
						Th(Text("Standing")),
					)),
					TBody(Map(members, func(m db.ListGuildMembersWithNamesRow) Node {
						role := _guild.MemberRole(m.Role)
						var roleNode Node
						if role == _guild.RoleApplicant {
							roleNode = El("span", Class("role-tag role-tag--supporter"),
								Icon("star", 12, false), Text("Founder"),
							)
						} else {
							roleNode = guildRoleTag(role)
						}
						return Tr(
							Td(Span(Class("table-id-name"), Text(m.KingdomName))),
							Td(roleNode),
						)
					})),
				),
			),
		)),
	)
}

func guildRoleTag(role _guild.MemberRole) Node {
	mod := ""
	switch role {
	case _guild.RoleLeader:
		mod = " role-tag--leader"
	case _guild.RoleOfficer:
		mod = " role-tag--officer"
	case _guild.RoleSupporter:
		mod = " role-tag--supporter"
	}
	return El("span", Class("role-tag"+mod),
		If(role == _guild.RoleLeader, Icon("crown", 12, false)),
		Text(role.Display()),
	)
}

type guildStanding struct {
	cls     string
	buttons []guildBtn
	note    string
}

type guildBtn struct {
	label  string
	mod    string
	action string
}

func standingNode(s guildStanding) Node {
	return Div(Class(s.cls),
		Iff(len(s.buttons) > 0, func() Node {
			return Div(Class("standing-actions"), Map(s.buttons, func(b guildBtn) Node {
				return Button(Class("btn"+b.mod), ds.On("click", b.action), Text(b.label))
			}))
		}),
		If(s.note != "", Div(Class("standing-note"), Text(s.note))),
	)
}

func guildViewerStanding(g db.Guild, viewerRole _guild.MemberRole, invitationID int, supportCount int) guildStanding {
	slug := g.Slug
	isPending := _guild.GuildStatus(g.Status).IsPending()
	isActive := _guild.GuildStatus(g.Status).IsActive()
	foot := viewerRole == _guild.RoleMember || viewerRole == _guild.RoleOfficer || viewerRole == _guild.RoleLeader
	cls := "standing-bare"
	if foot {
		cls += " standing-bare--foot"
	}

	switch {
	case isPending && viewerRole == _guild.RoleNone:
		remaining := 5 - supportCount
		word := fmt.Sprintf("%d seals are yet wanting", remaining)
		if remaining == 1 {
			word = "One seal is wanting — yours would found the guild."
		}
		return guildStanding{cls: cls, note: word, buttons: []guildBtn{
			{label: "Pledge Your Seal", action: datastar.PostSSE("%s", slugURL(routes.GuildSupportPath, slug))},
		}}
	case isPending && viewerRole == _guild.RoleSupporter:
		return guildStanding{cls: cls, note: "Your seal is upon this charter.", buttons: []guildBtn{
			{label: "Withdraw Your Seal", mod: " btn--danger", action: datastar.PostSSE("%s", slugURL(routes.GuildWithdrawSupportPath, slug))},
		}}
	case isPending && viewerRole == _guild.RoleApplicant:
		return guildStanding{cls: cls, note: "At the fifth seal the guild is founded, with you as its leader.", buttons: []guildBtn{
			{label: "Withdraw the Charter", mod: " btn--danger", action: datastar.PostSSE("%s", slugURL(routes.GuildCancelProposalPath, slug))},
		}}
	case isPending && viewerRole == _guild.RoleInOtherGuild:
		return guildStanding{cls: cls, note: "Your banner is already pledged to another fellowship; you may lend no seal."}
	case isActive && viewerRole == _guild.RoleNone && invitationID == 0:
		return guildStanding{cls: cls, note: "The officers will weigh your request.", buttons: []guildBtn{
			{label: "Request to Join", action: datastar.PostSSE("%s", slugURL(routes.GuildRequestJoinPath, slug))},
		}}
	case isActive && invitationID != 0:
		return guildStanding{cls: cls, note: "You are bidden to this fellowship.", buttons: []guildBtn{
			{label: "Accept the Invitation", action: datastar.PostSSE("%s", memberActionURL(routes.GuildInvitationAcceptPath, slug, invitationID))},
			{label: "Decline", mod: " btn--danger", action: datastar.PostSSE("%s", memberActionURL(routes.GuildInvitationDeclinePath, slug, invitationID))},
		}}
	case isActive && viewerRole == _guild.RolePendingApproval:
		return guildStanding{cls: cls, note: "Your request is before the officers.", buttons: []guildBtn{
			{label: "Withdraw the Request", mod: " btn--danger", action: datastar.PostSSE("%s", slugURL(routes.GuildCancelRequestPath, slug))},
		}}
	case isActive && viewerRole == _guild.RoleMember:
		return guildStanding{cls: cls, buttons: []guildBtn{
			{label: "Leave the Guild", mod: " btn--danger", action: datastar.PostSSE("%s", slugURL(routes.GuildLeavePath, slug))},
		}}
	case isActive && viewerRole == _guild.RoleOfficer:
		return guildStanding{cls: cls, buttons: []guildBtn{
			{label: "Manage the Guild", action: fmt.Sprintf(`window.location="%s"`, slugURL(routes.GuildManagePath, slug))},
			{label: "Leave the Guild", mod: " btn--danger", action: datastar.PostSSE("%s", slugURL(routes.GuildLeavePath, slug))},
		}}
	case isActive && viewerRole == _guild.RoleLeader:
		return guildStanding{cls: cls,
			note: "To quit the fellowship, the banner must first pass to another.",
			buttons: []guildBtn{
				{label: "Manage the Guild", action: fmt.Sprintf(`window.location="%s"`, slugURL(routes.GuildManagePath, slug))},
			},
		}
	case viewerRole == _guild.RoleInOtherGuild:
		return guildStanding{cls: cls, note: "Your banner is already pledged to another fellowship."}
	}
	return guildStanding{cls: cls}
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
