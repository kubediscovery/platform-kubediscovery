// Package stream contains the client-side stream connection logic for kd-agent:
// exponential-backoff reconnect, AgentHello frame construction, and the
// runtime goroutine triplet (Sender, Ticker, Receiver).
package stream

import "time"

// Retrier implements exponential backoff for bidirectional stream reconnects.
//
// Sequence of delays for the default configuration (base=1s, weight=3, maxRetry=5):
//
//	attempt 0  → 0   (initial connect, no wait)
//	attempt 1  → 1s
//	attempt 2  → 3s
//	attempt 3  → 9s
//	attempt 4  → 27s
//	attempt 5  → 81s
//	attempt 6+ → give up (ShouldRetry returns false)
type Retrier struct {
	// Base is the delay applied before the first retry (attempt 1).
	Base time.Duration
	// Weight is the multiplicative factor applied to each subsequent retry.
	Weight int
	// MaxRetry is the maximum number of retries permitted (excluding the
	// initial attempt).  The agent will give up after MaxRetry+1 total
	// attempts have all failed.
	MaxRetry int
}

// DefaultRetrier matches the kd-agent specification:
//   - base    1 s
//   - weight  3
//   - maximum 5 retries (6 total attempts before fatal)
var DefaultRetrier = Retrier{
	Base:     time.Second,
	Weight:   3,
	MaxRetry: 5,
}

// Delay returns (d, true) where d is the duration the caller should wait
// before making attempt n.  When n exceeds MaxRetry the function returns
// (-1, false) signalling that the caller should give up.
//
// attempt 0 always returns (0, true) — no wait for the very first try.
func (r Retrier) Delay(attempt int) (time.Duration, bool) {
	if attempt > r.MaxRetry {
		return -1, false
	}
	return r.delayFor(attempt), true
}

// ShouldRetry is a convenience predicate that returns true when attempt n
// is still within the allowed retry budget.
func (r Retrier) ShouldRetry(attempt int) bool {
	return attempt <= r.MaxRetry
}

// delayFor computes the raw delay for attempt n without bounds-checking.
func (r Retrier) delayFor(attempt int) time.Duration {
	if attempt == 0 {
		return 0
	}
	d := r.Base
	for i := 1; i < attempt; i++ {
		d *= time.Duration(r.Weight)
	}
	return d
}
