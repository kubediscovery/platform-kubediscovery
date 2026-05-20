// Package executor defines the interface through which kd-agent communicates
// with the local kd-executor component.
//
// The kd-agent never executes Kubernetes commands directly; it delegates every
// command to kd-executor and forwards the result back to kd-gateway.
// When kd-executor is not reachable the agent must surface the failure
// upstream with the gRPC UNAVAILABLE status code.
package executor

import (
	"context"
	"errors"

	executorv1 "github.com/kubediscovery/kd-libs/core/v1/executor"
)

// ErrUnavailable is returned by Client.Execute when kd-executor is not
// reachable or has not yet registered with the agent.
// Callers should map this to gRPC codes.Unavailable when forwarding the
// error to kd-gateway.
var ErrUnavailable = errors.New("executor not available")

// Client is the contract that kd-agent uses to delegate Kubernetes commands
// to the local kd-executor component.
//
// Implementations must return ErrUnavailable (or wrap it) whenever the
// underlying transport cannot reach kd-executor.
type Client interface {
	// Execute forwards cmd to kd-executor and blocks until kd-executor
	// returns a result or the context is cancelled.
	//
	// Returns ErrUnavailable if kd-executor is not reachable.
	Execute(ctx context.Context, cmd *executorv1.ExecutorCommand) (*executorv1.ExecutorResponse, error)
}
