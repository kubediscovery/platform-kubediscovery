package connection

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// ErrExhausted is returned by Connector.Run when all retry attempts are
// exhausted without a successful connection.
var ErrExhausted = errors.New("connection attempts exhausted")

// DialFunc is the function invoked by the Connector to establish and maintain
// the gRPC stream. It blocks until the stream is closed.
//
//   - A nil return means the stream ended gracefully; the Connector resets its
//     retry counter and immediately attempts to reconnect.
//   - A non-nil return is treated as a connection failure and triggers the
//     exponential backoff retry sequence.
//
// Implementations should respect ctx cancellation: when ctx is done, they
// should return ctx.Err() promptly.
type DialFunc func(ctx context.Context) error

// Connector manages the persistent gRPC connection loop for kd-agent.
// It calls DialFunc, applies exponential backoff on failures, and terminates
// with ErrExhausted after MaxRetries consecutive failures.
//
// A successful connection (DialFunc returning nil) resets the consecutive
// failure counter, so the full retry sequence is available after every
// healthy stream session.
type Connector struct {
	dial    DialFunc
	backoff BackoffConfig
	log     *slog.Logger

	// timer is injectable so tests can control time without sleeping.
	// Defaults to time.After.
	timer func(time.Duration) <-chan time.Time
}

// ConnectorOption configures a Connector.
type ConnectorOption func(*Connector)

// WithTimer replaces the default time.After implementation used for backoff
// sleeps. Primarily useful in tests.
func WithTimer(fn func(time.Duration) <-chan time.Time) ConnectorOption {
	return func(c *Connector) {
		c.timer = fn
	}
}

// NewConnector constructs a Connector with the provided DialFunc and backoff
// configuration. Additional options (e.g. WithTimer) may be supplied.
func NewConnector(dial DialFunc, cfg BackoffConfig, log *slog.Logger, opts ...ConnectorOption) *Connector {
	c := &Connector{
		dial:    dial,
		backoff: cfg,
		log:     log,
		timer:   time.After,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Run executes the connection loop. It keeps calling dial in a tight loop,
// applying exponential backoff after each consecutive failure.
//
// The loop terminates when:
//   - ctx is cancelled → returns ctx.Err()
//   - MaxRetries consecutive failures occur → returns ErrExhausted (wrapped
//     with the last error from dial)
//
// A successful dial call (nil return) resets the consecutive failure counter
// so that the next failure sequence starts fresh from the base delay.
func (c *Connector) Run(ctx context.Context) error {
	consecutiveFailures := 0

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		c.log.Info("connecting to gateway",
			slog.Int("attempt", consecutiveFailures+1),
		)

		err := c.dial(ctx)
		if err == nil {
			c.log.Info("stream closed gracefully, reconnecting")
			consecutiveFailures = 0
			continue
		}

		// ctx cancellation propagated through dial — treat as clean shutdown.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}

		consecutiveFailures++

		if consecutiveFailures > c.backoff.MaxRetries {
			c.log.Error("connection attempts exhausted, terminating",
				slog.Int("max_retries", c.backoff.MaxRetries),
				slog.Any("last_error", err),
			)
			return fmt.Errorf("%w after %d retries: %v", ErrExhausted, c.backoff.MaxRetries, err)
		}

		delay := c.backoff.DelayFor(consecutiveFailures - 1)
		c.log.Warn("connection failed, retrying with backoff",
			slog.Int("consecutive_failures", consecutiveFailures),
			slog.Int("max_retries", c.backoff.MaxRetries),
			slog.Duration("delay", delay),
			slog.Any("error", err),
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.timer(delay):
		}
	}
}
