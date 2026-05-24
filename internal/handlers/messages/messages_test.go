package messages

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/hub"
	"bahago/internal/routes"
	"bahago/internal/testhelper"
)

// ── Stub querier ──────────────────────────────────────────────────────────────

type stubQuerier struct {
	db.Querier
	onGetKingdomsByNames func(ctx context.Context, names []string) ([]db.Kingdom, error)
	onBulkCreateMessages func(ctx context.Context, arg db.BulkCreateMessagesParams) error
}

func (s *stubQuerier) GetKingdomsByNames(ctx context.Context, names []string) ([]db.Kingdom, error) {
	if s.onGetKingdomsByNames != nil {
		return s.onGetKingdomsByNames(ctx, names)
	}
	panic("stubQuerier: unexpected call to GetKingdomsByNames")
}

func (s *stubQuerier) BulkCreateMessages(ctx context.Context, arg db.BulkCreateMessagesParams) error {
	if s.onBulkCreateMessages != nil {
		return s.onBulkCreateMessages(ctx, arg)
	}
	panic("stubQuerier: unexpected call to BulkCreateMessages")
}

var sender = &db.Kingdom{ID: 1, Name: "Senderia"}

// ── validateComposeInput ──────────────────────────────────────────────────────

func TestValidateComposeInput(t *testing.T) {
	tests := []struct {
		name     string
		input    *composeInput
		wantErrs []error
	}{
		{"valid", &composeInput{To: "Other", Subject: "Hi", Body: "Hello"}, nil},
		{"empty_to", &composeInput{To: "", Subject: "Hi", Body: "Hello"}, []error{ErrRecipientRequired}},
		{"whitespace_to", &composeInput{To: "   ", Subject: "Hi", Body: "Hello"}, []error{ErrRecipientRequired}},
		{"empty_subject", &composeInput{To: "Other", Subject: "", Body: "Hello"}, []error{ErrSubjectRequired}},
		{"empty_body", &composeInput{To: "Other", Subject: "Hi", Body: ""}, []error{ErrBodyRequired}},
		{"body_too_long", &composeInput{To: "Other", Subject: "Hi", Body: strings.Repeat("x", 5001)}, []error{ErrBodyTooLong}},
		{
			name:     "all_empty",
			input:    &composeInput{},
			wantErrs: []error{ErrRecipientRequired, ErrSubjectRequired, ErrBodyRequired},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validateComposeInput(tc.input)
			if len(got) != len(tc.wantErrs) {
				t.Fatalf("got %d errs (%v), want %d (%v)", len(got), got, len(tc.wantErrs), tc.wantErrs)
			}
			for i, want := range tc.wantErrs {
				if !errors.Is(got[i], want) {
					t.Errorf("errs[%d] = %v, want %v", i, got[i], want)
				}
			}
		})
	}
}

func TestValidateGuildMessageInput(t *testing.T) {
	tests := []struct {
		name     string
		input    *guildMsgInput
		wantErrs []error
	}{
		{"valid_all", &guildMsgInput{Subject: "Hi", Body: "Hello", Target: "all"}, nil},
		{"valid_officers", &guildMsgInput{Subject: "Hi", Body: "Hello", Target: "officers"}, nil},
		{"invalid_target", &guildMsgInput{Subject: "Hi", Body: "Hello", Target: "everyone"}, []error{ErrInvalidRecipientGroup}},
		{"empty_target", &guildMsgInput{Subject: "Hi", Body: "Hello", Target: ""}, []error{ErrInvalidRecipientGroup}},
		{"body_too_long", &guildMsgInput{Subject: "Hi", Body: strings.Repeat("x", 5001), Target: "all"}, []error{ErrBodyTooLong}},
		{
			name:     "all_invalid",
			input:    &guildMsgInput{},
			wantErrs: []error{ErrSubjectRequired, ErrBodyRequired, ErrInvalidRecipientGroup},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validateGuildMessageInput(tc.input)
			if len(got) != len(tc.wantErrs) {
				t.Fatalf("got %d errs (%v), want %d (%v)", len(got), got, len(tc.wantErrs), tc.wantErrs)
			}
			for i, want := range tc.wantErrs {
				if !errors.Is(got[i], want) {
					t.Errorf("errs[%d] = %v, want %v", i, got[i], want)
				}
			}
		})
	}
}

// ── sendMessages ──────────────────────────────────────────────────────────────

func TestSendMessages_EmptyRecipientsAfterSplit(t *testing.T) {
	// Input passes the validator (To non-empty) but contains only separators.
	q := &stubQuerier{}
	h := &handler{queries: q}
	err := h.sendMessages(context.Background(), sender.ID, &composeInput{To: ", ;", Subject: "S", Body: "B"})
	if !errors.Is(err, ErrRecipientRequired) {
		t.Fatalf("err = %v, want ErrRecipientRequired", err)
	}
}

func TestSendMessages_UnknownRecipientsCarriesNames(t *testing.T) {
	q := &stubQuerier{
		onGetKingdomsByNames: func(_ context.Context, _ []string) ([]db.Kingdom, error) {
			return []db.Kingdom{{ID: 2, Name: "Targeria"}}, nil
		},
	}
	h := &handler{queries: q}
	err := h.sendMessages(context.Background(), sender.ID, &composeInput{To: "Targeria,Atlantis,GhostKingdom", Subject: "S", Body: "B"})
	if !errors.Is(err, ErrUnknownRecipients) {
		t.Fatalf("err = %v, want wrapped ErrUnknownRecipients", err)
	}
	// Wrapped error must include the unknown names in its message.
	if !strings.Contains(err.Error(), "Atlantis") || !strings.Contains(err.Error(), "GhostKingdom") {
		t.Errorf("err.Error() = %q, want to include Atlantis and GhostKingdom", err.Error())
	}
}

func TestSendMessages_SelfSend(t *testing.T) {
	q := &stubQuerier{
		onGetKingdomsByNames: func(_ context.Context, _ []string) ([]db.Kingdom, error) {
			return []db.Kingdom{{ID: sender.ID, Name: sender.Name}}, nil
		},
	}
	h := &handler{queries: q}
	err := h.sendMessages(context.Background(), sender.ID, &composeInput{To: "Senderia", Subject: "S", Body: "B"})
	if !errors.Is(err, ErrSelfSend) {
		t.Fatalf("err = %v, want ErrSelfSend", err)
	}
}

func TestSendMessages_TooManyRecipients(t *testing.T) {
	var names []string
	for i := range 21 {
		names = append(names, fmt.Sprintf("K%d", i+1))
	}
	q := &stubQuerier{}
	h := &handler{queries: q}
	err := h.sendMessages(context.Background(), sender.ID, &composeInput{To: strings.Join(names, ","), Subject: "S", Body: "B"})
	if !errors.Is(err, ErrTooManyRecipients) {
		t.Fatalf("err = %v, want ErrTooManyRecipients", err)
	}
}

func TestSendMessages_Success(t *testing.T) {
	recipient := db.Kingdom{ID: 2, Name: "Targeria"}
	var inserted db.BulkCreateMessagesParams
	q := &stubQuerier{
		onGetKingdomsByNames: func(_ context.Context, _ []string) ([]db.Kingdom, error) {
			return []db.Kingdom{recipient}, nil
		},
		onBulkCreateMessages: func(_ context.Context, arg db.BulkCreateMessagesParams) error {
			inserted = arg
			return nil
		},
	}
	h := &handler{queries: q, hub: hub.New()}
	err := h.sendMessages(context.Background(), sender.ID, &composeInput{To: "Targeria", Subject: " Hi ", Body: " Hello "})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if inserted.FromKingdomID != sender.ID {
		t.Errorf("FromKingdomID = %d, want %d", inserted.FromKingdomID, sender.ID)
	}
	if inserted.Subject != "Hi" || inserted.Body != "Hello" {
		t.Errorf("subject/body not trimmed: %+v", inserted)
	}
	if len(inserted.ToKingdomIds) != 1 || inserted.ToKingdomIds[0] != recipient.ID {
		t.Errorf("ToKingdomIds = %v, want [%d]", inserted.ToKingdomIds, recipient.ID)
	}
}

func TestSendMessages_DuplicateRecipientsDedupedToOne(t *testing.T) {
	var inserted db.BulkCreateMessagesParams
	q := &stubQuerier{
		onGetKingdomsByNames: func(_ context.Context, names []string) ([]db.Kingdom, error) {
			if len(names) != 1 {
				t.Fatalf("expected dedupe to produce 1 name, got %d", len(names))
			}
			return []db.Kingdom{{ID: 2, Name: "Targeria"}}, nil
		},
		onBulkCreateMessages: func(_ context.Context, arg db.BulkCreateMessagesParams) error {
			inserted = arg
			return nil
		},
	}
	h := &handler{queries: q, hub: hub.New()}
	err := h.sendMessages(context.Background(), sender.ID, &composeInput{To: "Targeria,Targeria,TARGERIA", Subject: "S", Body: "B"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(inserted.ToKingdomIds) != 1 {
		t.Errorf("ToKingdomIds = %v, want exactly 1 after dedup", inserted.ToKingdomIds)
	}
}

// ── Handler shell smoke tests ─────────────────────────────────────────────────

func sendHandler(q db.Querier, h *hub.Hub) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	RegisterRoutes(cr, q, h)
	return cr.Handlers["POST "+routes.KingdomMessagesSendPath]
}

func sendReq(body string, kingdom *db.Kingdom) *http.Request {
	r := httptest.NewRequest("POST", routes.KingdomMessagesSendPath, strings.NewReader(body))
	return r.WithContext(context.WithValue(r.Context(), contextkeys.Kingdom, kingdom))
}

func TestHandleSend_EmptyRecipient(t *testing.T) {
	h := sendHandler(&stubQuerier{}, nil)
	w := httptest.NewRecorder()
	h(w, sendReq(`{"msg_to":"","msg_subject":"Hello","msg_body":"World"}`, sender))
	testhelper.AssertContains(t, w.Body.String(), "recipient is required")
}

func TestHandleSend_UnknownKingdomReportsName(t *testing.T) {
	stub := &stubQuerier{
		onGetKingdomsByNames: func(_ context.Context, _ []string) ([]db.Kingdom, error) {
			return nil, nil
		},
	}
	h := sendHandler(stub, nil)
	w := httptest.NewRecorder()
	h(w, sendReq(`{"msg_to":"Atlantis","msg_subject":"Hello","msg_body":"World"}`, sender))
	body := w.Body.String()
	testhelper.AssertContains(t, body, "unknown kingdom")
	testhelper.AssertContains(t, body, "Atlantis")
}

func TestHandleSend_SelfSend(t *testing.T) {
	stub := &stubQuerier{
		onGetKingdomsByNames: func(_ context.Context, _ []string) ([]db.Kingdom, error) {
			return []db.Kingdom{{ID: sender.ID, Name: sender.Name}}, nil
		},
	}
	h := sendHandler(stub, nil)
	w := httptest.NewRecorder()
	h(w, sendReq(`{"msg_to":"Senderia","msg_subject":"Hello","msg_body":"World"}`, sender))
	testhelper.AssertContains(t, w.Body.String(), "cannot send a message to yourself")
}

func TestHandleSend_Success(t *testing.T) {
	stub := &stubQuerier{
		onGetKingdomsByNames: func(_ context.Context, _ []string) ([]db.Kingdom, error) {
			return []db.Kingdom{{ID: 2, Name: "Targeria"}}, nil
		},
		onBulkCreateMessages: func(_ context.Context, _ db.BulkCreateMessagesParams) error {
			return nil
		},
	}
	h := sendHandler(stub, hub.New())
	w := httptest.NewRecorder()
	h(w, sendReq(`{"msg_to":"Targeria","msg_subject":"Hello","msg_body":"World"}`, sender))
	testhelper.AssertContains(t, w.Body.String(), routes.KingdomMessagesPath)
}
