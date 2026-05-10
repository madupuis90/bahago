package messages

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/starfederation/datastar-go/datastar"
	. "maragu.dev/gomponents"
	ds "maragu.dev/gomponents-datastar"
	. "maragu.dev/gomponents/components"
	. "maragu.dev/gomponents/html"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/hub"
	. "bahago/internal/layout"
	"bahago/internal/router"
	"bahago/internal/routes"
)

// ── Input structs ─────────────────────────────────────────────────────────────

type composeInput struct {
	To      string `json:"msg_to"`
	Subject string `json:"msg_subject"`
	Body    string `json:"msg_body"`
}

// ── Route registration ────────────────────────────────────────────────────────

// deleteURL substitutes {id} into the delete route path constant.
func deleteURL(id int) string {
	return strings.ReplaceAll(routes.KingdomMessagesDeletePath, "{id}", strconv.Itoa(id))
}

func RegisterRoutes(r router.Router, queries db.Querier, tickHub *hub.Hub) {
	h := &handler{queries: queries, hub: tickHub}
	r.HandleFunc("GET "+routes.KingdomMessagesPath, h.handleMessagesPage())
	r.HandleFunc("GET "+routes.KingdomMessagesRefreshPath, h.handleMessagesRefresh())
	r.HandleFunc("GET "+routes.KingdomMessagesViewPath, h.handleView())
	r.HandleFunc("GET "+routes.KingdomMessagesComposePath, h.handleComposePage())
	r.HandleFunc("POST "+routes.KingdomMessagesSendPath, h.handleSend())
	r.HandleFunc("POST "+routes.KingdomMessagesDeletePath, h.handleDelete())
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

		KingdomLayout(r, "Messages", routes.KingdomMessagesPath, kingdom, messagesShell(msgs, 0, nil)).Render(w)
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
				if err := sse.PatchElementGostar(messagesList(msgs, selectedMessageID)); err != nil {
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

		KingdomLayout(r, "Messages", routes.KingdomMessagesPath, kingdom, messagesShell(msgs, msg.ID, viewPanel(&msg))).Render(w)
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

		to := r.URL.Query().Get("to")
		subject := r.URL.Query().Get("subject")
		KingdomLayout(r, "Messages", routes.KingdomMessagesPath, kingdom, messagesShell(msgs, 0, composePanel(to, subject))).Render(w)
	}
}

func (h *handler) handleSend() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		kingdom := r.Context().Value(contextkeys.Kingdom).(*db.Kingdom)

		input := &composeInput{}
		if err := datastar.ReadSignals(r, input); err != nil {
			log.Printf("messages send: read signals: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(messagesError(errors.New("invalid request")))
			return
		}

		if strings.TrimSpace(input.To) == "" {
			datastar.NewSSE(w, r).PatchElementGostar(messagesError(errors.New("recipient is required")))
			return
		}
		if strings.TrimSpace(input.Subject) == "" {
			datastar.NewSSE(w, r).PatchElementGostar(messagesError(errors.New("subject is required")))
			return
		}
		if strings.TrimSpace(input.Body) == "" {
			datastar.NewSSE(w, r).PatchElementGostar(messagesError(errors.New("message body is required")))
			return
		}
		if len(input.Body) > 5000 {
			datastar.NewSSE(w, r).PatchElementGostar(messagesError(errors.New("message body must be 5000 characters or fewer")))
			return
		}

		names := splitRecipients(input.To)
		if len(names) == 0 {
			datastar.NewSSE(w, r).PatchElementGostar(messagesError(errors.New("recipient is required")))
			return
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
			datastar.NewSSE(w, r).PatchElementGostar(messagesError(errors.New("cannot send to more than 20 recipients at once")))
			return
		}

		// Resolve all recipient kingdoms in a single query.
		// The name column is CITEXT so matching is case-insensitive.
		kingdoms, err := h.queries.GetKingdomsByNames(r.Context(), names)
		if err != nil {
			log.Printf("messages send: get kingdoms by names: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(messagesError(errors.New("internal error")))
			return
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
			datastar.NewSSE(w, r).PatchElementGostar(messagesError(fmt.Errorf("unknown kingdom(s): %s", strings.Join(unknown, ", "))))
			return
		}

		// All names resolved. Build the ID list and check for self-send.
		toIDs := make([]int, len(kingdoms))
		for i, k := range kingdoms {
			if k.ID == kingdom.ID {
				datastar.NewSSE(w, r).PatchElementGostar(messagesError(errors.New("cannot send a message to yourself")))
				return
			}
			toIDs[i] = k.ID
		}

		params := db.BulkCreateMessagesParams{
			FromKingdomID: kingdom.ID,
			ToKingdomIds:  toIDs,
			Subject:       strings.TrimSpace(input.Subject),
			Body:          strings.TrimSpace(input.Body),
		}
		if err := h.queries.BulkCreateMessages(r.Context(), params); err != nil {
			log.Printf("messages send: bulk create messages: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(messagesError(errors.New("internal error")))
			return
		}

		// Notify each recipient's live SSE connections so their sidebar badge
		// updates immediately rather than waiting for the next game tick.
		for _, k := range kingdoms {
			h.hub.Publish(k)
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
			datastar.NewSSE(w, r).PatchElementGostar(messagesError(errors.New("invalid message id")))
			return
		}

		params := db.DeleteMessageParams{
			ID:          id,
			ToKingdomID: kingdom.ID,
		}
		if err := h.queries.DeleteMessage(r.Context(), params); err != nil {
			log.Printf("messages delete: %v", err)
			datastar.NewSSE(w, r).PatchElementGostar(messagesError(errors.New("internal error")))
			return
		}

		if err := datastar.NewSSE(w, r).Redirect(routes.KingdomMessagesPath); err != nil {
			log.Printf("messages delete: redirect: %v", err)
		}
	}
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

// ── Components ────────────────────────────────────────────────────────────────

func messagesShell(msgs []db.ListInboxMessagesRow, selectedMessageID int, panel Node) Node {
	return Group([]Node{
		H1(Class("page-title"), Text("Messages")),
		Div(Class("messages-panel"),
			Div(Class("messages-left"),
				A(Href(routes.KingdomMessagesComposePath), Classes{"btn": true, "messages-compose-btn": true}, Text("Compose")),
				messagesList(msgs, selectedMessageID),
			),
			Div(Class("messages-right"),
				Iff(panel == nil, func() Node {
					return P(Class("messages-empty-state"), Text("Select a message to read it."))
				}),
				Iff(panel != nil, func() Node { return panel }),
			),
		),
		Div(ds.Init(GetSSENoSignals(routes.KingdomMessagesRefreshPath+"?active=%d", selectedMessageID))),
	})
}

func messagesList(msgs []db.ListInboxMessagesRow, selectedMessageID int) Node {
	return Div(ID("messages-list"), Class("messages-list panel"),
		Iff(len(msgs) == 0, func() Node {
			return P(Class("messages-empty-state"), Text("No messages."))
		}),
		Group(Map(msgs, func(m db.ListInboxMessagesRow) Node {
			return messageListItem(m, selectedMessageID)
		})),
	)
}

func messageListItem(m db.ListInboxMessagesRow, selectedMessageID int) Node {
	return A(
		Href(fmt.Sprintf("%s?id=%d", routes.KingdomMessagesViewPath, m.ID)),
		Classes{
			"message-item":         true,
			"message-item--unread": !m.ReadAt.Valid,
			"message-item--active": m.ID == selectedMessageID,
		},
		P(Class("message-item-header"),
			Span(Class("message-item-from"), Text(m.FromKingdomName)),
			Text(": "),
			Span(Class("message-item-subject"), Text(m.Subject)),
		),
		P(Class("message-item-date"), Text(m.CreatedAt.Format("Jan 2, 15:04"))),
	)
}

func viewPanel(m *db.GetInboxMessageByIDRow) Node {
	replySubject := m.Subject
	if !strings.HasPrefix(replySubject, "RE: ") {
		replySubject = "RE: " + replySubject
	}
	replyURL := fmt.Sprintf("%s?to=%s&subject=%s", routes.KingdomMessagesComposePath, url.QueryEscape(m.FromKingdomName), url.QueryEscape(replySubject))
	return Div(
		messagesError(nil),
		Div(Class("messages-detail panel"),
			Div(Class("message-detail-header"),
				P(Class("message-detail-subject"), Span(Class("label"), Text("Subject")), Text(m.Subject)),
				P(Class("message-detail-from"), Span(Class("label"), Text("From: ")), Text(m.FromKingdomName)),
				P(Class("message-detail-date"), Text(m.CreatedAt.Format("2 Jan 2006, 15:04"))),
			),
			Div(Class("message-detail-body"), Text(m.Body)),
			Div(Class("message-detail-footer"),
				Div(
					Iff(m.ActionUrl.Valid && m.ActionUrl.String != "", func() Node {
						return A(Href(m.ActionUrl.String), Class("btn"), Text(m.ActionText.String))
					}),
				),
				Div(
					A(Href(replyURL), Class("btn btn-text"), Text("Reply")),
					Button(
						Class("btn btn-text"),
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
		messagesError(nil),
		Div(Class("compose-form panel"),
			ds.Signals(map[string]any{
				"msg_to":      to,
				"msg_subject": subject,
			}),
			Div(Class("compose-fields"),
				Label(For("msg-to"), Text("To (separate multiple with , or ;)")),
				Input(
					ID("msg-to"),
					Type("text"),
					Placeholder("Kingdom name"),
					ds.Bind("msg_to"),
				),

				Label(For("msg-subject"), Text("Subject")),
				Input(
					ID("msg-subject"),
					Type("text"),
					Placeholder("Subject"),
					ds.Bind("msg_subject"),
				),

				Label(For("msg-body"), Text("Message")),
				Textarea(
					ID("msg-body"),
					Rows("8"),
					Placeholder("Write your message here..."),
					ds.Bind("msg_body"),
				),
			),
			Div(Class("compose-actions"),
				Button(
					Class("btn"),
					ds.On("click", datastar.PostSSE(routes.KingdomMessagesSendPath)),
					Text("Send"),
				),
				A(Href(routes.KingdomMessagesPath), Class("btn btn-text"), Text("Cancel")),
			),
		),
	)
}

func messagesError(err error) Node {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return Div(
		ID("messages-alert"),
		Classes{"alert--error": err != nil},
		Text(msg),
	)
}
