// Package handler implements the GatewayService gRPC server for kd-gateway.
//
// # Bidirectional stream lifecycle
//
//  1. The agent dials kd-gateway and opens the AgentStream RPC.
//  2. The first frame MUST be AgentHello carrying a non-empty caller_id.
//     The handler rejects the stream with codes.InvalidArgument if hello is absent
//     or caller_id is empty.
//  3. The caller_id is used to register the agent in the in-memory Registry.
//     If an active registration already exists for that caller_id the duplicate
//     policy (configured via AGENT_DUPLICATE_POLICY) decides the outcome:
//       - "reject_new" (default): reject the incoming stream with codes.AlreadyExists.
//       - "evict_previous": forcibly terminate the existing stream (codes.Aborted) and
//         accept the new connection.
//  4. Subsequent frames are AgentHeartbeat or AgentCommandResult.
//     Heartbeats update LastSeenAt in the registry; command results are forwarded
//     to the ResultSink (typically the Router) which matches them to waiting callers.
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

	"github.com/kubediscovery/kd-gateway/configs"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/registry"
)

// ResultSink receives AgentCommandResult frames from the agent stream loop and
// routes them back to the caller waiting on the matching request_id.
//
// The Router in internal/core/agent/router implements this interface.
// Passing nil disables result routing (command results are only logged).
type ResultSink interface {
	Deliver(result *gatewayv1.AgentCommandResult)
}

// recvResult wraps a single Recv() call outcome.
type recvResult struct {
	msg *gatewayv1.AgentStreamMessage
	err error
}

// Handler implements gatewayv1.GatewayServiceServer.
type Handler struct {
	gatewayv1.UnimplementedGatewayServiceServer

	registry        *registry.Registry
	resultSink      ResultSink // nil-safe: results are logged but not routed when nil
	log             *slog.Logger
	duplicatePolicy configs.DuplicatePolicy
}

// New constructs a Handler wired to the given Registry, logger, result sink,
// and application config.  Passing nil for sink disables active result routing.
func New(reg *registry.Registry, log *slog.Logger, sink ResultSink, cfg *configs.Config) *Handler {
	return &Handler{
		registry:        reg,
		resultSink:      sink,
		log:             log,
		duplicatePolicy: cfg.Agent.DuplicatePolicy,
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

	// --- 2. Register the agent, applying the configured duplicate policy ---
	evictCh, regErr := h.registerWithPolicy(callerID, stream, metadata)
	if regErr != nil {
		return regErr
	}

	h.log.Info("agent registered",
		slog.String("caller_id", callerID),
		slog.Int("total_connected", h.registry.ConnectedCount()),
	)

	// evicted tracks whether this stream was terminated by a ForceRegister from
	// a new connection.  When true the deferred Deregister call must be skipped
	// because the registry already holds the new agent's entry for caller_id.
	evicted := false

	defer func() {
		if !evicted {
			h.registry.Deregister(callerID)
			h.log.Info("agent deregistered",
				slog.String("caller_id", callerID),
				slog.Int("total_connected", h.registry.ConnectedCount()),
			)
		} else {
			h.log.Info("agent stream ended (evicted by new connection)",
				slog.String("caller_id", callerID),
			)
		}
	}()

	// --- 3. Fan out stream.Recv() into a channel ---
	// Running Recv in a goroutine allows the main loop to select on both
	// incoming messages and the evict signal without blocking indefinitely.
	done := make(chan struct{})
	defer close(done)

	recvCh := make(chan recvResult, 1)
	go func() {
		for {
			msg, err := stream.Recv()
			select {
			case recvCh <- recvResult{msg, err}:
			case <-done:
				return
			}
			if err != nil {
				return
			}
		}
	}()

	// --- 4. Stream loop: heartbeats, command results, and eviction ---
	for {
		select {
		case <-evictCh:
			// A new connection with the same caller_id has taken over via
			// ForceRegister.  Terminate this stream so the client reconnects.
			evicted = true
			h.log.Warn("agent stream evicted: new connection with same caller_id",
				slog.String("caller_id", callerID),
			)
			return status.Errorf(codes.Aborted, "caller_id %q evicted: new connection established", callerID)

		case result := <-recvCh:
			msg, err := result.msg, result.err
			if err != nil {
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
				if h.resultSink != nil {
					h.resultSink.Deliver(result)
				}

			default:
				h.log.Warn("unexpected message type in stream",
					slog.String("caller_id", callerID),
				)
			}
		}
	}
}

// registerWithPolicy attempts to register the agent and applies the configured
// duplicate policy when ErrAlreadyConnected is returned.
//
// On success it returns the evict channel for this agent registration.
// On failure it returns a gRPC status error ready to be returned from AgentStream.
func (h *Handler) registerWithPolicy(
	callerID string,
	stream gatewayv1.GatewayService_AgentStreamServer,
	metadata map[string]any,
) (<-chan struct{}, error) {
	evictCh, err := h.registry.Register(callerID, stream, metadata)
	if err == nil {
		return evictCh, nil
	}

	if !errors.Is(err, registry.ErrAlreadyConnected) {
		return nil, status.Errorf(codes.Internal, "register agent: %v", err)
	}

	// Duplicate caller_id — apply policy.
	switch h.duplicatePolicy {
	case configs.DuplicatePolicyEvictPrevious:
		h.log.Warn("agent stream: caller_id already connected, evicting previous stream",
			slog.String("caller_id", callerID),
			slog.String("policy", string(h.duplicatePolicy)),
		)
		evictCh = h.registry.ForceRegister(callerID, stream, metadata)
		return evictCh, nil

	default: // DuplicatePolicyRejectNew or any unrecognised value
		h.log.Warn("agent stream: caller_id already connected, rejecting new stream",
			slog.String("caller_id", callerID),
			slog.String("policy", string(h.duplicatePolicy)),
		)
		return nil, status.Errorf(codes.AlreadyExists, "caller_id %q already has an active stream", callerID)
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
