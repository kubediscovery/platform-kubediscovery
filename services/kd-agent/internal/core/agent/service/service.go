// Package service implements the kd-agent connection lifecycle: retry loop,
// AgentHello sending, heartbeat, and command dispatch.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	gatewayv1 "github.com/kubediscovery/kd-libs/core/v1/gateway"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kubediscovery/kd-agent/internal/core/agent/executor"
)

const heartbeatInterval = 10 * time.Second

// ErrEmptyAgentID is returned when the agent identifier is blank.
var ErrEmptyAgentID = errors.New("agent_id must not be empty")

// ErrMaxRetriesExceeded is returned when the retry loop exhausts all attempts.
var ErrMaxRetriesExceeded = errors.New("max connection retries exceeded")

// RetryConfig holds the exponential-backoff parameters for the connection loop.
type RetryConfig struct {
	// BaseDelay is the wait before the second attempt (attempt index 1).
	BaseDelay time.Duration

	// Multiplier is applied to BaseDelay on each subsequent attempt.
	Multiplier int

	// MaxAttempts is the total number of allowed stream opens. After this
	// count is exhausted Run returns ErrMaxRetriesExceeded.
	MaxAttempts int
}

// DefaultRetryConfig returns the spec-mandated retry configuration:
// base 1 s, multiplier 3, maximum 5 attempts
// (pre-attempt delays: 0 s, 1 s, 3 s, 9 s, 27 s → fatal after 5th).
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		BaseDelay:   1 * time.Second,
		Multiplier:  3,
		MaxAttempts: 5,
	}
}

// DelayForAttempt returns the pre-attempt delay for the given attempt index
// (0-based). Attempt 0 always returns 0 (no delay before the first try).
func (c RetryConfig) DelayForAttempt(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	delay := c.BaseDelay
	for i := 1; i < attempt; i++ {
		delay *= time.Duration(c.Multiplier)
	}
	return delay
}

// StreamOpener opens a bidirectional AgentStream to the gateway.
// The single method is intentionally minimal to allow easy test doubles.
type StreamOpener interface {
	OpenStream(ctx context.Context) (gatewayv1.GatewayService_AgentStreamClient, error)
}

// Service manages the agent connection lifecycle: exponential-backoff retry
// loop, hello handshake, heartbeat, and command dispatch.
type Service struct {
	agentID  string
	retry    RetryConfig
	opener   StreamOpener
	dispatch executor.Dispatcher
	log      *slog.Logger
}

// New constructs a Service. Returns ErrEmptyAgentID when agentID is blank.
func New(
	agentID string,
	retry RetryConfig,
	opener StreamOpener,
	dispatch executor.Dispatcher,
	log *slog.Logger,
) (*Service, error) {
	if agentID == "" {
		return nil, ErrEmptyAgentID
	}
	return &Service{
		agentID:  agentID,
		retry:    retry,
		opener:   opener,
		dispatch: dispatch,
		log:      log,
	}, nil
}

// AgentID returns the logical identifier of this agent instance.
func (s *Service) AgentID() string { return s.agentID }

// Run executes the retry loop, blocking until ctx is cancelled or the maximum
// number of attempts is exhausted.
//
// On each iteration:
//  1. Wait for the back-off delay (zero on the first attempt).
//  2. Open the stream, send hello, and run the recv loop.
//  3. If the stream exits due to context cancellation, return immediately.
//  4. Otherwise increment the attempt counter and retry (up to MaxAttempts).
func (s *Service) Run(ctx context.Context) error {
	for attempt := 0; ; attempt++ {
		if attempt >= s.retry.MaxAttempts {
			return fmt.Errorf("%w: after %d attempts", ErrMaxRetriesExceeded, s.retry.MaxAttempts)
		}

		delay := s.retry.DelayForAttempt(attempt)
		if delay > 0 {
			s.log.Info("waiting before reconnect",
				slog.Int("attempt", attempt),
				slog.Duration("delay", delay),
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := s.runStream(ctx)
		if err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}

		s.log.Warn("stream ended with error, will retry",
			slog.Int("attempt", attempt),
			slog.Any("error", err),
		)
	}
}

// runStream opens one stream, sends the initial AgentHello and then runs the
// recv loop until the stream ends or ctx is cancelled.
func (s *Service) runStream(ctx context.Context) error {
	stream, err := s.opener.OpenStream(ctx)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}

	if err := s.sendHello(stream); err != nil {
		return err
	}

	return s.recvLoop(ctx, stream)
}

// sendHello sends the initial AgentHello frame carrying the caller_id.
func (s *Service) sendHello(stream gatewayv1.GatewayService_AgentStreamClient) error {
	msg := &gatewayv1.AgentStreamMessage{
		Payload: &gatewayv1.AgentStreamMessage_Hello{
			Hello: &gatewayv1.AgentHello{
				CallerId: s.agentID,
				SentAt:   timestamppb.Now(),
			},
		},
	}
	if err := stream.Send(msg); err != nil {
		return fmt.Errorf("send hello: %w", err)
	}
	s.log.Info("agent hello sent", slog.String("caller_id", s.agentID))
	return nil
}

// recvLoop reads messages from the gateway and dispatches them synchronously.
// It manages a sender goroutine (sole writer to the stream) and a heartbeat
// goroutine. Both goroutines are stopped and waited for before the function
// returns, ensuring no goroutine leaks and no data races on the stream.
func (s *Service) recvLoop(ctx context.Context, stream gatewayv1.GatewayService_AgentStreamClient) error {
	loopCtx, loopCancel := context.WithCancel(ctx)
	defer loopCancel()

	// sendCh funnels all outgoing frames through a single sender goroutine,
	// which is the only goroutine allowed to call stream.Send().
	sendCh := make(chan *gatewayv1.AgentStreamMessage, 64)

	var hbDone, senderDone sync.WaitGroup

	// Sender goroutine: sole writer to the stream.
	senderDone.Add(1)
	go func() {
		defer senderDone.Done()
		for msg := range sendCh {
			if err := stream.Send(msg); err != nil {
				s.log.Error("stream send failed", slog.Any("error", err))
				return
			}
		}
	}()

	// Heartbeat goroutine: sends a heartbeat frame every heartbeatInterval.
	hbDone.Add(1)
	go func() {
		defer hbDone.Done()
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-loopCtx.Done():
				return
			case <-ticker.C:
				select {
				case sendCh <- s.heartbeatMsg():
				case <-loopCtx.Done():
					return
				}
			}
		}
	}()

	var recvErr error
	for {
		msg, err := stream.Recv()
		if err != nil {
			recvErr = err
			break
		}

		switch p := msg.Payload.(type) {
		case *gatewayv1.AgentStreamMessage_Command:
			result := s.executeCommand(loopCtx, p.Command)
			select {
			case sendCh <- &gatewayv1.AgentStreamMessage{
				Payload: &gatewayv1.AgentStreamMessage_CommandResult{
					CommandResult: result,
				},
			}:
			case <-loopCtx.Done():
			}
		case *gatewayv1.AgentStreamMessage_Heartbeat:
			s.log.Debug("heartbeat ack from gateway",
				slog.String("request_id", p.Heartbeat.GetRequestId()),
			)
		default:
			s.log.Warn("unexpected message type received from gateway")
		}
	}

	// Stop the heartbeat goroutine, then safely close the channel and wait
	// for the sender goroutine to drain it before returning.
	loopCancel()
	hbDone.Wait()
	close(sendCh)
	senderDone.Wait()

	return recvErr
}

// heartbeatMsg builds a periodic heartbeat message.
func (s *Service) heartbeatMsg() *gatewayv1.AgentStreamMessage {
	return &gatewayv1.AgentStreamMessage{
		Payload: &gatewayv1.AgentStreamMessage_Heartbeat{
			Heartbeat: &gatewayv1.AgentHeartbeat{
				CallerId: s.agentID,
				SentAt:   timestamppb.Now(),
			},
		},
	}
}

// executeCommand dispatches a GatewayCommand to the local executor and returns
// the result. When the executor reports an error, an error result is returned
// so the gateway is always notified. The caller_id is always stamped on the
// result so the gateway knows who replied.
func (s *Service) executeCommand(ctx context.Context, cmd *gatewayv1.GatewayCommand) *gatewayv1.AgentCommandResult {
	result, err := s.dispatch.Dispatch(ctx, cmd)
	if err != nil {
		return &gatewayv1.AgentCommandResult{
			RequestId: cmd.GetRequestId(),
			CallerId:  s.agentID,
			Success:   false,
			Message:   fmt.Sprintf("executor error: %v", err),
		}
	}
	if result != nil {
		result.CallerId = s.agentID
	}
	return result
}
