package ui

import "testing"

func TestSymbolSize(t *testing.T) {
	cases := []struct {
		id         string
		w, h       float64
		sizePx     int
		wantHeight int
	}{
		{"crown", 24, 24, 24, 24},        // square
		{"res-wood", 24, 24, 24, 24},     // square
		{"sandglass", 24, 24, 24, 24},    // square (redrawn from 10×14)
		{"idle", 24, 24, 32, 32},         // square (renamed from zzz)
	}
	for _, c := range cases {
		vb, ok := symbolSize[c.id]
		if !ok {
			t.Fatalf("missing %s", c.id)
		}
		if vb.w != c.w || vb.h != c.h {
			t.Errorf("%s viewBox = %v, want %v %v", c.id, vb, c.w, c.h)
		}
		if got := iconHeight(c.id, c.sizePx); got != c.wantHeight {
			t.Errorf("%s height@%d = %d, want %d", c.id, c.sizePx, got, c.wantHeight)
		}
	}
	// The slim sprite retires the shield frame and the 20×23 heraldic space.
	if _, ok := symbolSize["shield-frame"]; ok {
		t.Error("stale shield-frame symbol still present in sprite")
	}
	if _, ok := symbolSize["zzz"]; ok {
		t.Error("stale zzz symbol still present in sprite (renamed to idle)")
	}
}
