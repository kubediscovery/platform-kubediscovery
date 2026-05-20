// Package handler implements the GatewayService gRPC server for kd-gateway.
//
// # Bidirectional stream lifecycle
//
//  1. The agent dials kd-gateway and opens the AgentStream RPC.
//  2. The first frame MUST be AgentHello carrying a non-empty caller_id.
//     The handler rejects the stream with codes.InvalidArgument if hello is absent
//     or caller_id is empty.
//  3. The caller_id is used to register the agent in the in-memory Registry.
//     If an active registration already exists for that caller_id the stream is
//     rejected with codes.AlreadyExists (conflict policy: reject the new stream).
//  4. Subsequent frames are AgentHeartbeat or AgentCommandResult.
//     Heartbeats update LastSeenAt in the registry; command results are currently
//     logged (routing to waiters is introduced in task 4.5).
//  5. When the stream ends (agent disconnect or context cancellation) the agent
//     is deregistered and its status set to StatusDisconnected.
package handler

import (
	"errors"
	"io"
	"log/slog"

	gatewayv1 "github.com/kubediscovery/kd-libs/core/v1/gateway"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/registry"
)

// Handler implements gatewayv1.GatewayServiceServer.
type Handler struct {
	gatewayv1.UnimplementedGatewayServiceServer

	registry *registry.Registry
	log      *slog.Logger
}

// New constructs a Handler wired to the given Registry and logger.
func New(reg *registry.Registry, log *slog.Logger) *Handler {
	return &Handler{
		registry: reg,
		log:      log,
	}
}

// AgentStream is the single bidirectional RPC exposed by GatewayService.
// It blocks until the agent disconnects or an error occurs.
func (h *Handler) AgentStream(
	stream gatewayv1.GatewayService_AgentStreamServer,
) error {
	// --- 1. Receive and validate AgentHello ---
	firstMsg, err := stream.Recv()
	if err != nil {
		h.log.Warn("agent stream: failed to receive first message", slog.Any("error", err))
		return status.Errorf(codes.Internal, "receive first message: %v", err)
	}

	hello := firstMsg.GetHello()
	if hello == nil {
		return status.Error(codes.InvalidArgument, "first message must be AgentHello")
	}
	if hello.GetCallerId() == "" {
		return status.Error(codes.InvalidArgument, "AgentHello.caller_id must not be empty")
	}

	callerID := hello.GetCallerId()
	metadata := structToMap(hello.GetMetadata().AsMap())

	h.log.Info("agent hello received",
		slog.String("caller_id", callerID),
	)

	// --- 2. Register the agent in the in-memory map ---
	if err := h.registry.Register(callerID, stream, metadata); err != nil {
		if errors.Is(err, registry.ErrAlreadyConnected) {
			h.log.Warn("agent stream: caller_id already connected, rejecting new stream",
				slog.String("caller_id", callerID),
			)
			return status.Errorf(codes.AlreadyExists, "caller_id %q already has an active stream", callerID)
		}
		return status.Errorf(codes.Internal, "register agent: %v", err)
	}

	h.log.Info("agent registered",
		slog.String("caller_id", callerID),
		slog.Int("total_connected", h.registry.ConnectedCount()),
	)

	// Deregister on exit regardless of the reason.
	defer func() {
		h.registry.Deregister(callerID)
		h.log.Info("agent deregistered",
			slog.String("caller_id", callerID),
			slog.Int("total_connected", h.registry.ConnectedCount()),
		)
	}()

	// --- 3. Stream loop: heartbeats and command results ---
	for {
		msg, err := stream.Recv()
		if err != nil {
			// EOF or context cancellation are expected; anything else is an error.
			if isStreamClosed(err) {
				h.log.Info("agent stream closed", slog.String("caller_id", callerID))
				return nil
			}
			h.log.Error("agent stream recv error",
				slog.String("caller_id", callerID),
				slog.Any("error", err),
			)
			return status.Errorf(codes.Internal, "stream recv: %v", err)
		}

		switch p := msg.Payload.(type) {
		case *gatewayv1.AgentStreamMessage_Heartbeat:
			hb := p.Heartbeat
			if !h.registry.TouchHeartbeat(callerID) {
				h.log.Warn("heartbeat received for unknown/disconnected agent",
					slog.String("caller_id", callerID),
				)
			} else {
				h.log.Debug("heartbeat received",
					slog.String("caller_id", callerID),
					slog.String("request_id", hb.GetRequestId()),
				)
			}

		case *gatewayv1.AgentStreamMessage_CommandResult:
			result := p.CommandResult
			h.log.Info("command result received",
				slog.String("caller_id", callerID),
				slog.String("request_id", result.GetRequestId()),
				slog.Bool("success", result.GetSuccess()),
				slog.String("message", result.GetMessage()),
			)

		default:
			h.log.Warn("unexpected message type in stream",
				slog.String("caller_id", callerID),
			)
		}
	}
}

// structToMap converts the output of protobuf Struct.AsMap() to a plain
// map[string]any, returning nil for a nil input.
func structToMap(m map[string]any) map[string]any {
	if len(m) == 0 {
		return nil
	}
	return m
}

// isStreamClosed returns true for errors that represent a normal stream end
// (EOF, context cancellation or deadline exceeded).
func isStreamClosed(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	code := status.Code(err)
	return code == codes.Canceled || code == codes.DeadlineExceeded || code == codes.OK
}
