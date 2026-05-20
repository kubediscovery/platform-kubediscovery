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
	mu         sync.RWMutex
	agents     map[string]*entity.Agent
	evictChans map[string]chan struct{} // per-callerID evict signal channels
}

// New returns an empty Registry ready for use.
func New() *Registry {
	return &Registry{
		agents:     make(map[string]*entity.Agent),
		evictChans: make(map[string]chan struct{}),
	}
}

// Register adds a newly-connected agent to the registry.
//
// On success it returns a receive-only channel that will be closed when the
// agent entry is forcibly evicted (e.g. by a ForceRegister call from a new
// connection with the same caller_id).  The caller's stream loop should select
// on this channel alongside stream.Recv() so that eviction is detected promptly.
//
// If a connected agent with the same caller_id already exists the call returns
// (nil, ErrAlreadyConnected) without modifying the registry.  The caller
// decides the conflict resolution policy (e.g. reject new or evict old via
// ForceRegister).
func (r *Registry) Register(
	callerID string,
	stream grpc.BidiStreamingServer[gatewayv1.AgentStreamMessage, gatewayv1.AgentStreamMessage],
	metadata map[string]any,
) (<-chan struct{}, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.agents[callerID]; ok && existing.Status == entity.StatusConnected {
		return nil, ErrAlreadyConnected
	}

	evictCh := make(chan struct{})
	r.evictChans[callerID] = evictCh

	now := time.Now()
	r.agents[callerID] = &entity.Agent{
		CallerID:    callerID,
		Stream:      stream,
		Metadata:    metadata,
		ConnectedAt: now,
		LastSeenAt:  now,
		Status:      entity.StatusConnected,
	}
	return evictCh, nil
}

// ForceRegister atomically evicts any existing connected agent with the given
// caller_id and registers a new one in its place.
//
// If a connected agent already occupies the caller_id, its evict channel is
// closed (signalling the stream goroutine to terminate) and its registry entry
// is immediately marked as StatusDisconnected before the new entry is written.
// This ensures the new registration never races with Deregister from the old
// stream goroutine.
//
// The returned channel is the evict channel for the newly registered agent; the
// caller should select on it in the stream loop just like after a Register call.
func (r *Registry) ForceRegister(
	callerID string,
	stream grpc.BidiStreamingServer[gatewayv1.AgentStreamMessage, gatewayv1.AgentStreamMessage],
	metadata map[string]any,
) <-chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Evict any existing connected agent.
	if existing, ok := r.agents[callerID]; ok && existing.Status == entity.StatusConnected {
		existing.Status = entity.StatusDisconnected
		existing.Stream = nil
		if ch, ok := r.evictChans[callerID]; ok {
			close(ch)
			delete(r.evictChans, callerID)
		}
	}

	evictCh := make(chan struct{})
	r.evictChans[callerID] = evictCh

	now := time.Now()
	r.agents[callerID] = &entity.Agent{
		CallerID:    callerID,
		Stream:      stream,
		Metadata:    metadata,
		ConnectedAt: now,
		LastSeenAt:  now,
		Status:      entity.StatusConnected,
	}
	return evictCh
}

// Deregister marks an agent as disconnected and clears its stream reference.
// If no agent with the given callerID exists the call is a no-op.
// It also removes the evict channel for the caller_id; if the old handler was
// evicted it will already have set evicted=true and skip calling Deregister,
// so there is no risk of overwriting a fresh registration.
func (r *Registry) Deregister(callerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if a, ok := r.agents[callerID]; ok {
		a.Status = entity.StatusDisconnected
		a.Stream = nil
	}
	delete(r.evictChans, callerID)
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

// GetConnectedStream returns the live bidirectional stream for the agent
// identified by callerID if and only if that agent is currently connected.
//
// The boolean return value is false when the agent is absent, disconnected,
// or its stream reference has already been cleared.  Callers that need to
// send a command to an agent should use this method instead of Get to avoid
// a data race on the Stream field.
func (r *Registry) GetConnectedStream(callerID string) (grpc.BidiStreamingServer[gatewayv1.AgentStreamMessage, gatewayv1.AgentStreamMessage], bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.agents[callerID]
	if !ok || a.Status != entity.StatusConnected || a.Stream == nil {
		return nil, false
	}
	return a.Stream, true
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

// ExpireStale marks as disconnected every connected agent whose LastSeenAt is
// older than ttl.  It returns the caller IDs of every agent that was expired
// so the caller can log or act on them.
//
// This method is the write-side counterpart to TouchHeartbeat: the heartbeat
// monitor calls it periodically to enforce TTL-based disconnection.
// The evict channel for each expired agent is removed from the map but NOT
// closed; TTL expiry is not the same as eviction by a new connection.
func (r *Registry) ExpireStale(ttl time.Duration) []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	var expired []string
	for id, a := range r.agents {
		if a.Status == entity.StatusConnected && now.Sub(a.LastSeenAt) > ttl {
			a.Status = entity.StatusDisconnected
			a.Stream = nil
			// Remove the channel from the map so a subsequent re-registration
			// does not accidentally close a stale channel reference.
			delete(r.evictChans, id)
			expired = append(expired, id)
		}
	}
	return expired
}
