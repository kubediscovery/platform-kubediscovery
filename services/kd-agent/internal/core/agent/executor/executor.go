// Package executor defines the interface for dispatching gateway commands to
// the local kd-executor service.
package executor

import (
	"context"

	gatewayv1 "github.com/kubediscovery/kd-libs/core/v1/gateway"
)

// Dispatcher dispatches a GatewayCommand to the local kd-executor and returns
// the result. Implementations must be safe for concurrent use.
type Dispatcher interface {
	Dispatch(ctx context.Context, cmd *gatewayv1.GatewayCommand) (*gatewayv1.AgentCommandResult, error)
}

// UnavailableDispatcher is a no-op Dispatcher that always reports the
// kd-executor as unavailable. It is used when no real executor is configured.
type UnavailableDispatcher struct{}

// Dispatch always returns an UNAVAILABLE error so the gateway is informed that
// no kd-executor is reachable on this agent.
func (UnavailableDispatcher) Dispatch(_ context.Context, cmd *gatewayv1.GatewayCommand) (*gatewayv1.AgentCommandResult, error) {
	return &gatewayv1.AgentCommandResult{
		RequestId: cmd.GetRequestId(),
		Success:   false,
		Message:   "kd-executor unavailable",
	}, nil
}
