// Package entity defines pure data structures for the agent domain.
package entity

import "time"

// ConnectionStatus represents the current state of the agent's stream connection.
type ConnectionStatus string

const (
	// StatusConnecting indicates the agent is establishing or re-establishing
	// the gRPC stream to the gateway.
	StatusConnecting ConnectionStatus = "connecting"

	// StatusConnected indicates the stream is open and the hello frame was
	// successfully sent.
	StatusConnected ConnectionStatus = "connected"

	// StatusDisconnected indicates the stream ended (gracefully or due to an
	// error) and a retry may follow.
	StatusDisconnected ConnectionStatus = "disconnected"
)

// Connection holds the runtime state of one agent-to-gateway stream attempt.
type Connection struct {
	// AgentID is the logical identifier sent as caller_id in every frame.
	AgentID string

	// Status is the current stream lifecycle state.
	Status ConnectionStatus

	// ConnectedAt is when the stream was first opened successfully.
	ConnectedAt time.Time

	// LastHeartbeatAt is when the most recent heartbeat was sent.
	LastHeartbeatAt time.Time

	// Attempt is the current retry attempt index (0-based).
	Attempt int
}
