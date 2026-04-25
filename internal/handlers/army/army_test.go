package army_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"bahago/internal/contextkeys"
	"bahago/internal/database/db"
	"bahago/internal/game"
	"bahago/internal/handlers/army"
	"bahago/internal/routes"
	"bahago/internal/testhelper"
)

// stubQuerier embeds a nil db.Querier. Any method not explicitly overridden
// panics via a nil pointer dereference, making unexpected DB calls immediately
// visible. Override only the methods a specific test expects to be called.
type stubQuerier struct {
	db.Querier
	onGetKingdomByName func(ctx context.Context, name string) (db.Kingdom, error)
	onCancelCampaign   func(ctx context.Context, arg db.CancelCampaignParams) (int, error)
}

func (s *stubQuerier) GetKingdomByName(ctx context.Context, name string) (db.Kingdom, error) {
	if s.onGetKingdomByName != nil {
		return s.onGetKingdomByName(ctx, name)
	}
	panic("stubQuerier: unexpected call to GetKingdomByName")
}

func (s *stubQuerier) CancelCampaign(ctx context.Context, arg db.CancelCampaignParams) (int, error) {
	if s.onCancelCampaign != nil {
		return s.onCancelCampaign(ctx, arg)
	}
	panic("stubQuerier: unexpected call to CancelCampaign")
}

// sendHandler extracts the POST send handler from RegisterRoutes.
func sendHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	army.RegisterRoutes(cr, q, nil, nil)
	return cr.Handlers["POST "+routes.KingdomArmySendPath]
}

// sendReq builds a POST request with the given JSON signals body and the
// provided kingdom injected into context.
func sendReq(body string, kingdom *db.Kingdom) *http.Request {
	r := httptest.NewRequest("POST", routes.KingdomArmySendPath, strings.NewReader(body))
	return r.WithContext(context.WithValue(r.Context(), contextkeys.Kingdom, kingdom))
}

var attacker = &db.Kingdom{ID: 1, X: 0, Y: 0, Name: "Attackia"}

func TestHandleSend_UnknownUnitType(t *testing.T) {
	h := sendHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, sendReq(`{"unit_type":"dragon","send_count":10,"action":"attack","target_name":"Other","duration_ticks":8}`, attacker))
	testhelper.AssertContains(t, w.Body.String(), "unknown unit type")
}

func TestHandleSend_CountZero(t *testing.T) {
	h := sendHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, sendReq(`{"unit_type":"recruit","send_count":0,"action":"attack","target_name":"Other","duration_ticks":8}`, attacker))
	testhelper.AssertContains(t, w.Body.String(), "count must be at least 1")
}

func TestHandleSend_CountNegative(t *testing.T) {
	h := sendHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, sendReq(`{"unit_type":"recruit","send_count":-5,"action":"attack","target_name":"Other","duration_ticks":8}`, attacker))
	testhelper.AssertContains(t, w.Body.String(), "count must be at least 1")
}

func TestHandleSend_CountTooLarge(t *testing.T) {
	h := sendHandler(&stubQuerier{})
	body := fmt.Sprintf(`{"unit_type":"recruit","send_count":%d,"action":"attack","target_name":"Other","duration_ticks":8}`, game.MaxUnitInput+1)
	w := httptest.NewRecorder()
	h(w, sendReq(body, attacker))
	testhelper.AssertContains(t, w.Body.String(), "count is too large")
}

func TestHandleSend_InvalidAction(t *testing.T) {
	h := sendHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, sendReq(`{"unit_type":"recruit","send_count":10,"action":"pillage","target_name":"Other","duration_ticks":8}`, attacker))
	testhelper.AssertContains(t, w.Body.String(), "invalid action")
}

func TestHandleSend_DurationNotDivisibleBy4(t *testing.T) {
	h := sendHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, sendReq(`{"unit_type":"recruit","send_count":10,"action":"attack","target_name":"Other","duration_ticks":5}`, attacker))
	testhelper.AssertContains(t, w.Body.String(), "invalid duration")
}

func TestHandleSend_AttackDurationTooLarge(t *testing.T) {
	h := sendHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	// Attack max is 20; 24 exceeds it.
	h(w, sendReq(`{"unit_type":"recruit","send_count":10,"action":"attack","target_name":"Other","duration_ticks":24}`, attacker))
	testhelper.AssertContains(t, w.Body.String(), "invalid duration")
}

func TestHandleSend_DefendDurationTooLarge(t *testing.T) {
	h := sendHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	// Defend max is 96; 100 exceeds it.
	h(w, sendReq(`{"unit_type":"recruit","send_count":10,"action":"defend","target_name":"Other","duration_ticks":100}`, attacker))
	testhelper.AssertContains(t, w.Body.String(), "invalid duration")
}

func TestHandleSend_DefendDurationBelowMinimum(t *testing.T) {
	h := sendHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	// Min duration is 4; 0 is below it.
	h(w, sendReq(`{"unit_type":"recruit","send_count":10,"action":"defend","target_name":"Other","duration_ticks":0}`, attacker))
	testhelper.AssertContains(t, w.Body.String(), "invalid duration")
}

func TestHandleSend_EmptyTargetName(t *testing.T) {
	h := sendHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, sendReq(`{"unit_type":"recruit","send_count":10,"action":"attack","target_name":"","duration_ticks":8}`, attacker))
	testhelper.AssertContains(t, w.Body.String(), "target kingdom name is required")
}

func TestHandleSend_TargetNotFound(t *testing.T) {
	stub := &stubQuerier{
		onGetKingdomByName: func(_ context.Context, _ string) (db.Kingdom, error) {
			return db.Kingdom{}, pgx.ErrNoRows
		},
	}
	h := sendHandler(stub)
	w := httptest.NewRecorder()
	h(w, sendReq(`{"unit_type":"recruit","send_count":10,"action":"attack","target_name":"Atlantis","duration_ticks":8}`, attacker))
	testhelper.AssertContains(t, w.Body.String(), "not found")
}

func TestHandleSend_CannotTargetOwnKingdom(t *testing.T) {
	stub := &stubQuerier{
		// Return a kingdom with the same ID as attacker.
		onGetKingdomByName: func(_ context.Context, _ string) (db.Kingdom, error) {
			return db.Kingdom{ID: attacker.ID, Name: attacker.Name}, nil
		},
	}
	h := sendHandler(stub)
	w := httptest.NewRecorder()
	h(w, sendReq(`{"unit_type":"recruit","send_count":10,"action":"attack","target_name":"Attackia","duration_ticks":8}`, attacker))
	testhelper.AssertContains(t, w.Body.String(), "cannot target your own kingdom")
}

// ── handleCancel tests ────────────────────────────────────────────────────────

// cancelHandler extracts the POST cancel handler from RegisterRoutes.
func cancelHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	army.RegisterRoutes(cr, q, nil, nil)
	return cr.Handlers["POST "+routes.KingdomArmyCancelPath+"/{id}"]
}

// cancelReq builds a POST request with the campaign id as a path variable and
// the provided kingdom injected into context.
func cancelReq(id int, kingdom *db.Kingdom) *http.Request {
	url := fmt.Sprintf("%s/%d", routes.KingdomArmyCancelPath, id)
	r := httptest.NewRequest("POST", url, nil)
	// Simulate the ServeMux path variable that the real router would set.
	r.SetPathValue("id", strconv.Itoa(id))
	return r.WithContext(context.WithValue(r.Context(), contextkeys.Kingdom, kingdom))
}

func TestHandleCancel_CampaignNotFound(t *testing.T) {
	stub := &stubQuerier{
		onCancelCampaign: func(_ context.Context, _ db.CancelCampaignParams) (int, error) {
			return 0, pgx.ErrNoRows
		},
	}
	h := cancelHandler(stub)
	w := httptest.NewRecorder()
	h(w, cancelReq(99, attacker))
	testhelper.AssertContains(t, w.Body.String(), "campaign not found or already returning")
}

func TestHandleCancel_InvalidInput(t *testing.T) {
	h := cancelHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	// id=0 is treated as invalid.
	h(w, cancelReq(0, attacker))
	testhelper.AssertContains(t, w.Body.String(), "invalid campaign id")
}
