// Package registry provides the in-memory store of connected kd-agent instances.
//
// The Registry is the authoritative runtime state for the gateway: every
// gRPC handler that needs to know which agents are online, route commands, or
// track heartbeat TTLs reads and writes through this single component.
//
// Thread-safety guarantee: all exported methods are safe for concurrent use.
package registry

import (
	"fmt"
	"sync"
	"time"

	gatewayv1 "github.com/kubediscovery/kd-libs/core/v1/gateway"
	"google.golang.org/grpc"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/entity"
)

// ErrAlreadyConnected is returned by Register when a caller_id is already
// present in the registry with StatusConnected.
var ErrAlreadyConnected = fmt.Errorf("agent already connected")

// Registry is the thread-safe in-memory store for connected agents.
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*entity.Agent
}

// New returns an empty Registry ready for use.
func New() *Registry {
	return &Registry{
		agents: make(map[string]*entity.Agent),
	}
}

// Register adds a newly-connected agent to the registry.
//
// If a connected agent with the same caller_id already exists the call
// returns ErrAlreadyConnected without modifying the registry.  The caller
// decides the conflict resolution policy (e.g. reject new or evict old).
func (r *Registry) Register(
	callerID string,
	stream grpc.BidiStreamingServer[gatewayv1.AgentStreamMessage, gatewayv1.AgentStreamMessage],
	metadata map[string]any,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.agents[callerID]; ok && existing.Status == entity.StatusConnected {
		return ErrAlreadyConnected
	}

	now := time.Now()
	r.agents[callerID] = &entity.Agent{
		CallerID:    callerID,
		Stream:      stream,
		Metadata:    metadata,
		ConnectedAt: now,
		LastSeenAt:  now,
		Status:      entity.StatusConnected,
	}
	return nil
}

// Deregister marks an agent as disconnected and clears its stream reference.
// If no agent with the given callerID exists the call is a no-op.
func (r *Registry) Deregister(callerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if a, ok := r.agents[callerID]; ok {
		a.Status = entity.StatusDisconnected
		a.Stream = nil
	}
}

// TouchHeartbeat updates LastSeenAt for the agent identified by callerID.
// Returns false if no connected agent with that ID exists.
func (r *Registry) TouchHeartbeat(callerID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	a, ok := r.agents[callerID]
	if !ok || a.Status != entity.StatusConnected {
		return false
	}
	a.LastSeenAt = time.Now()
	return true
}

// Get returns the agent entry for the given callerID and whether it was found.
// The returned pointer should be treated as read-only by callers.
func (r *Registry) Get(callerID string) (*entity.Agent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.agents[callerID]
	return a, ok
}

// List returns a snapshot of all agent entries (connected and disconnected).
// The returned slice owns its elements; mutating them does not affect the registry.
func (r *Registry) List() []*entity.Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*entity.Agent, 0, len(r.agents))
	for _, a := range r.agents {
		cp := *a
		out = append(out, &cp)
	}
	return out
}

// ConnectedCount returns the number of agents currently in StatusConnected.
func (r *Registry) ConnectedCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	n := 0
	for _, a := range r.agents {
		if a.Status == entity.StatusConnected {
			n++
		}
	}
	return n
}
