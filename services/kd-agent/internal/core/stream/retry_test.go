package stream_test

import (
	"testing"
	"time"

	"github.com/kubediscovery/kd-agent/internal/core/stream"
)

// ── DefaultRetrier values ────────────────────────────────────────────────────

func TestDefaultRetrier_MatchesSpec(t *testing.T) {
	r := stream.DefaultRetrier
	if r.Base != time.Second {
		t.Errorf("Base = %v, want 1s", r.Base)
	}
	if r.Weight != 3 {
		t.Errorf("Weight = %d, want 3", r.Weight)
	}
	if r.MaxRetry != 5 {
		t.Errorf("MaxRetry = %d, want 5", r.MaxRetry)
	}
}

// ── Delay returns (0, true) for the initial attempt ──────────────────────────

func TestRetrier_Delay_AttemptZeroNoWait(t *testing.T) {
	r := stream.DefaultRetrier
	d, ok := r.Delay(0)
	if !ok {
		t.Fatal("Delay(0) should return ok=true")
	}
	if d != 0 {
		t.Errorf("Delay(0) = %v, want 0", d)
	}
}

// ── Exact delay sequence mandated by spec ────────────────────────────────────

func TestRetrier_DelaySequence(t *testing.T) {
	r := stream.DefaultRetrier
	want := []time.Duration{
		0,           // attempt 0 — initial connect
		1 * time.Second,  // attempt 1
		3 * time.Second,  // attempt 2
		9 * time.Second,  // attempt 3
		27 * time.Second, // attempt 4
		81 * time.Second, // attempt 5
	}

	for attempt, wantDelay := range want {
		d, ok := r.Delay(attempt)
		if !ok {
			t.Errorf("Delay(%d): got ok=false, want true", attempt)
			continue
		}
		if d != wantDelay {
			t.Errorf("Delay(%d) = %v, want %v", attempt, d, wantDelay)
		}
	}
}

// ── Give-up when attempt exceeds MaxRetry ────────────────────────────────────

func TestRetrier_Delay_BeyondMaxRetryGivesUp(t *testing.T) {
	r := stream.DefaultRetrier
	// attempt 6 is one beyond the limit
	_, ok := r.Delay(6)
	if ok {
		t.Error("Delay(MaxRetry+1) should return ok=false")
	}
}

func TestRetrier_Delay_BeyondMaxRetryReturnsNegative(t *testing.T) {
	r := stream.DefaultRetrier
	d, _ := r.Delay(6)
	if d >= 0 {
		t.Errorf("Delay beyond MaxRetry should be negative sentinel, got %v", d)
	}
}

// ── ShouldRetry predicate ────────────────────────────────────────────────────

func TestRetrier_ShouldRetry_WithinBudget(t *testing.T) {
	r := stream.DefaultRetrier
	for attempt := 0; attempt <= r.MaxRetry; attempt++ {
		if !r.ShouldRetry(attempt) {
			t.Errorf("ShouldRetry(%d) = false, want true", attempt)
		}
	}
}

func TestRetrier_ShouldRetry_ExceededBudget(t *testing.T) {
	r := stream.DefaultRetrier
	if r.ShouldRetry(r.MaxRetry + 1) {
		t.Errorf("ShouldRetry(%d) = true, want false", r.MaxRetry+1)
	}
}

// ── Custom retrier configuration ─────────────────────────────────────────────

func TestRetrier_CustomParams(t *testing.T) {
	r := stream.Retrier{Base: 500 * time.Millisecond, Weight: 2, MaxRetry: 3}

	cases := []struct {
		attempt int
		want    time.Duration
		wantOK  bool
	}{
		{0, 0, true},
		{1, 500 * time.Millisecond, true},
		{2, time.Second, true},
		{3, 2 * time.Second, true},
		{4, -1, false},
	}

	for _, tc := range cases {
		d, ok := r.Delay(tc.attempt)
		if ok != tc.wantOK {
			t.Errorf("Delay(%d) ok = %v, want %v", tc.attempt, ok, tc.wantOK)
		}
		if tc.wantOK && d != tc.want {
			t.Errorf("Delay(%d) = %v, want %v", tc.attempt, d, tc.want)
		}
		if !tc.wantOK && d >= 0 {
			t.Errorf("Delay(%d) beyond limit should be negative, got %v", tc.attempt, d)
		}
	}
}

// ── MaxRetry=0 means no retries (only initial attempt allowed) ───────────────

func TestRetrier_MaxRetryZero_OnlyInitialAttempt(t *testing.T) {
	r := stream.Retrier{Base: time.Second, Weight: 3, MaxRetry: 0}

	d, ok := r.Delay(0)
	if !ok || d != 0 {
		t.Errorf("Delay(0) = (%v, %v), want (0, true)", d, ok)
	}

	_, ok = r.Delay(1)
	if ok {
		t.Error("Delay(1) should give up when MaxRetry=0")
	}
}
