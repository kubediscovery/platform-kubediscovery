// Package entity defines the pure domain types for the agent core.
package entity

import (
	"time"

	gatewayv1 "github.com/kubediscovery/kd-libs/core/v1/gateway"
	"google.golang.org/grpc"
)

// Status represents the lifecycle state of a connected agent.
type Status string

const (
	// StatusConnected means the agent stream is active.
	StatusConnected Status = "connected"
	// StatusDisconnected means the agent stream was closed or the heartbeat TTL expired.
	StatusDisconnected Status = "disconnected"
)

// Agent holds everything the gateway knows about a connected kd-agent instance.
type Agent struct {
	// CallerID is the logical, self-reported identifier sent in AgentHello.
	// It is used as the map key in the in-memory registry.
	CallerID string

	// Stream is the live bidirectional gRPC stream for this agent.
	// It is nil when Status == StatusDisconnected.
	Stream grpc.BidiStreamingServer[gatewayv1.AgentStreamMessage, gatewayv1.AgentStreamMessage]

	// Metadata is the free-form diagnostics map from AgentHello.
	Metadata map[string]any

	// ConnectedAt is when the AgentHello frame was first received.
	ConnectedAt time.Time

	// LastSeenAt is the time of the most recently received frame (hello or heartbeat).
	LastSeenAt time.Time

	// Status is the current lifecycle state of this agent entry.
	Status Status
}
