package allocation

import (
	"context"
	"errors"
	"testing"

	"bahago/internal/database/db"
)

// ── Stub querier ──────────────────────────────────────────────────────────────

type stubQuerier struct {
	db.Querier
	onUpdateKingdomAllocations func(ctx context.Context, arg db.UpdateKingdomAllocationsParams) (db.Kingdom, error)
}

func (s *stubQuerier) UpdateKingdomAllocations(ctx context.Context, arg db.UpdateKingdomAllocationsParams) (db.Kingdom, error) {
	if s.onUpdateKingdomAllocations != nil {
		return s.onUpdateKingdomAllocations(ctx, arg)
	}
	panic("stubQuerier: unexpected call to UpdateKingdomAllocations")
}

// ── validateAllocationInput ───────────────────────────────────────────────────

func TestValidateAllocationInput(t *testing.T) {
	tests := []struct {
		name     string
		input    *allocationSignals
		wantErrs []error
	}{
		{
			name:  "valid_sum_under_100",
			input: &allocationSignals{WoodPct: 20, StonePct: 20, FoodPct: 20, ManaPct: 10, DevotionPct: 10, KnowledgePct: 10}, // 90, leaves 10 idle
		},
		{
			name:  "valid_sum_exactly_100",
			input: &allocationSignals{WoodPct: 20, StonePct: 20, FoodPct: 20, ManaPct: 20, DevotionPct: 10, KnowledgePct: 10},
		},
		{
			name:  "valid_all_zero",
			input: &allocationSignals{},
		},
		{
			name:     "negative_value",
			input:    &allocationSignals{WoodPct: -1},
			wantErrs: []error{ErrPercentageOutOfRange},
		},
		{
			name:     "value_over_100",
			input:    &allocationSignals{WoodPct: 101},
			wantErrs: []error{ErrPercentageOutOfRange, ErrAllocationExceeds100},
		},
		{
			name:     "sum_over_100",
			input:    &allocationSignals{WoodPct: 50, StonePct: 51},
			wantErrs: []error{ErrAllocationExceeds100},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validateAllocationInput(tc.input)
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

// ── updateAllocations ─────────────────────────────────────────────────────────

func TestUpdateAllocations_SetsIdlePctCorrectly(t *testing.T) {
	var seen db.UpdateKingdomAllocationsParams
	q := &stubQuerier{
		onUpdateKingdomAllocations: func(_ context.Context, arg db.UpdateKingdomAllocationsParams) (db.Kingdom, error) {
			seen = arg
			return db.Kingdom{ID: 1, UserID: arg.UserID}, nil
		},
	}
	h := &handler{queries: q}
	in := &allocationSignals{WoodPct: 30, StonePct: 20, FoodPct: 20, ManaPct: 10, DevotionPct: 5, KnowledgePct: 5}
	if _, err := h.updateAllocations(context.Background(), 42, in); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if seen.IdlePct != 10 {
		t.Errorf("IdlePct = %d, want 10 (100 - 90)", seen.IdlePct)
	}
	if seen.UserID != 42 {
		t.Errorf("UserID = %d, want 42", seen.UserID)
	}
}

func TestUpdateAllocations_DBError(t *testing.T) {
	boom := errors.New("connection refused")
	q := &stubQuerier{
		onUpdateKingdomAllocations: func(_ context.Context, _ db.UpdateKingdomAllocationsParams) (db.Kingdom, error) {
			return db.Kingdom{}, boom
		},
	}
	h := &handler{queries: q}
	_, err := h.updateAllocations(context.Background(), 42, &allocationSignals{})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrapped %v", err, boom)
	}
}
