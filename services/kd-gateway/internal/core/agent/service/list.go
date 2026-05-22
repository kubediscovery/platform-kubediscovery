// Package service implements agent domain business logic for HTTP and other callers.
package service

import (
	"sort"
	"time"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/entity"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/registry"
)

// AgentSummary is a read model of one registry entry for API consumers.
type AgentSummary struct {
	CallerID     string
	Status       entity.Status
	Environment  string
	LastActivity time.Time
}

// Service provides agent listing backed by the in-memory registry.
type Service struct {
	registry *registry.Registry
}

// New constructs a Service wired to the given Registry.
func New(reg *registry.Registry) *Service {
	return &Service{registry: reg}
}

// List returns a snapshot of all agents (connected and disconnected), sorted by caller_id.
func (s *Service) List() []AgentSummary {
	agents := s.registry.List()
	out := make([]AgentSummary, 0, len(agents))
	for _, a := range agents {
		out = append(out, AgentSummary{
			CallerID:     a.CallerID,
			Status:       a.Status,
			Environment:  environmentFromMetadata(a.Metadata),
			LastActivity: a.LastSeenAt,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CallerID < out[j].CallerID
	})
	return out
}

// environmentFromMetadata reads the deployment environment from agent hello metadata.
// Agents may send "environment" or "env"; missing keys yield an empty string.
func environmentFromMetadata(md map[string]any) string {
	if len(md) == 0 {
		return ""
	}
	for _, key := range []string{"environment", "env"} {
		v, ok := md[key]
		if !ok {
			continue
		}
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}
