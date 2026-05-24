package prayers

import (
	"context"
	"errors"
	"testing"

	"bahago/internal/database/db"
	"bahago/internal/game"
)

// ── Stub querier ──────────────────────────────────────────────────────────────

type stubQuerier struct {
	db.Querier
	onDeletePrayer func(ctx context.Context, arg db.DeletePrayerParams) error
}

func (s *stubQuerier) DeletePrayer(ctx context.Context, arg db.DeletePrayerParams) error {
	if s.onDeletePrayer != nil {
		return s.onDeletePrayer(ctx, arg)
	}
	panic("stubQuerier: unexpected call to DeletePrayer")
}

// ── validateCastInput ─────────────────────────────────────────────────────────

func TestValidateCastInput(t *testing.T) {
	base := func() *castInput {
		return &castInput{
			PrayerType:  game.PrayerManaPrayer,
			PrayerTicks: 8,
		}
	}

	tests := []struct {
		name     string
		mutate   func(*castInput)
		wantErrs []error
	}{
		{"valid", func(_ *castInput) {}, nil},
		{"valid_min_duration", func(in *castInput) { in.PrayerTicks = 1 }, nil},
		{"valid_max_duration", func(in *castInput) { in.PrayerTicks = 48 }, nil},
		{"unknown_type", func(in *castInput) { in.PrayerType = "fireball" }, []error{ErrUnknownPrayerType}},
		{"duration_zero", func(in *castInput) { in.PrayerTicks = 0 }, []error{ErrInvalidPrayerDuration}},
		{"duration_negative", func(in *castInput) { in.PrayerTicks = -1 }, []error{ErrInvalidPrayerDuration}},
		{"duration_too_large", func(in *castInput) { in.PrayerTicks = 49 }, []error{ErrInvalidPrayerDuration}},
		{
			name:     "both_invalid",
			mutate:   func(in *castInput) { in.PrayerType = "fireball"; in.PrayerTicks = 0 },
			wantErrs: []error{ErrUnknownPrayerType, ErrInvalidPrayerDuration},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := base()
			tc.mutate(in)
			got := validateCastInput(in)
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

// ── validatePrayerID ──────────────────────────────────────────────────────────

func TestValidatePrayerID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr error
	}{
		{"valid", "42", 42, nil},
		{"zero", "0", 0, ErrInvalidPrayerID},
		{"negative", "-3", 0, ErrInvalidPrayerID},
		{"non_numeric", "abc", 0, ErrInvalidPrayerID},
		{"empty", "", 0, ErrInvalidPrayerID},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validatePrayerID(tc.input)
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

// ── cancelPrayer ──────────────────────────────────────────────────────────────

func TestCancelPrayer_Success(t *testing.T) {
	var seen db.DeletePrayerParams
	q := &stubQuerier{
		onDeletePrayer: func(_ context.Context, arg db.DeletePrayerParams) error {
			seen = arg
			return nil
		},
	}
	h := &handler{queries: q}
	if err := h.cancelPrayer(context.Background(), 1, 42); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if seen.ID != 42 || seen.KingdomID != 1 {
		t.Errorf("params = %+v, want ID=42 KingdomID=1", seen)
	}
}

func TestCancelPrayer_DeleteError(t *testing.T) {
	boom := errors.New("connection refused")
	q := &stubQuerier{
		onDeletePrayer: func(_ context.Context, _ db.DeletePrayerParams) error {
			return boom
		},
	}
	h := &handler{queries: q}
	err := h.cancelPrayer(context.Background(), 1, 42)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want wrapped %v", err, boom)
	}
}
