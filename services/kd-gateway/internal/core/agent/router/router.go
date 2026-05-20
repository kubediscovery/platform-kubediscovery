// Package router implements command routing from the gateway to connected agents.
//
// The Router is the single component responsible for:
//   - Looking up the target agent by caller_id in the in-memory registry.
//   - Forwarding a GatewayCommand over the agent's live bidirectional stream.
//   - Matching the corresponding AgentCommandResult back to the waiting caller
//     via a per-request-id channel.
//
// If the target agent is absent or offline, Send returns a gRPC
// codes.Unavailable error immediately — no retry is attempted at this layer.
//
// Thread-safety: all exported methods are safe for concurrent use.
package router

import (
	"context"
	"log/slog"
	"sync"

	gatewayv1 "github.com/kubediscovery/kd-libs/core/v1/gateway"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/registry"
)

// Router routes GatewayCommands to connected agents and delivers results back
// to their callers.
type Router struct {
	registry *registry.Registry
	log      *slog.Logger

	mu      sync.Mutex
	waiters map[string]chan *gatewayv1.AgentCommandResult
}

// New returns a Router wired to the given Registry and logger.
func New(reg *registry.Registry, log *slog.Logger) *Router {
	return &Router{
		registry: reg,
		log:      log,
		waiters:  make(map[string]chan *gatewayv1.AgentCommandResult),
	}
}

// Send routes cmd to the agent identified by callerID and blocks until a
// matching AgentCommandResult is received or ctx is cancelled.
//
// Error semantics:
//   - codes.Unavailable  — agent is not connected (absent or offline)
//   - codes.Unavailable  — the stream Send call failed (agent just disconnected)
//   - codes.DeadlineExceeded / codes.Canceled — ctx expired before a result arrived
func (r *Router) Send(ctx context.Context, callerID string, cmd *gatewayv1.GatewayCommand) (*gatewayv1.AgentCommandResult, error) {
	stream, ok := r.registry.GetConnectedStream(callerID)
	if !ok {
		r.log.Warn("route command: agent offline",
			slog.String("caller_id", callerID),
			slog.String("request_id", cmd.GetRequestId()),
		)
		return nil, status.Errorf(codes.Unavailable, "agent %q is offline", callerID)
	}

	ch := make(chan *gatewayv1.AgentCommandResult, 1)
	r.addWaiter(cmd.GetRequestId(), ch)
	defer r.removeWaiter(cmd.GetRequestId())

	msg := &gatewayv1.AgentStreamMessage{
		Payload: &gatewayv1.AgentStreamMessage_Command{
			Command: cmd,
		},
	}

	if err := stream.Send(msg); err != nil {
		r.log.Error("route command: stream send failed",
			slog.String("caller_id", callerID),
			slog.String("request_id", cmd.GetRequestId()),
			slog.Any("error", err),
		)
		return nil, status.Errorf(codes.Unavailable, "send command to agent %q: %v", callerID, err)
	}

	r.log.Debug("command routed, awaiting result",
		slog.String("caller_id", callerID),
		slog.String("request_id", cmd.GetRequestId()),
		slog.String("operation", cmd.GetOperation()),
	)

	select {
	case result := <-ch:
		return result, nil
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	}
}

// Deliver routes an incoming AgentCommandResult to the caller waiting on the
// corresponding request_id.  It is called by the gRPC stream handler whenever
// an AgentCommandResult frame arrives from an agent.
//
// If no waiter is registered (e.g. the caller already timed out), the result
// is dropped and a warning is logged.
func (r *Router) Deliver(result *gatewayv1.AgentCommandResult) {
	reqID := result.GetRequestId()

	r.mu.Lock()
	ch, ok := r.waiters[reqID]
	r.mu.Unlock()

	if !ok {
		r.log.Warn("route result: no waiter found, dropping result",
			slog.String("request_id", reqID),
			slog.String("caller_id", result.GetCallerId()),
		)
		return
	}

	select {
	case ch <- result:
	default:
		// The channel has a buffer of 1; a second delivery for the same
		// request_id would block — drop it to avoid a goroutine leak.
		r.log.Warn("route result: waiter channel full, dropping duplicate result",
			slog.String("request_id", reqID),
		)
	}
}

// addWaiter registers a result channel for the given requestID.
func (r *Router) addWaiter(requestID string, ch chan *gatewayv1.AgentCommandResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.waiters[requestID] = ch
}

// removeWaiter removes the result channel for the given requestID.
func (r *Router) removeWaiter(requestID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.waiters, requestID)
}
