package game

import (
	"testing"
	"time"
)

func TestNextAlignedBoundary(t *testing.T) {
	t.Parallel()

	loc := time.UTC

	cases := []struct {
		name     string
		now      time.Time
		interval time.Duration
		want     time.Time
	}{
		{
			name:     "mid-hour with 1h interval",
			now:      time.Date(2026, 1, 1, 9, 43, 0, 0, loc),
			interval: time.Hour,
			want:     time.Date(2026, 1, 1, 10, 0, 0, 0, loc),
		},
		{
			name:     "exactly on boundary with 1h interval",
			now:      time.Date(2026, 1, 1, 10, 0, 0, 0, loc),
			interval: time.Hour,
			want:     time.Date(2026, 1, 1, 11, 0, 0, 0, loc),
		},
		{
			name:     "sub-second past boundary with 1h interval",
			now:      time.Date(2026, 1, 1, 10, 0, 0, 1, loc),
			interval: time.Hour,
			want:     time.Date(2026, 1, 1, 11, 0, 0, 0, loc),
		},
		{
			name:     "9:43 with 30m interval",
			now:      time.Date(2026, 1, 1, 9, 43, 0, 0, loc),
			interval: 30 * time.Minute,
			want:     time.Date(2026, 1, 1, 10, 0, 0, 0, loc),
		},
		{
			name:     "9:15 with 30m interval",
			now:      time.Date(2026, 1, 1, 9, 15, 0, 0, loc),
			interval: 30 * time.Minute,
			want:     time.Date(2026, 1, 1, 9, 30, 0, 0, loc),
		},
		{
			name:     "9:47 with 15m interval",
			now:      time.Date(2026, 1, 1, 9, 47, 0, 0, loc),
			interval: 15 * time.Minute,
			want:     time.Date(2026, 1, 1, 10, 0, 0, 0, loc),
		},
		{
			name:     "midnight rollover",
			now:      time.Date(2026, 1, 1, 23, 30, 0, 0, loc),
			interval: time.Hour,
			want:     time.Date(2026, 1, 2, 0, 0, 0, 0, loc),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := alignedTick(tc.now, tc.interval)
			if !got.Equal(tc.want) {
				t.Errorf("nextAlignedBoundary(%v, %v) = %v, want %v", tc.now, tc.interval, got, tc.want)
			}
		})
	}
}
