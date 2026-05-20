// Package connection provides the persistent gRPC connection loop for kd-agent,
// including exponential backoff logic for reconnection.
//
// Backoff schedule (default): base 1s, multiplier 3, max 5 retries
//
//	Retry 1 → wait 1s
//	Retry 2 → wait 3s
//	Retry 3 → wait 9s
//	Retry 4 → wait 27s
//	Retry 5 → wait 81s
//	→ fatal if all retries exhausted
package connection

import "time"

const (
	// DefaultBase is the initial backoff delay.
	DefaultBase = 1 * time.Second

	// DefaultMultiplier is the factor applied to the previous delay.
	DefaultMultiplier = 3

	// DefaultMaxRetries is the maximum number of consecutive connection
	// retry attempts before the agent terminates with a fatal error.
	// Delays: 1s → 3s → 9s → 27s → 81s → fatal
	DefaultMaxRetries = 5
)

// BackoffConfig defines exponential backoff parameters for connection retries.
type BackoffConfig struct {
	// Base is the delay before the first retry.
	Base time.Duration

	// Multiplier is the factor by which the delay grows on each successive retry.
	Multiplier int

	// MaxRetries is the number of consecutive failures allowed before the
	// connector returns a fatal error. A successful connection resets the counter.
	MaxRetries int
}

// DefaultBackoffConfig returns the standard kd-agent backoff configuration:
// base 1s, multiplier 3, max retries 5 → delays: 1s, 3s, 9s, 27s, 81s.
func DefaultBackoffConfig() BackoffConfig {
	return BackoffConfig{
		Base:       DefaultBase,
		Multiplier: DefaultMultiplier,
		MaxRetries: DefaultMaxRetries,
	}
}

// DelayFor returns the wait duration before retry number n (0-indexed).
// For n=0 returns Base, for n=1 returns Base*Multiplier, and so on.
func (b BackoffConfig) DelayFor(n int) time.Duration {
	d := b.Base
	for i := 0; i < n; i++ {
		d *= time.Duration(b.Multiplier)
	}
	return d
}
