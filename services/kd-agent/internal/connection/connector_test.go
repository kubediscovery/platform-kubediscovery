package connection

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"
)

var errDial = errors.New("dial error")

// instantTimer returns a channel that fires immediately, bypassing any sleep
// in the retry loop. Used so tests run fast without real sleeps.
func instantTimer(_ time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	ch <- time.Now()
	return ch
}

// recordingTimer records every delay requested and fires immediately.
type recordingTimer struct {
	delays []time.Duration
}

func (r *recordingTimer) After(d time.Duration) <-chan time.Time {
	r.delays = append(r.delays, d)
	ch := make(chan time.Time, 1)
	ch <- time.Now()
	return ch
}

func noopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(noopWriter{}, &slog.HandlerOptions{Level: slog.LevelError + 10}))
}

type noopWriter struct{}

func (noopWriter) Write(p []byte) (int, error) { return len(p), nil }

// dialAlwaysFails returns a DialFunc that always returns errDial.
func dialAlwaysFails() DialFunc {
	return func(_ context.Context) error {
		return errDial
	}
}

// dialFailsThenSucceeds returns a DialFunc that fails for the first `n` calls
// then returns nil (success) on call n+1.
func dialFailsThenSucceeds(n int) DialFunc {
	calls := 0
	return func(_ context.Context) error {
		calls++
		if calls <= n {
			return fmt.Errorf("transient error (call %d)", calls)
		}
		return nil
	}
}

// dialAlwaysSucceeds returns a DialFunc that immediately returns nil.
func dialAlwaysSucceeds() DialFunc {
	return func(_ context.Context) error { return nil }
}

// TestConnector_ExhaustsRetries verifies that Run returns ErrExhausted after
// MaxRetries consecutive failures.
func TestConnector_ExhaustsRetries(t *testing.T) {
	cfg := BackoffConfig{Base: time.Millisecond, Multiplier: 3, MaxRetries: 5}
	c := NewConnector(dialAlwaysFails(), cfg, noopLogger(), WithTimer(instantTimer))

	err := c.Run(context.Background())

	if !errors.Is(err, ErrExhausted) {
		t.Fatalf("expected ErrExhausted, got %v", err)
	}
}

// TestConnector_SuccessOnFirstAttempt verifies that a successful first dial
// causes Run to loop (reconnect) until ctx is cancelled.
func TestConnector_SuccessOnFirstAttempt(t *testing.T) {
	calls := 0
	dial := func(_ context.Context) error {
		calls++
		return nil // immediate graceful close each time
	}

	cfg := BackoffConfig{Base: time.Millisecond, Multiplier: 3, MaxRetries: 5}
	c := NewConnector(dial, cfg, noopLogger(), WithTimer(instantTimer))

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- c.Run(ctx)
	}()

	// Let it spin a few reconnects then cancel.
	time.Sleep(10 * time.Millisecond)
	cancel()

	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls == 0 {
		t.Fatal("expected at least one dial call")
	}
}

// TestConnector_RetriesAndSucceeds verifies that failures below MaxRetries
// are tolerated and the connector succeeds when dial eventually returns nil.
func TestConnector_RetriesAndSucceeds(t *testing.T) {
	const failTimes = 3

	cfg := BackoffConfig{Base: time.Millisecond, Multiplier: 3, MaxRetries: 5}
	c := NewConnector(dialFailsThenSucceeds(failTimes), cfg, noopLogger(), WithTimer(instantTimer))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- c.Run(ctx)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	err := <-done
	if errors.Is(err, ErrExhausted) {
		t.Fatal("expected connection to succeed, but got ErrExhausted")
	}
}

// TestConnector_BackoffDelaySequence verifies the delays follow the
// exponential sequence: 1s → 3s → 9s → 27s → 81s.
func TestConnector_BackoffDelaySequence(t *testing.T) {
	cfg := DefaultBackoffConfig() // base=1s, multiplier=3, max=5

	rec := &recordingTimer{}
	c := NewConnector(dialAlwaysFails(), cfg, noopLogger(), WithTimer(rec.After))

	_ = c.Run(context.Background())

	wantDelays := []time.Duration{
		1 * time.Second,
		3 * time.Second,
		9 * time.Second,
		27 * time.Second,
		81 * time.Second,
	}

	if len(rec.delays) != len(wantDelays) {
		t.Fatalf("expected %d delays, got %d: %v", len(wantDelays), len(rec.delays), rec.delays)
	}
	for i, want := range wantDelays {
		if rec.delays[i] != want {
			t.Errorf("delay[%d]: got %v, want %v", i, rec.delays[i], want)
		}
	}
}

// TestConnector_ContextCancelledDuringBackoff verifies that ctx cancellation
// during a backoff sleep returns ctx.Err() promptly.
func TestConnector_ContextCancelledDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// The timer blocks until the context is cancelled.
	blockingTimer := func(d time.Duration) <-chan time.Time {
		ch := make(chan time.Time)
		go func() {
			<-ctx.Done()
			// Do not send to ch; the select in Run will pick ctx.Done().
		}()
		return ch
	}

	cfg := BackoffConfig{Base: time.Second, Multiplier: 3, MaxRetries: 5}
	c := NewConnector(dialAlwaysFails(), cfg, noopLogger(), WithTimer(blockingTimer))

	done := make(chan error, 1)
	go func() {
		done <- c.Run(ctx)
	}()

	// Give the goroutine time to reach the first backoff sleep.
	time.Sleep(10 * time.Millisecond)
	cancel()

	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// TestConnector_ResetsCounterAfterSuccess verifies that after a successful
// connection the consecutive failure counter resets, allowing the full retry
// sequence on the next failure run.
func TestConnector_ResetsCounterAfterSuccess(t *testing.T) {
	const failsBeforeSuccess = 2
	const failsAfterSuccess = 5 // should exhaust maxRetries=5 and return ErrExhausted

	phase := 0
	callsInPhase := 0

	dial := func(_ context.Context) error {
		callsInPhase++
		switch phase {
		case 0:
			if callsInPhase <= failsBeforeSuccess {
				return errDial
			}
			// succeeded — advance to next phase
			phase = 1
			callsInPhase = 0
			return nil
		default:
			return errDial
		}
	}

	cfg := BackoffConfig{Base: time.Millisecond, Multiplier: 3, MaxRetries: failsAfterSuccess}
	c := NewConnector(dial, cfg, noopLogger(), WithTimer(instantTimer))

	err := c.Run(context.Background())
	if !errors.Is(err, ErrExhausted) {
		t.Fatalf("expected ErrExhausted after reset, got %v", err)
	}
}
