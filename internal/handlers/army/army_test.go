package army

import (
	"context"
	"errors"
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
	"bahago/internal/routes"
	"bahago/internal/testhelper"
)

// ── Stub querier ──────────────────────────────────────────────────────────────

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

var attacker = &db.Kingdom{ID: 1, X: 0, Y: 0, Name: "Attackia"}

// ── validateSendInput ─────────────────────────────────────────────────────────

func TestValidateSendInput(t *testing.T) {
	base := func() *sendInput {
		return &sendInput{
			LegionID:      1,
			Action:        "attack",
			TargetName:    "Other",
			DurationTicks: 3,
		}
	}

	tests := []struct {
		name     string
		mutate   func(*sendInput)
		wantErrs []error
	}{
		{
			name:     "valid_attack",
			mutate:   func(_ *sendInput) {},
			wantErrs: nil,
		},
		{
			name:     "valid_defend_at_max_duration",
			mutate:   func(in *sendInput) { in.Action = "defend"; in.DurationTicks = 24 },
			wantErrs: nil,
		},
		{
			name:     "invalid_legion_id",
			mutate:   func(in *sendInput) { in.LegionID = 0 },
			wantErrs: []error{ErrInvalidLegionID},
		},
		{
			name:     "invalid_action",
			mutate:   func(in *sendInput) { in.Action = "pillage" },
			wantErrs: []error{ErrInvalidAction},
		},
		{
			name:     "attack_duration_above_max",
			mutate:   func(in *sendInput) { in.DurationTicks = 6 },
			wantErrs: []error{ErrInvalidDuration},
		},
		{
			name:     "attack_duration_at_max",
			mutate:   func(in *sendInput) { in.DurationTicks = 5 },
			wantErrs: nil,
		},
		{
			name:     "defend_duration_above_max",
			mutate:   func(in *sendInput) { in.Action = "defend"; in.DurationTicks = 25 },
			wantErrs: []error{ErrInvalidDuration},
		},
		{
			name:     "duration_below_minimum",
			mutate:   func(in *sendInput) { in.DurationTicks = 0 },
			wantErrs: []error{ErrInvalidDuration},
		},
		{
			name:     "empty_target_name",
			mutate:   func(in *sendInput) { in.TargetName = "" },
			wantErrs: []error{ErrTargetRequired},
		},
		{
			name:   "multiple_errors_accumulated",
			mutate: func(in *sendInput) { in.LegionID = 0; in.Action = ""; in.TargetName = "" },
			// Validator surfaces every problem at once so the user sees them all.
			wantErrs: []error{ErrInvalidLegionID, ErrInvalidAction, ErrTargetRequired},
		},
		{
			name: "invalid_action_suppresses_duration_check",
			// Duration would otherwise be flagged against the attack-max of 5, but
			// the action is invalid — adding a duration error would be redundant.
			mutate:   func(in *sendInput) { in.Action = "pillage"; in.DurationTicks = 99 },
			wantErrs: []error{ErrInvalidAction},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := base()
			tc.mutate(in)
			got := validateSendInput(in)

			if len(got) != len(tc.wantErrs) {
				t.Fatalf("validateSendInput returned %d errs (%v), want %d (%v)", len(got), got, len(tc.wantErrs), tc.wantErrs)
			}
			for i, want := range tc.wantErrs {
				if !errors.Is(got[i], want) {
					t.Errorf("errs[%d] = %v, want %v", i, got[i], want)
				}
			}
		})
	}
}

// ── validateCancelID ──────────────────────────────────────────────────────────

func TestValidateCancelID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr error
	}{
		{"valid", "42", 42, nil},
		{"zero", "0", 0, ErrInvalidCampaignID},
		{"negative", "-3", 0, ErrInvalidCampaignID},
		{"non_numeric", "abc", 0, ErrInvalidCampaignID},
		{"empty", "", 0, ErrInvalidCampaignID},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateCancelID(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

// ── sendCampaign (pre-tx paths only) ──────────────────────────────────────────
//
// sendCampaign reaches a real *pgxpool.Pool via BeginTx after the target lookup
// and self-target check. The cases below cover what's reachable with a nil pool
// — i.e. every branch up to (but not including) the transaction. Tx-level
// branches (ErrNoRows from the CTE, serialization failure) live in the
// internal/database/db integration tests.

func TestSendCampaign_TargetNotFound(t *testing.T) {
	q := &stubQuerier{
		onGetKingdomByName: func(_ context.Context, _ string) (db.Kingdom, error) {
			return db.Kingdom{}, pgx.ErrNoRows
		},
	}
	h := &handler{queries: q}
	err := h.sendCampaign(context.Background(), attacker, &sendInput{TargetName: "Atlantis"})
	if !errors.Is(err, ErrTargetNotFound) {
		t.Fatalf("err = %v, want ErrTargetNotFound", err)
	}
}

func TestSendCampaign_TargetLookupOtherError(t *testing.T) {
	boom := errors.New("connection refused")
	q := &stubQuerier{
		onGetKingdomByName: func(_ context.Context, _ string) (db.Kingdom, error) {
			return db.Kingdom{}, boom
		},
	}
	h := &handler{queries: q}
	err := h.sendCampaign(context.Background(), attacker, &sendInput{TargetName: "Atlantis"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrapped %v", err, boom)
	}
	if isSendUserError(err) {
		t.Errorf("unexpected user-error classification for wrapped infra error: %v", err)
	}
}

func TestSendCampaign_SelfTarget(t *testing.T) {
	q := &stubQuerier{
		onGetKingdomByName: func(_ context.Context, _ string) (db.Kingdom, error) {
			return db.Kingdom{ID: attacker.ID, Name: attacker.Name}, nil
		},
	}
	h := &handler{queries: q}
	err := h.sendCampaign(context.Background(), attacker, &sendInput{TargetName: attacker.Name})
	if !errors.Is(err, ErrSelfTarget) {
		t.Fatalf("err = %v, want ErrSelfTarget", err)
	}
}

// ── cancelCampaign ────────────────────────────────────────────────────────────

func TestCancelCampaign_Success(t *testing.T) {
	var seen db.CancelCampaignParams
	q := &stubQuerier{
		onCancelCampaign: func(_ context.Context, arg db.CancelCampaignParams) (int, error) {
			seen = arg
			return 1, nil
		},
	}
	h := &handler{queries: q}
	if err := h.cancelCampaign(context.Background(), attacker.ID, 42); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if seen.ID != 42 || seen.KingdomID != attacker.ID {
		t.Errorf("params = %+v, want ID=42 KingdomID=%d", seen, attacker.ID)
	}
}

func TestCancelCampaign_NotFound(t *testing.T) {
	q := &stubQuerier{
		onCancelCampaign: func(_ context.Context, _ db.CancelCampaignParams) (int, error) {
			return 0, pgx.ErrNoRows
		},
	}
	h := &handler{queries: q}
	err := h.cancelCampaign(context.Background(), attacker.ID, 42)
	if !errors.Is(err, ErrCampaignNotFound) {
		t.Fatalf("err = %v, want ErrCampaignNotFound", err)
	}
}

func TestCancelCampaign_OtherError(t *testing.T) {
	boom := fmt.Errorf("connection refused")
	q := &stubQuerier{
		onCancelCampaign: func(_ context.Context, _ db.CancelCampaignParams) (int, error) {
			return 0, boom
		},
	}
	h := &handler{queries: q}
	err := h.cancelCampaign(context.Background(), attacker.ID, 42)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrapped %v", err, boom)
	}
	if isCancelUserError(err) {
		t.Errorf("unexpected user-error classification for wrapped infra error: %v", err)
	}
}

// ── handler shell smoke tests ─────────────────────────────────────────────────
//
// These exercise the HTTP/SSE wiring of RegisterRoutes' handlers and assert
// the user-visible alert text. Fine-grained coverage of validator/orchestrator
// branches lives in the unit tests above.

func sendHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	RegisterRoutes(cr, q, nil, nil)
	return cr.Handlers["POST "+routes.KingdomArmySendPath]
}

func sendReq(body string, kingdom *db.Kingdom) *http.Request {
	r := httptest.NewRequest("POST", routes.KingdomArmySendPath, strings.NewReader(body))
	return r.WithContext(context.WithValue(r.Context(), contextkeys.Kingdom, kingdom))
}

// TestHandleSend_ValidatorErrorRenders proves the validator integrates with the
// handler — an invalid legion id surfaces as alert text.
func TestHandleSend_ValidatorErrorRenders(t *testing.T) {
	h := sendHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, sendReq(`{"send_legion":0,"send_action":"attack","send_target":"Other","send_ticks":3}`, attacker))
	testhelper.AssertContains(t, w.Body.String(), "invalid legion id")
}

// TestHandleSend_TargetNotFoundRenders proves an orchestrator user-error
// (pgx.ErrNoRows from GetKingdomByName) surfaces as alert text via
// isSendUserError.
func TestHandleSend_TargetNotFoundRenders(t *testing.T) {
	stub := &stubQuerier{
		onGetKingdomByName: func(_ context.Context, _ string) (db.Kingdom, error) {
			return db.Kingdom{}, pgx.ErrNoRows
		},
	}
	h := sendHandler(stub)
	w := httptest.NewRecorder()
	h(w, sendReq(`{"send_legion":1,"send_action":"attack","send_target":"Atlantis","send_ticks":3}`, attacker))
	testhelper.AssertContains(t, w.Body.String(), "target kingdom not found")
}

// TestHandleSend_CannotTargetOwnKingdomRenders proves ErrSelfTarget reaches
// the alert.
func TestHandleSend_CannotTargetOwnKingdomRenders(t *testing.T) {
	stub := &stubQuerier{
		onGetKingdomByName: func(_ context.Context, _ string) (db.Kingdom, error) {
			return db.Kingdom{ID: attacker.ID, Name: attacker.Name}, nil
		},
	}
	h := sendHandler(stub)
	w := httptest.NewRecorder()
	h(w, sendReq(`{"send_legion":1,"send_action":"attack","send_target":"Attackia","send_ticks":3}`, attacker))
	testhelper.AssertContains(t, w.Body.String(), "cannot target your own kingdom")
}

func cancelHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	RegisterRoutes(cr, q, nil, nil)
	return cr.Handlers["POST "+routes.KingdomArmyCancelPath]
}

func cancelReq(id int, kingdom *db.Kingdom) *http.Request {
	url := strings.ReplaceAll(routes.KingdomArmyCancelPath, "{id}", strconv.Itoa(id))
	r := httptest.NewRequest("POST", url, nil)
	r.SetPathValue("id", strconv.Itoa(id))
	return r.WithContext(context.WithValue(r.Context(), contextkeys.Kingdom, kingdom))
}

func TestHandleCancel_InvalidInputRenders(t *testing.T) {
	h := cancelHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, cancelReq(0, attacker))
	testhelper.AssertContains(t, w.Body.String(), "invalid campaign id")
}

func TestHandleCancel_CampaignNotFoundRenders(t *testing.T) {
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

// ── validateTransferInput ─────────────────────────────────────────────────────

func TestValidateTransferInput(t *testing.T) {
	base := func() *transferInput {
		return &transferInput{
			FromID:   0,
			ToID:     1,
			UnitType: "recruit",
			Count:    1,
		}
	}

	tests := []struct {
		name     string
		mutate   func(*transferInput)
		wantErrs []error
	}{
		{
			name:     "valid_reserve_to_legion",
			mutate:   func(_ *transferInput) {},
			wantErrs: nil,
		},
		{
			name:     "valid_legion_to_reserve",
			mutate:   func(in *transferInput) { in.FromID = 1; in.ToID = 0 },
			wantErrs: nil,
		},
		{
			name:     "valid_count_at_max",
			mutate:   func(in *transferInput) { in.Count = game.MaxUnitInput },
			wantErrs: nil,
		},
		{
			name:     "zero_count",
			mutate:   func(in *transferInput) { in.Count = 0 },
			wantErrs: []error{ErrInvalidCount},
		},
		{
			name:     "negative_count",
			mutate:   func(in *transferInput) { in.Count = -1 },
			wantErrs: []error{ErrInvalidCount},
		},
		{
			name:     "count_too_large",
			mutate:   func(in *transferInput) { in.Count = game.MaxUnitInput + 1 },
			wantErrs: []error{ErrCountTooLarge},
		},
		{
			name:     "same_legion_source_and_destination",
			mutate:   func(in *transferInput) { in.FromID = 2; in.ToID = 2 },
			wantErrs: []error{ErrSameSourceAndDestination},
		},
		{
			name:     "both_reserve",
			mutate:   func(in *transferInput) { in.FromID = 0; in.ToID = 0 },
			wantErrs: []error{ErrSameSourceAndDestination},
		},
		{
			name:     "unknown_unit_type",
			mutate:   func(in *transferInput) { in.UnitType = "dragon" },
			wantErrs: []error{ErrUnknownUnitType},
		},
		{
			name:   "multiple_errors_accumulated",
			mutate: func(in *transferInput) { in.Count = 0; in.FromID = 1; in.ToID = 1; in.UnitType = "" },
			// All three checks run independently — user sees every problem at once.
			wantErrs: []error{ErrInvalidCount, ErrSameSourceAndDestination, ErrUnknownUnitType},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := base()
			tc.mutate(in)
			got := validateTransferInput(in)

			if len(got) != len(tc.wantErrs) {
				t.Fatalf("validateTransferInput returned %d errs (%v), want %d (%v)", len(got), got, len(tc.wantErrs), tc.wantErrs)
			}
			for i, want := range tc.wantErrs {
				if !errors.Is(got[i], want) {
					t.Errorf("errs[%d] = %v, want %v", i, got[i], want)
				}
			}
		})
	}
}

// ── handler shell smoke tests (transfer / disband) ────────────────────────────
//
// transferUnits and disbandLegion reach pool.BeginTx immediately after
// validation, so only the validator path is reachable with a nil pool. Tx-level
// paths (ErrInsufficientUnitsInSource, ErrLegionCapReached, ErrTransferConflict)
// belong in internal/database/db integration tests.

func transferHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	RegisterRoutes(cr, q, nil, nil)
	return cr.Handlers["POST "+routes.KingdomArmyTransferPath]
}

func transferReq(body string, kingdom *db.Kingdom) *http.Request {
	r := httptest.NewRequest("POST", routes.KingdomArmyTransferPath, strings.NewReader(body))
	return r.WithContext(context.WithValue(r.Context(), contextkeys.Kingdom, kingdom))
}

// TestHandleTransfer_ValidatorErrorRenders proves the validator integrates with
// the handler — a zero count surfaces as alert text.
func TestHandleTransfer_ValidatorErrorRenders(t *testing.T) {
	h := transferHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, transferReq(`{"xfer_from":0,"xfer_to":1,"xfer_unit":"recruit","xfer_count":0}`, attacker))
	testhelper.AssertContains(t, w.Body.String(), "count must be at least 1")
}

func disbandHandler(q db.Querier) http.HandlerFunc {
	cr := testhelper.NewCaptureRouter()
	RegisterRoutes(cr, q, nil, nil)
	return cr.Handlers["POST "+routes.KingdomArmyDisbandPath]
}

func disbandReq(id int, kingdom *db.Kingdom) *http.Request {
	url := strings.ReplaceAll(routes.KingdomArmyDisbandPath, "{id}", strconv.Itoa(id))
	r := httptest.NewRequest("POST", url, nil)
	r.SetPathValue("id", strconv.Itoa(id))
	return r.WithContext(context.WithValue(r.Context(), contextkeys.Kingdom, kingdom))
}

func TestHandleDisband_InvalidInputRenders(t *testing.T) {
	h := disbandHandler(&stubQuerier{})
	w := httptest.NewRecorder()
	h(w, disbandReq(0, attacker))
	testhelper.AssertContains(t, w.Body.String(), "invalid legion id")
}
