// Package dispatch handles incoming GatewayCommand frames from the kd-gateway
// bidirectional stream and forwards them to the local kd-executor.
package dispatch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	gatewayv1 "github.com/kubediscovery/kd-libs/core/v1/gateway"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ErrExecutorUnavailable is returned by Executor.Execute implementations
// when the kd-executor service is not reachable.
var ErrExecutorUnavailable = errors.New("executor unavailable")

// Executor is the interface that the local kd-executor must satisfy.
// The Dispatcher calls Execute for every GatewayCommand received from the stream.
type Executor interface {
	// Execute runs the requested operation and returns a free-form result map.
	// Implementations must return ErrExecutorUnavailable (or wrap it) when the
	// kd-executor service cannot be reached so the Dispatcher can report
	// codes.Unavailable back to the gateway.
	Execute(ctx context.Context, operation string, payload map[string]any) (map[string]any, error)
}

// Dispatcher receives GatewayCommand frames and routes them to the Executor.
// It is not safe for concurrent use by multiple goroutines; the stream
// Receiver goroutine is the sole caller.
type Dispatcher struct {
	callerID string
	executor Executor
	log      *slog.Logger
}

// New creates a Dispatcher bound to the given executor and logger.
// callerID is the agent's own identifier (AGENT_ID) used to populate
// AgentCommandResult.caller_id.
func New(callerID string, executor Executor, log *slog.Logger) *Dispatcher {
	return &Dispatcher{
		callerID: callerID,
		executor: executor,
		log:      log,
	}
}

// Dispatch processes a single GatewayCommand and returns an AgentCommandResult
// to be sent back to the gateway.
//
// When the executor reports ErrExecutorUnavailable (or wraps it) the result
// carries a gRPC codes.Unavailable status embedded in the message field and
// success=false, matching task 5.7 of the specification.
func (d *Dispatcher) Dispatch(
	ctx context.Context,
	cmd *gatewayv1.GatewayCommand,
) (*gatewayv1.AgentStreamMessage, error) {
	if cmd == nil {
		return nil, fmt.Errorf("nil command")
	}

	d.log.Info("dispatching command",
		slog.String("request_id", cmd.GetRequestId()),
		slog.String("operation", cmd.GetOperation()),
	)

	resultPayload, execErr := d.executor.Execute(ctx, cmd.GetOperation(), cmd.GetPayload().AsMap())

	var (
		success bool
		msg     string
		pbPayload *structpb.Struct
	)

	if execErr != nil {
		if errors.Is(execErr, ErrExecutorUnavailable) {
			msg = status.New(codes.Unavailable, "kd-executor not available").Err().Error()
		} else {
			msg = execErr.Error()
		}
		d.log.Warn("command execution failed",
			slog.String("request_id", cmd.GetRequestId()),
			slog.String("error", execErr.Error()),
		)
	} else {
		success = true
		msg = "ok"
		var err error
		pbPayload, err = structpb.NewStruct(resultPayload)
		if err != nil {
			d.log.Warn("failed to marshal executor result payload",
				slog.String("request_id", cmd.GetRequestId()),
				slog.Any("error", err),
			)
		}
	}

	result := &gatewayv1.AgentCommandResult{
		RequestId:   cmd.GetRequestId(),
		CallerId:    d.callerID,
		Success:     success,
		Message:     msg,
		Payload:     pbPayload,
		RespondedAt: timestamppb.Now(),
	}

	return &gatewayv1.AgentStreamMessage{
		Payload: &gatewayv1.AgentStreamMessage_CommandResult{
			CommandResult: result,
		},
	}, nil
}
