package service_test

import (
	"testing"
	"time"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/entity"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/registry"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/service"
)

func TestService_List_EmptyRegistry(t *testing.T) {
	svc := service.New(registry.New())
	list := svc.List()
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d items", len(list))
	}
}

func TestService_List_ReturnsAgentsWithEnvironment(t *testing.T) {
	reg := registry.New()
	lastSeen := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)

	if _, err := reg.Register("agent-b", nil, map[string]any{"env": "staging"}); err != nil {
		t.Fatalf("register agent-b: %v", err)
	}
	if _, err := reg.Register("agent-a", nil, map[string]any{"environment": "production"}); err != nil {
		t.Fatalf("register agent-a: %v", err)
	}

	if a, ok := reg.Get("agent-a"); ok {
		a.LastSeenAt = lastSeen
	}
	if a, ok := reg.Get("agent-b"); ok {
		a.LastSeenAt = lastSeen.Add(time.Minute)
	}

	svc := service.New(reg)
	list := svc.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(list))
	}

	// Sorted by caller_id.
	if list[0].CallerID != "agent-a" || list[0].Environment != "production" {
		t.Errorf("first agent: %+v", list[0])
	}
	if list[0].Status != entity.StatusConnected {
		t.Errorf("agent-a status = %q, want connected", list[0].Status)
	}
	if !list[0].LastActivity.Equal(lastSeen) {
		t.Errorf("agent-a last activity = %v, want %v", list[0].LastActivity, lastSeen)
	}

	if list[1].CallerID != "agent-b" || list[1].Environment != "staging" {
		t.Errorf("second agent: %+v", list[1])
	}
}

func TestService_List_IncludesDisconnected(t *testing.T) {
	reg := registry.New()
	if _, err := reg.Register("agent-1", nil, nil); err != nil {
		t.Fatalf("register: %v", err)
	}
	reg.Deregister("agent-1")

	list := service.New(reg).List()
	if len(list) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(list))
	}
	if list[0].Status != entity.StatusDisconnected {
		t.Errorf("status = %q, want disconnected", list[0].Status)
	}
}
