// Package stream handles the bidirectional gRPC stream between kd-agent and
// kd-gateway, including receiving GatewayCommand frames and sending back
// AgentCommandResult frames.
package stream

import (
	"context"
	"errors"
	"log/slog"
	"time"

	executorv1 "github.com/kubediscovery/kd-libs/core/v1/executor"
	gatewayv1 "github.com/kubediscovery/kd-libs/core/v1/gateway"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/kubediscovery/kd-agent/internal/executor"
)

// Handler processes GatewayCommand messages received from the kd-gateway
// bidirectional stream and produces AgentCommandResult frames.
//
// Commands are delegated to the local kd-executor via the executor.Client
// interface. When kd-executor is unavailable the handler returns a failed
// CommandResult with the "executor not available" message, allowing kd-gateway
// to surface a gRPC UNAVAILABLE status to the original caller.
type Handler struct {
	callerID       string
	executorClient executor.Client
	log            *slog.Logger
}

// New constructs a Handler for the given agent identity and executor client.
// A nil executorClient is treated as executor unavailable — every command will
// return an UNAVAILABLE result.
func New(callerID string, execClient executor.Client, log *slog.Logger) *Handler {
	return &Handler{
		callerID:       callerID,
		executorClient: execClient,
		log:            log,
	}
}

// HandleCommand processes a single GatewayCommand and returns the
// AgentCommandResult to be sent back through the gateway stream.
//
// When kd-executor is unavailable the result will have:
//   - Success: false
//   - Message: "executor not available"
//
// This allows kd-gateway to map the result to gRPC codes.Unavailable when
// responding to the original upstream caller.
func (h *Handler) HandleCommand(ctx context.Context, cmd *gatewayv1.GatewayCommand) *gatewayv1.AgentCommandResult {
	h.log.Info("command received from gateway",
		slog.String("caller_id", h.callerID),
		slog.String("request_id", cmd.GetRequestId()),
		slog.String("operation", cmd.GetOperation()),
	)

	if h.executorClient == nil {
		h.log.Warn("executor client not configured, returning UNAVAILABLE",
			slog.String("request_id", cmd.GetRequestId()),
		)
		return h.unavailableResult(cmd.GetRequestId())
	}

	execCmd := gatewayCommandToExecutorCommand(cmd)
	resp, err := h.executorClient.Execute(ctx, execCmd)
	if err != nil {
		if errors.Is(err, executor.ErrUnavailable) {
			h.log.Warn("kd-executor unavailable",
				slog.String("request_id", cmd.GetRequestId()),
				slog.Any("error", err),
			)
			return h.unavailableResult(cmd.GetRequestId())
		}

		h.log.Error("executor returned error",
			slog.String("request_id", cmd.GetRequestId()),
			slog.Any("error", err),
		)
		return &gatewayv1.AgentCommandResult{
			RequestId:   cmd.GetRequestId(),
			CallerId:    h.callerID,
			Success:     false,
			Message:     err.Error(),
			RespondedAt: timestamppb.New(time.Now()),
		}
	}

	h.log.Info("executor command succeeded",
		slog.String("request_id", cmd.GetRequestId()),
		slog.Bool("success", resp.GetSuccess()),
	)

	msg := resp.GetOutput()
	if !resp.GetSuccess() {
		msg = resp.GetError()
	}

	return &gatewayv1.AgentCommandResult{
		RequestId:   cmd.GetRequestId(),
		CallerId:    h.callerID,
		Success:     resp.GetSuccess(),
		Message:     msg,
		RespondedAt: timestamppb.New(time.Now()),
	}
}

// unavailableResult returns the canonical AgentCommandResult for the case
// where kd-executor is not available.
func (h *Handler) unavailableResult(requestID string) *gatewayv1.AgentCommandResult {
	return &gatewayv1.AgentCommandResult{
		RequestId:   requestID,
		CallerId:    h.callerID,
		Success:     false,
		Message:     "executor not available",
		RespondedAt: timestamppb.New(time.Now()),
	}
}

// gatewayCommandToExecutorCommand converts a GatewayCommand received from
// kd-gateway into the ExecutorCommand format expected by kd-executor.
// The GatewayCommand.Payload fields are mapped to the ExecutorCommand params.
func gatewayCommandToExecutorCommand(cmd *gatewayv1.GatewayCommand) *executorv1.ExecutorCommand {
	params := make(map[string]string)
	if cmd.GetPayload() != nil {
		for k, v := range cmd.GetPayload().AsMap() {
			if s, ok := v.(string); ok {
				params[k] = s
			}
		}
	}

	return &executorv1.ExecutorCommand{
		RequestId:  cmd.GetRequestId(),
		ActionType: cmd.GetOperation(),
		Source:     "gateway",
		Params:     params,
		SentAt:     cmd.GetSentAt(),
	}
}
