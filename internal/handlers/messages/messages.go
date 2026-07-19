package messages

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/components"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	g "bahago/internal/guild"
	"bahago/internal/hub"
	. "bahago/internal/ui"
	"bahago/internal/router"
	"bahago/internal/routes"
)

// ── Input structs ─────────────────────────────────────────────────────────────

type composeInput struct {
	To      string `json:"msg_to"`
	Subject string `json:"msg_subject"`
	Body    string `json:"msg_body"`
}

type guildMsgInput struct {
	Subject string `json:"msg_subject"`
	Body    string `json:"msg_body"`
	Target  string `json:"guild_msg_target"`
}

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	// Validator sentinels.
	ErrRecipientRequired = errors.New("recipient is required")
	ErrSubjectRequired   = errors.New("subject is required")
	ErrBodyRequired      = errors.New("message body is required")
	ErrBodyTooLong       = errors.New("message body must be 5000 characters or fewer")
	ErrInvalidRecipientGroup = errors.New("invalid recipient group")

	// Orchestrator sentinels.
	ErrTooManyRecipients      = errors.New("cannot send to more than 20 recipients at once")
	ErrSelfSend               = errors.New("cannot send a message to yourself")
	ErrUnknownRecipients      = errors.New("unknown kingdom(s)")
	ErrNotInGuild             = errors.New("not authorized to send guild messages")
	ErrNotAuthorizedAll       = errors.New("not authorized to message all members")
	ErrNotAuthorizedOfficers  = errors.New("not authorized to message officers")
	ErrNoGuildRecipients      = errors.New("no recipients found in your guild")
)

// ── Guild message context ─────────────────────────────────────────────────────

// guildMsgCtx carries the information needed to decide which guild-message
// options are available to the current player.
type guildMsgCtx struct {
	GuildID int
	Role    g.MemberRole
	Perms   g.MessagePermissions
}

// guildMsgTarget is one selectable recipient group in the guild-message compose form.
type guildMsgTarget struct {
	Value string
	Label string
}

func (gc *guildMsgCtx) canSendAny() bool {
	return gc != nil && gc.Perms.CanSendAny(gc.Role)
}

// allowedTargets returns the recipient-group options this player may use,
// ordered from most-restricted to least-restricted.
func (gc *guildMsgCtx) allowedTargets() []guildMsgTarget {
	var targets []guildMsgTarget
	if gc.Perms.CanSendToOfficers(gc.Role) {
		targets = append(targets, guildMsgTarget{"officers", "Officers"})
	}
	if gc.Perms.CanSendToAll(gc.Role) {
		targets = append(targets, guildMsgTarget{"all", "All Members"})
	}
	return targets
}

// ── Route registration ────────────────────────────────────────────────────────

// deleteURL substitutes {id} into the delete route path constant.
func deleteURL(id int) string {
	return strings.ReplaceAll(routes.KingdomMessagesDeletePath, "{id}", strconv.Itoa(id))
}

// classifyKind returns "guild", "decree", or "post" from the two discriminating
// booleans. "decree" is an actionable royal notice (carries a link).
func classifyKind(isGuild, hasAction bool) string {
	if isGuild {
		return "guild"
	}
	if hasAction {
		return "decree"
	}
	return "post"
}

// kindFromDetail returns the kind key ("post", "guild", "action") for a detail row.
func kindFromDetail(m *db.GetInboxMessageByIDRow) string {
	return classifyKind(m.IsGuildMessage, m.ActionUrl != "")
}

// kindLabel maps a kind key to its display label.
func kindLabel(k string) string {
	switch k {
	case "guild":
		return "Guild"
	case "decree":
		return "Decree"
	default:
		return "Message"
	}
}

func RegisterRoutes(r router.Router, queries db.Querier, tickHub *hub.Hub) {
	h := &handler{queries: queries, hub: tickHub}
	r.HandleFunc("GET "+routes.KingdomMessagesPath, h.handleMessagesPage())
	r.HandleFunc("GET "+routes.KingdomMessagesRefreshPath, h.handleMessagesRefresh())
	r.HandleFunc("GET "+routes.KingdomMessagesViewPath, h.handleView())
	r.HandleFunc("GET "+routes.KingdomMessagesComposePath, h.handleComposePage())
	r.HandleFunc("POST "+routes.KingdomMessagesSendPath, h.handleSend())
	r.HandleFunc("POST "+routes.KingdomMessagesDeletePath, h.handleDelete())
	r.HandleFunc("POST "+routes.KingdomMessagesDeleteManyPath, h.handleDeleteMany())
	r.HandleFunc("GET "+routes.KingdomMessagesGuildComposePath, h.handleGuildMessageCompose())
	r.HandleFunc("POST "+routes.KingdomMessagesGuildMessageSendPath, h.handleGuildMessageSend())
}

type handler struct {
	queries db.Querier
	hub     *hub.Hub
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (h *handler) handleMessagesPage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		msgs, err := h.queries.ListInboxMessages(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("messages page: list inbox: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		gc, err := h.loadGuildCtx(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("messages page: load guild ctx: %v", err)
		}

		KingdomLayout(r, "Messages", routes.KingdomMessagesPath, kingdom, messagesShell(groupByDate(msgs, time.Now().UTC()), 0, nil, gc)).Render(w)
	}
}

func (h *handler) handleMessagesRefresh() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		selectedMessageID, _ := strconv.Atoi(r.URL.Query().Get("active"))

		sse := datastar.NewSSE(w, r)

		ch, cleanup := h.hub.Subscribe(kingdom.ID)
		defer cleanup()

		for {
			select {
			case <-r.Context().Done():
				return
			case k := <-ch:
				msgs, err := h.queries.ListInboxMessages(r.Context(), k.ID)
				if err != nil {
					log.Printf("messages stream: list inbox: %v", err)
					return
				}
				if err := sse.PatchElementGostar(messagesList(groupByDate(msgs, time.Now().UTC()), selectedMessageID)); err != nil {
					log.Printf("messages stream: patch: %v", err)
					return
				}
			}
		}
	}
}

func (h *handler) handleView() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		idStr := r.URL.Query().Get("id")
		id, err := strconv.Atoi(idStr)
		if err != nil || id <= 0 {
			http.Error(w, "invalid message id", http.StatusBadRequest)
			return
		}

		params := db.GetInboxMessageByIDParams{
			ID:          id,
			ToKingdomID: kingdom.ID,
		}
		msg, err := h.queries.GetInboxMessageByID(r.Context(), params)
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "message not found", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("messages view: get message: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		if !msg.ReadAt.Valid {
			readParams := db.MarkMessageReadParams{
				ID:          msg.ID,
				ToKingdomID: kingdom.ID,
			}
			if err := h.queries.MarkMessageRead(r.Context(), readParams); err != nil {
				log.Printf("messages view: mark read: %v", err)
			}
		}

		msgs, err := h.queries.ListInboxMessages(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("messages view: list inbox: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		gc, err := h.loadGuildCtx(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("messages view: load guild ctx: %v", err)
		}

		KingdomLayout(r, "Messages", routes.KingdomMessagesPath, kingdom, messagesShell(groupByDate(msgs, time.Now().UTC()), msg.ID, viewPanel(&msg), gc)).Render(w)
	}
}

func (h *handler) handleComposePage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		msgs, err := h.queries.ListInboxMessages(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("messages compose: list inbox: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		gc, err := h.loadGuildCtx(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("messages compose: load guild ctx: %v", err)
		}

		to := r.URL.Query().Get("to")
		subject := r.URL.Query().Get("subject")
		KingdomLayout(r, "Messages", routes.KingdomMessagesPath, kingdom, messagesShell(groupByDate(msgs, time.Now().UTC()), 0, composePanel(to, subject), gc)).Render(w)
	}
}

func (h *handler) handleSend() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		input := &composeInput{}
		if err := datastar.ReadSignals(r, input); err != nil {
			log.Printf("messages send: read signals: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(messagesAlert(AlertError(errors.New("invalid request"))))
			return
		}

		if errs := validateComposeInput(input); len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(messagesAlert(AlertError(errs...)))
			return
		}

		if err := h.sendMessages(r.Context(), kingdom.ID, input); err != nil {
			if isSendUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(messagesAlert(AlertError(err)))
				return
			}
			log.Printf("messages send: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(messagesAlert(AlertError(errors.New("internal error"))))
			return
		}

		if err := datastar.NewSSE(w, r).Redirect(routes.KingdomMessagesPath); err != nil {
			log.Printf("messages send: redirect: %v", err)
		}
	}
}

func (h *handler) handleDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		idStr := r.PathValue("id")
		id, err := strconv.Atoi(idStr)
		if err != nil || id <= 0 {
			datastar.NewSSE(w, r).PatchElementGostar(messagesAlert(AlertError(errors.New("invalid message id"))))
			return
		}

		params := db.DeleteMessageParams{
			ID:          id,
			ToKingdomID: kingdom.ID,
		}
		if err := h.queries.DeleteMessage(r.Context(), params); err != nil {
			log.Printf("messages delete: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(messagesAlert(AlertError(errors.New("internal error"))))
			return
		}

		if err := datastar.NewSSE(w, r).Redirect(routes.KingdomMessagesPath); err != nil {
			log.Printf("messages delete: redirect: %v", err)
		}
	}
}

func (h *handler) handleDeleteMany() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, routes.KingdomMessagesPath, http.StatusSeeOther)
			return
		}

		rawIDs := r.Form["ids"]
		ids := make([]int, 0, len(rawIDs))
		for _, s := range rawIDs {
			id, err := strconv.Atoi(s)
			if err != nil || id <= 0 {
				continue
			}
			ids = append(ids, id)
		}

		if len(ids) > 0 {
			if err := h.queries.DeleteMessages(r.Context(), db.DeleteMessagesParams{
				Ids:         ids,
				ToKingdomID: kingdom.ID,
			}); err != nil {
				log.Printf("messages delete-many: %v", err)
			}
		}

		http.Redirect(w, r, routes.KingdomMessagesPath, http.StatusSeeOther)
	}
}

func (h *handler) handleGuildMessageCompose() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		msgs, err := h.queries.ListInboxMessages(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("messages guild compose: list inbox: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		gc, err := h.loadGuildCtx(r.Context(), kingdom.ID)
		if err != nil {
			log.Printf("messages guild compose: load guild ctx: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !gc.canSendAny() {
			http.Redirect(w, r, routes.KingdomMessagesPath, http.StatusFound)
			return
		}

		KingdomLayout(r, "Messages", routes.KingdomMessagesPath, kingdom,
			messagesShell(groupByDate(msgs, time.Now().UTC()), 0, guildMessagePanel(gc.allowedTargets()), gc),
		).Render(w)
	}
}

func (h *handler) handleGuildMessageSend() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		input := &guildMsgInput{}
		if err := datastar.ReadSignals(r, input); err != nil {
			log.Printf("messages guild send: read signals: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(messagesAlert(AlertError(errors.New("invalid request"))))
			return
		}

		if errs := validateGuildMessageInput(input); len(errs) > 0 {
			datastar.NewSSE(w, r).PatchElementGostar(messagesAlert(AlertError(errs...)))
			return
		}

		if err := h.sendGuildMessage(r.Context(), kingdom.ID, input); err != nil {
			if isGuildSendUserError(err) {
				datastar.NewSSE(w, r).PatchElementGostar(messagesAlert(AlertError(err)))
				return
			}
			log.Printf("messages guild send: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(messagesAlert(AlertError(errors.New("internal error"))))
			return
		}

		if err := datastar.NewSSE(w, r).Redirect(routes.KingdomMessagesPath); err != nil {
			log.Printf("messages guild send: redirect: %v", err)
		}
	}
}

// ── Validation ────────────────────────────────────────────────────────────────

func validateComposeInput(in *composeInput) []error {
	var errs []error
	if strings.TrimSpace(in.To) == "" {
		errs = append(errs, ErrRecipientRequired)
	}
	if strings.TrimSpace(in.Subject) == "" {
		errs = append(errs, ErrSubjectRequired)
	}
	if strings.TrimSpace(in.Body) == "" {
		errs = append(errs, ErrBodyRequired)
	} else if len(in.Body) > 5000 {
		errs = append(errs, ErrBodyTooLong)
	}
	return errs
}

func validateGuildMessageInput(in *guildMsgInput) []error {
	var errs []error
	if strings.TrimSpace(in.Subject) == "" {
		errs = append(errs, ErrSubjectRequired)
	}
	if strings.TrimSpace(in.Body) == "" {
		errs = append(errs, ErrBodyRequired)
	} else if len(in.Body) > 5000 {
		errs = append(errs, ErrBodyTooLong)
	}
	if in.Target != "all" && in.Target != "officers" {
		errs = append(errs, ErrInvalidRecipientGroup)
	}
	return errs
}

// ── Orchestration ─────────────────────────────────────────────────────────────

// sendMessages parses recipient names, deduplicates, resolves them to kingdoms
// via a single bulk lookup, and inserts the messages. Returns sentinel errors
// for user-visible cases. Unknown recipients are surfaced via a wrapped error
// that includes the missing names in its message but matches ErrUnknownRecipients
// under errors.Is.
func (h *handler) sendMessages(ctx context.Context, fromKingdomID int, input *composeInput) error {
	names := splitRecipients(input.To)
	if len(names) == 0 {
		return ErrRecipientRequired
	}

	// Deduplicate names case-insensitively, preserving first-occurrence order.
	seen := make(map[string]struct{}, len(names))
	dedupe := names[:0]
	for _, name := range names {
		key := strings.ToLower(name)
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			dedupe = append(dedupe, name)
		}
	}
	names = dedupe

	if len(names) > 20 {
		return ErrTooManyRecipients
	}

	// Resolve all recipient kingdoms in a single query.
	// The name column is CITEXT so matching is case-insensitive.
	kingdoms, err := h.queries.GetKingdomsByNames(ctx, names)
	if err != nil {
		return fmt.Errorf("get kingdoms by names: %w", err)
	}

	// If fewer kingdoms came back than names requested, find and report the missing ones.
	if len(kingdoms) != len(names) {
		found := make(map[string]struct{}, len(kingdoms))
		for _, k := range kingdoms {
			found[strings.ToLower(k.Name)] = struct{}{}
		}
		var unknown []string
		for _, name := range names {
			if _, ok := found[strings.ToLower(name)]; !ok {
				unknown = append(unknown, name)
			}
		}
		// Wrap the sentinel so errors.Is matches AND err.Error() includes the names.
		return fmt.Errorf("%w: %s", ErrUnknownRecipients, strings.Join(unknown, ", "))
	}

	toIDs := make([]int, len(kingdoms))
	for i, k := range kingdoms {
		if k.ID == fromKingdomID {
			return ErrSelfSend
		}
		toIDs[i] = k.ID
	}

	if err := h.queries.BulkCreateMessages(ctx, db.BulkCreateMessagesParams{
		FromKingdomID: fromKingdomID,
		ToKingdomIds:  toIDs,
		Subject:       strings.TrimSpace(input.Subject),
		Body:          strings.TrimSpace(input.Body),
	}); err != nil {
		return fmt.Errorf("bulk create messages: %w", err)
	}

	// Notify each recipient's live SSE connections so their sidebar badge
	// updates immediately rather than waiting for the next game tick.
	for _, k := range kingdoms {
		h.hub.Publish(k)
	}
	return nil
}

// sendGuildMessage loads the sender's guild context, enforces the per-target
// permission, lists the recipient set, and inserts the messages. Returns
// sentinel errors for user-visible cases. The handler-level validator catches
// invalid targets before this is called, so input.Target is always "all" or
// "officers" here.
func (h *handler) sendGuildMessage(ctx context.Context, fromKingdomID int, input *guildMsgInput) error {
	gc, err := h.loadGuildCtx(ctx, fromKingdomID)
	if err != nil {
		return fmt.Errorf("load guild ctx: %w", err)
	}
	if !gc.canSendAny() {
		return ErrNotInGuild
	}

	switch input.Target {
	case "all":
		if !gc.Perms.CanSendToAll(gc.Role) {
			return ErrNotAuthorizedAll
		}
	case "officers":
		if !gc.Perms.CanSendToOfficers(gc.Role) {
			return ErrNotAuthorizedOfficers
		}
	}

	var toIDs []int
	switch input.Target {
	case "all":
		toIDs, err = h.queries.ListGuildMembersExcludingSelf(ctx, db.ListGuildMembersExcludingSelfParams{
			GuildID:          gc.GuildID,
			ExcludeKingdomID: fromKingdomID,
		})
	case "officers":
		toIDs, err = h.queries.ListGuildOfficersExcludingSelf(ctx, db.ListGuildOfficersExcludingSelfParams{
			GuildID:          gc.GuildID,
			ExcludeKingdomID: fromKingdomID,
		})
	}
	if err != nil {
		return fmt.Errorf("list recipients: %w", err)
	}
	if len(toIDs) == 0 {
		return ErrNoGuildRecipients
	}

	if err := h.queries.BulkCreateMessages(ctx, db.BulkCreateMessagesParams{
		FromKingdomID:  fromKingdomID,
		ToKingdomIds:   toIDs,
		Subject:        strings.TrimSpace(input.Subject),
		Body:           strings.TrimSpace(input.Body),
		IsGuildMessage: true,
	}); err != nil {
		return fmt.Errorf("bulk create: %w", err)
	}

	for _, id := range toIDs {
		h.hub.Publish(db.Kingdom{ID: id})
	}
	return nil
}

func isSendUserError(err error) bool {
	return errors.Is(err, ErrRecipientRequired) ||
		errors.Is(err, ErrTooManyRecipients) ||
		errors.Is(err, ErrSelfSend) ||
		errors.Is(err, ErrUnknownRecipients)
}

func isGuildSendUserError(err error) bool {
	return errors.Is(err, ErrNotInGuild) ||
		errors.Is(err, ErrNotAuthorizedAll) ||
		errors.Is(err, ErrNotAuthorizedOfficers) ||
		errors.Is(err, ErrNoGuildRecipients)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// splitRecipients splits a recipient string on commas and semicolons, trims
// whitespace, and drops empty entries.
func splitRecipients(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if name := strings.TrimSpace(p); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// loadGuildCtx fetches the kingdom's active guild membership and guild settings.
// Returns nil, nil when the kingdom is not an active guild member.
func (h *handler) loadGuildCtx(ctx context.Context, kingdomID int) (*guildMsgCtx, error) {
	membership, err := h.queries.GetKingdomGuildMembership(ctx, kingdomID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get membership: %w", err)
	}

	role := g.MemberRole(membership.Role)
	if !role.IsActiveMember() {
		return nil, nil // applicant or supporter phase — not eligible
	}

	guild, err := h.queries.GetGuildByID(ctx, membership.GuildID)
	if err != nil {
		return nil, fmt.Errorf("get guild: %w", err)
	}

	return &guildMsgCtx{
		GuildID: membership.GuildID,
		Role:    role,
		Perms:   g.ParseMessagePermissions(guild.Settings),
	}, nil
}

// ── List helpers ──────────────────────────────────────────────────────────────

// MsgGroup holds messages under a single date label for display in the list.
type MsgGroup struct {
	Label    string
	Messages []db.ListInboxMessagesRow
}

// groupByDate buckets msgs (ordered newest-first) into date-labelled groups.
func groupByDate(msgs []db.ListInboxMessagesRow, now time.Time) []MsgGroup {
	if len(msgs) == 0 {
		return nil
	}
	today := now.UTC().Truncate(24 * time.Hour)
	yesterday := today.AddDate(0, 0, -1)

	var groups []MsgGroup
	var cur *MsgGroup
	for _, m := range msgs {
		day := m.CreatedAt.UTC().Truncate(24 * time.Hour)
		var label string
		switch {
		case day.Equal(today):
			label = "Today"
		case day.Equal(yesterday):
			label = "Yesterday"
		default:
			label = m.CreatedAt.Format("Jan 2")
		}
		if cur == nil || cur.Label != label {
			groups = append(groups, MsgGroup{Label: label})
			cur = &groups[len(groups)-1]
		}
		cur.Messages = append(cur.Messages, m)
	}
	return groups
}

// kind returns "guild", "decree", or "post" for a message row.
func kind(m db.ListInboxMessagesRow) string {
	return classifyKind(m.IsGuildMessage, m.HasAction)
}

// messageMark renders a kind seal: a coloured shape (blue square for post, red
// disc for guild, brass square for decree) with a crown glyph on decrees. lg
// selects the larger size used in the reading pane + compose headers.
func messageMark(k string, lg bool) Node {
	// The "decree" kind label is user-facing domain language, but the CSS
	// variant is named message-mark--official ("decree" is flavour, not a
	// game-mechanic noun — see docs/instructions/ui.md).
	suffix := k
	if k == "decree" {
		suffix = "official"
	}
	cls := "message-mark message-mark--" + suffix
	if lg {
		cls += " message-mark--lg"
	}
	children := []Node{Class(cls), Title(kindLabel(k)), Attr("aria-label", kindLabel(k))}
	if k == "decree" {
		children = append(children, Icon("crown", 20, false))
	}
	return Span(children...)
}

// ── Components ────────────────────────────────────────────────────────────────

func messagesShell(groups []MsgGroup, selectedMessageID int, panel Node, gc *guildMsgCtx) Node {
	actions := []Node{
		A(Href(routes.KingdomMessagesComposePath), Class("btn"), Text("Write a Letter")),
	}
	if gc.canSendAny() {
		actions = append(actions,
			A(Href(routes.KingdomMessagesGuildComposePath), Class("btn btn--accent"), Text("Guild Dispatch")))
	}

	return Group([]Node{
		PageHeader("Messages", Group(actions)),
		Div(Class("messages-panel"),
			Div(Class("messages-left"),
				messagesList(groups, selectedMessageID),
			),
			Div(Class("messages-right"),
				Iff(panel == nil, func() Node {
					return Div(Class("card"),
						Div(Class("card-inner"),
							Div(Class("empty-state"),
								Crest("", 58, ""),
								P(Class("empty-state-title"), Text("No letter in hand")),
								P(Text("Choose a message from the pile, or write a new one.")),
							),
						),
					)
				}),
				Iff(panel != nil, func() Node { return panel }),
			),
		),
		Div(ds.Init(GetSSENoSignals(routes.KingdomMessagesRefreshPath+"?active=%d", selectedMessageID))),
	})
}

func messagesList(groups []MsgGroup, selectedMessageID int) Node {
	return Form(
		ID("messages-form"),
		Method("post"),
		Action(routes.KingdomMessagesDeleteManyPath),
		ds.Signals(map[string]any{
			"filter":         "all",
			"managing":       false,
			"selected_count": 0,
		}),
		Div(
			ID("messages-list"),
			Classes{"card": true, "messages-list": true},
			ds.Class("'is-managing'", "$managing"),
			Div(Class("messages-list-toolbar"),
				filterTabs(groups),
				Button(
					Type("button"),
					Classes{"btn": true, "btn--sm": true, "messages-manage-toggle": true},
					ds.Class("'is-on'", "$managing"),
					ds.Text("$managing ? 'Done' : 'Select'"),
					ds.On("click", "$managing = !$managing; if (!$managing) { document.querySelectorAll('.msg-check:checked').forEach(el => { el.checked = false; }); $selected_count = 0; }"),
				),
			),
			Div(Class("message-list-scroll"),
				If(len(groups) == 0, Div(Class("messages-list-empty"),
					P(Text("No messages have arrived yet.")))),
				Group(Map(groups, func(grp MsgGroup) Node {
					return Group([]Node{
						Div(Class("date-divider"),
							Span(Class("date-divider-label"), Text(grp.Label)),
							Span(Class("date-divider-rule")),
						),
						Group(Map(grp.Messages, func(m db.ListInboxMessagesRow) Node {
							return messageListItem(m, selectedMessageID)
						})),
					})
				})),
			),
			Div(Class("msg-list-foot"),
				Button(
					Type("submit"),
					Classes{"btn": true, "btn--sm": true, "btn--danger": true},
					ds.Attr("disabled", "$selected_count === 0"),
					ds.Show("$managing"),
					Text("Delete selected"),
				),
			),
		),
	)
}

func filterTabs(groups []MsgGroup) Node {
	var unread, guild int
	for _, grp := range groups {
		for _, m := range grp.Messages {
			if !m.ReadAt.Valid {
				unread++
			}
			if m.IsGuildMessage {
				guild++
			}
		}
	}
	return Div(
		Role("tablist"),
		Class("filter-tabs"),
		filterTab("all", "All", 0),
		filterTab("unread", "Unread", unread),
		filterTab("guild", "Guild", guild),
	)
}

func filterTab(value, label string, count int) Node {
	return Button(
		Type("button"),
		Class("filter-tab"),
		ds.Class("'is-on'", fmt.Sprintf("$filter === '%s'", value)),
		ds.On("click", fmt.Sprintf("$filter = '%s'", value)),
		Text(label),
		If(count > 0, Span(Class("filter-count"), Text(strconv.Itoa(count)))),
	)
}

func messageListItem(m db.ListInboxMessagesRow, selectedMessageID int) Node {
	k := kind(m)
	showExpr := fmt.Sprintf("$filter === 'all' || ($filter === 'unread' && %t) || ($filter === 'guild' && %t)",
		!m.ReadAt.Valid, m.IsGuildMessage)
	return Div(
		Classes{
			"message-item":         true,
			"message-item--unread": !m.ReadAt.Valid,
			"message-item--active": m.ID == selectedMessageID,
		},
		ds.Show(showExpr),
		Label(Class("message-item-check"),
			Input(
				Type("checkbox"),
				Name("ids"),
				Value(strconv.Itoa(m.ID)),
				Class("msg-check check"),
				ds.On("change", "$selected_count = document.querySelectorAll('.msg-check:checked').length"),
			),
		),
		messageMark(k, false),
		A(
			Href(fmt.Sprintf("%s?id=%d", routes.KingdomMessagesViewPath, m.ID)),
			Class("message-item-body"),
			P(Class("message-item-subject"), Text(m.Subject)),
			P(Class("message-item-meta"),
				Span(Class("message-item-from"), Text(m.FromKingdomName)),
				Span(Class("message-item-date"), Text(m.CreatedAt.Format("Jan 2, 15:04"))),
			),
		),
		Span(Class("unread-dot"), Attr("aria-hidden", "true")),
	)
}

func viewPanel(m *db.GetInboxMessageByIDRow) Node {
	replySubject := m.Subject
	if !strings.HasPrefix(replySubject, "RE: ") {
		replySubject = "RE: " + replySubject
	}
	replyURL := fmt.Sprintf("%s?to=%s&subject=%s", routes.KingdomMessagesComposePath, url.QueryEscape(m.FromKingdomName), url.QueryEscape(replySubject))
	k := kindFromDetail(m)
	hasAction := m.ActionUrl != ""
	return Div(
		messagesAlert(nil),
		Div(Classes{"card": true, "messages-detail": true},
			Div(Class("detail-meta"),
				Div(Class("detail-from"),
					messageMark(k, true),
					Div(Class("detail-fromblock"),
						P(Class("detail-from-name"), Text(m.FromKingdomName)),
						P(Class("detail-kind"), Text(kindLabel(k))),
					),
				),
				P(Class("detail-date"), Text("Received "+m.CreatedAt.Format("2 Jan 2006, 15:04"))),
			),
			Div(Class("message-detail-wrap"),
				Iff(m.IsGuildMessage || hasAction, func() Node {
					return Span(Class("message-detail-badge"), messageMark(k, false))
				}),
				H2(Class("message-detail-title"), Text(m.Subject)),
				Hr(Class("message-detail-divider")),
				P(Class("message-detail-body"), Text(m.Body)),
				Iff(hasAction, func() Node {
					return Div(Class("message-detail-action"),
						A(Href(m.ActionUrl), Class("btn btn--primary"), Text(m.ActionText)),
						P(Class("message-detail-action-note"), Text("The seal carries you to the matter at hand.")),
					)
				}),
			),
			Div(Class("detail-footer"),
				Div(Class("footer-group"),
					A(Href(replyURL), Class("btn"), Text("Reply")),
				),
				Div(Class("footer-group"),
					Button(
						Classes{"btn": true, "btn--quiet": true, "is-danger": true},
						ds.On("click", datastar.PostSSE("%s", deleteURL(m.ID))),
						Text("Delete"),
					),
				),
			),
		),
	)
}

func composePanel(to, subject string) Node {
	return Div(
		messagesAlert(nil),
		Div(Class("card compose-form"),
			ds.Signals(map[string]any{
				"msg_to":      to,
				"msg_subject": subject,
			}),
			Div(Class("compose-head"),
				H2(Class("card-title"), Text("Write a Letter")),
				messageMark("post", true),
			),
			Div(Class("compose-fields"),
				Div(Class("field-group"),
					Label(Class("field-label"), For("msg-to"), Text("To")),
					Input(
						ID("msg-to"),
						Type("text"),
						Class("field"),
						Placeholder("Kingdom name"),
						ds.Bind("msg_to"),
					),
					Span(Class("field-hint"), Text("Separate several kingdoms with , or ; — twenty at most.")),
				),
				Div(Class("field-group"),
					Label(Class("field-label"), For("msg-subject"), Text("Subject")),
					Input(
						ID("msg-subject"),
						Type("text"),
						Class("field"),
						Placeholder("Subject"),
						ds.Bind("msg_subject"),
					),
				),
				Div(Class("field-group"),
					Label(Class("field-label"), For("msg-body"), Text("Message")),
					Textarea(
						ID("msg-body"),
						Classes{"field": true, "field--area": true},
						Rows("8"),
						Placeholder("Write your message here…"),
						ds.Bind("msg_body"),
					),
				),
			),
			Div(Class("compose-actions"),
				Button(
					Classes{"btn": true, "btn--primary": true},
					ds.On("click", datastar.PostSSE(routes.KingdomMessagesSendPath)),
					Text("Send"),
				),
				A(Href(routes.KingdomMessagesPath), Class("btn btn--quiet"), Text("Cancel")),
				Span(Class("compose-count"),
					ds.Text("(5000 - $msg_body.length).toLocaleString() + ' characters remain'"),
				),
			),
		),
	)
}

func guildMessagePanel(targets []guildMsgTarget) Node {
	defaultTarget := ""
	if len(targets) > 0 {
		defaultTarget = targets[0].Value
	}
	return Div(
		messagesAlert(nil),
		Div(Class("card compose-form"),
			ds.Signals(map[string]any{
				"guild_msg_target": defaultTarget,
			}),
			Div(Class("compose-head"),
				H2(Class("card-title"), Text("Guild Dispatch")),
				messageMark("guild", true),
			),
			Div(Class("compose-fields"),
				Div(Class("field-group"),
					Label(Class("field-label"), For("guild-msg-target"), Text("To")),
					Select(
						ID("guild-msg-target"),
						Class("select"),
						ds.Bind("guild_msg_target"),
						Group(Map(targets, func(t guildMsgTarget) Node {
							return Option(Value(t.Value), Text(t.Label))
						})),
					),
					Span(Class("field-hint"), Text("One copy goes to every member of the chosen rank.")),
				),
				Div(Class("field-group"),
					Label(Class("field-label"), For("msg-subject"), Text("Subject")),
					Input(
						ID("msg-subject"),
						Type("text"),
						Class("field"),
						Placeholder("Subject"),
						ds.Bind("msg_subject"),
					),
				),
				Div(Class("field-group"),
					Label(Class("field-label"), For("msg-body"), Text("Message")),
					Textarea(
						ID("msg-body"),
						Classes{"field": true, "field--area": true},
						Rows("8"),
						Placeholder("Write your dispatch here…"),
						ds.Bind("msg_body"),
					),
				),
			),
			Div(Class("compose-actions"),
				Button(
					Classes{"btn": true, "btn--primary": true},
					ds.On("click", datastar.PostSSE(routes.KingdomMessagesGuildMessageSendPath)),
					Text("Send to the Guild"),
				),
				A(Href(routes.KingdomMessagesPath), Class("btn btn--quiet"), Text("Cancel")),
				Span(Class("compose-count"),
					ds.Text("(5000 - $msg_body.length).toLocaleString() + ' characters remain'"),
				),
			),
		),
	)
}

func messagesAlert(inner Node) Node { return AlertContainer("messages-alert", inner) }
