package connection

import (
	"testing"
	"time"
)

func TestDefaultBackoffConfig(t *testing.T) {
	cfg := DefaultBackoffConfig()

	if cfg.Base != DefaultBase {
		t.Errorf("Base: got %v, want %v", cfg.Base, DefaultBase)
	}
	if cfg.Multiplier != DefaultMultiplier {
		t.Errorf("Multiplier: got %d, want %d", cfg.Multiplier, DefaultMultiplier)
	}
	if cfg.MaxRetries != DefaultMaxRetries {
		t.Errorf("MaxRetries: got %d, want %d", cfg.MaxRetries, DefaultMaxRetries)
	}
}

func TestBackoffConfig_DelayFor(t *testing.T) {
	cfg := DefaultBackoffConfig() // base=1s, multiplier=3, max=5

	// Expected delays: 1s → 3s → 9s → 27s → 81s
	want := []time.Duration{
		1 * time.Second,
		3 * time.Second,
		9 * time.Second,
		27 * time.Second,
		81 * time.Second,
	}

	for i, w := range want {
		got := cfg.DelayFor(i)
		if got != w {
			t.Errorf("DelayFor(%d): got %v, want %v", i, got, w)
		}
	}
}

func TestBackoffConfig_DelayFor_CustomConfig(t *testing.T) {
	cfg := BackoffConfig{
		Base:       500 * time.Millisecond,
		Multiplier: 2,
		MaxRetries: 3,
	}

	// 500ms, 1s, 2s
	cases := []struct {
		n    int
		want time.Duration
	}{
		{0, 500 * time.Millisecond},
		{1, 1 * time.Second},
		{2, 2 * time.Second},
	}

	for _, tc := range cases {
		got := cfg.DelayFor(tc.n)
		if got != tc.want {
			t.Errorf("DelayFor(%d): got %v, want %v", tc.n, got, tc.want)
		}
	}
}
