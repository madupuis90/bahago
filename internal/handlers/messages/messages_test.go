package messages_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/handlers/messages"
	"bahago/internal/hub"
	"bahago/internal/routes"
	"bahago/internal/testhelper"
)

// stubQuerier embeds a nil db.Querier. Any method not explicitly overridden
// panics via a nil pointer dereference, making unexpected DB calls immediately
// visible. Override only the methods a specific test expects to be called.
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

func sendHandler(q db.Querier, h *hub.Hub) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	messages.RegisterRoutes(cr, q, h)
	return cr.Handlers["POST "+routes.KingdomMessagesSendPath]
}

func sendReq(body string, kingdom *db.Kingdom) *http.Request {
	r := httptest.NewRequest("POST", routes.KingdomMessagesSendPath, strings.NewReader(body))
	return r.WithContext(context.WithValue(r.Context(), contextkeys.Kingdom, kingdom))
}

var sender = &db.Kingdom{ID: 1, Name: "Senderia"}

func TestHandleSend_EmptyRecipient(t *testing.T) {
	h := sendHandler(&stubQuerier{}, nil)
	w := httptest.NewRecorder()
	h(w, sendReq(`{"msg_to":"","msg_subject":"Hello","msg_body":"World"}`, sender))
	testhelper.AssertContains(t, w.Body.String(), "recipient is required")
}

func TestHandleSend_WhitespaceRecipient(t *testing.T) {
	h := sendHandler(&stubQuerier{}, nil)
	w := httptest.NewRecorder()
	h(w, sendReq(`{"msg_to":"   ","msg_subject":"Hello","msg_body":"World"}`, sender))
	testhelper.AssertContains(t, w.Body.String(), "recipient is required")
}

func TestHandleSend_EmptySubject(t *testing.T) {
	h := sendHandler(&stubQuerier{}, nil)
	w := httptest.NewRecorder()
	h(w, sendReq(`{"msg_to":"Other","msg_subject":"","msg_body":"World"}`, sender))
	testhelper.AssertContains(t, w.Body.String(), "subject is required")
}

func TestHandleSend_EmptyBody(t *testing.T) {
	h := sendHandler(&stubQuerier{}, nil)
	w := httptest.NewRecorder()
	h(w, sendReq(`{"msg_to":"Other","msg_subject":"Hello","msg_body":""}`, sender))
	testhelper.AssertContains(t, w.Body.String(), "message body is required")
}

func TestHandleSend_BodyTooLong(t *testing.T) {
	h := sendHandler(&stubQuerier{}, nil)
	w := httptest.NewRecorder()
	payload := `{"msg_to":"Other","msg_subject":"Hello","msg_body":"` + strings.Repeat("x", 5001) + `"}`
	h(w, sendReq(payload, sender))
	testhelper.AssertContains(t, w.Body.String(), "5000 characters or fewer")
}

func TestHandleSend_UnknownKingdom(t *testing.T) {
	stub := &stubQuerier{
		onGetKingdomsByNames: func(_ context.Context, _ []string) ([]db.Kingdom, error) {
			return nil, nil // no kingdoms found → all unknown
		},
	}
	h := sendHandler(stub, nil)
	w := httptest.NewRecorder()
	h(w, sendReq(`{"msg_to":"Atlantis","msg_subject":"Hello","msg_body":"World"}`, sender))
	testhelper.AssertContains(t, w.Body.String(), "unknown kingdom")
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

func TestHandleSend_MultipleRecipientsOneUnknown(t *testing.T) {
	stub := &stubQuerier{
		onGetKingdomsByNames: func(_ context.Context, _ []string) ([]db.Kingdom, error) {
			// Only Targeria is known; Atlantis is absent from the result.
			return []db.Kingdom{{ID: 2, Name: "Targeria"}}, nil
		},
	}
	h := sendHandler(stub, nil)
	w := httptest.NewRecorder()
	h(w, sendReq(`{"msg_to":"Targeria,Atlantis","msg_subject":"Hello","msg_body":"World"}`, sender))
	testhelper.AssertContains(t, w.Body.String(), "unknown kingdom")
}

func TestHandleSend_DuplicateRecipientDeduped(t *testing.T) {
	calls := 0
	stub := &stubQuerier{
		onGetKingdomsByNames: func(_ context.Context, names []string) ([]db.Kingdom, error) {
			calls++
			return []db.Kingdom{{ID: 2, Name: "Targeria"}}, nil
		},
		onBulkCreateMessages: func(_ context.Context, arg db.BulkCreateMessagesParams) error {
			if len(arg.ToKingdomIds) != 1 {
				panic("expected exactly 1 recipient after dedup")
			}
			return nil
		},
	}
	h := sendHandler(stub, hub.New())
	w := httptest.NewRecorder()
	h(w, sendReq(`{"msg_to":"Targeria,Targeria","msg_subject":"Hello","msg_body":"World"}`, sender))
	testhelper.AssertContains(t, w.Body.String(), routes.KingdomMessagesPath)
}

func TestHandleSend_TooManyRecipients(t *testing.T) {
	// Build 21 distinct names: K1,K2,...,K21
	var names []string
	for i := range 21 {
		names = append(names, fmt.Sprintf("K%d", i+1))
	}
	payload := `{"msg_to":"` + strings.Join(names, ",") + `","msg_subject":"Hello","msg_body":"World"}`
	h := sendHandler(&stubQuerier{}, nil)
	w := httptest.NewRecorder()
	h(w, sendReq(payload, sender))
	testhelper.AssertContains(t, w.Body.String(), "20 recipients")
}

func TestHandleSend_Success(t *testing.T) {
	recipient := db.Kingdom{ID: 2, Name: "Targeria"}
	stub := &stubQuerier{
		onGetKingdomsByNames: func(_ context.Context, _ []string) ([]db.Kingdom, error) {
			return []db.Kingdom{recipient}, nil
		},
		onBulkCreateMessages: func(_ context.Context, _ db.BulkCreateMessagesParams) error {
			return nil
		},
	}
	h := sendHandler(stub, hub.New())
	w := httptest.NewRecorder()
	h(w, sendReq(`{"msg_to":"Targeria","msg_subject":"Hello","msg_body":"World"}`, sender))
	// Successful send issues a Datastar redirect to the inbox.
	testhelper.AssertContains(t, w.Body.String(), routes.KingdomMessagesPath)
}
