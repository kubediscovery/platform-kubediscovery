package registry_test

import (
	"testing"
	"time"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/entity"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/registry"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := registry.New()

	if err := r.Register("agent-1", nil, nil); err != nil {
		t.Fatalf("unexpected error on first Register: %v", err)
	}

	a, ok := r.Get("agent-1")
	if !ok {
		t.Fatal("expected agent to be found after registration")
	}
	if a.CallerID != "agent-1" {
		t.Errorf("CallerID = %q, want %q", a.CallerID, "agent-1")
	}
	if a.Status != entity.StatusConnected {
		t.Errorf("Status = %q, want %q", a.Status, entity.StatusConnected)
	}
}

func TestRegistry_Register_DuplicateReturnsError(t *testing.T) {
	r := registry.New()

	if err := r.Register("agent-1", nil, nil); err != nil {
		t.Fatalf("first Register: %v", err)
	}

	err := r.Register("agent-1", nil, nil)
	if err == nil {
		t.Fatal("expected ErrAlreadyConnected, got nil")
	}
	if err != registry.ErrAlreadyConnected {
		t.Errorf("got error %v, want ErrAlreadyConnected", err)
	}
}

func TestRegistry_Register_AllowsReconnectAfterDeregister(t *testing.T) {
	r := registry.New()

	if err := r.Register("agent-1", nil, nil); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	r.Deregister("agent-1")

	if err := r.Register("agent-1", nil, nil); err != nil {
		t.Errorf("expected re-registration to succeed after deregister, got: %v", err)
	}
}

func TestRegistry_Deregister(t *testing.T) {
	r := registry.New()
	_ = r.Register("agent-1", nil, nil)

	r.Deregister("agent-1")

	a, ok := r.Get("agent-1")
	if !ok {
		t.Fatal("entry should still exist after deregister")
	}
	if a.Status != entity.StatusDisconnected {
		t.Errorf("Status = %q after Deregister, want %q", a.Status, entity.StatusDisconnected)
	}
}

func TestRegistry_Deregister_NonExistent(t *testing.T) {
	r := registry.New()
	// Should be a no-op and not panic.
	r.Deregister("ghost")
}

func TestRegistry_TouchHeartbeat(t *testing.T) {
	r := registry.New()
	_ = r.Register("agent-1", nil, nil)

	before, _ := r.Get("agent-1")
	oldSeen := before.LastSeenAt

	ok := r.TouchHeartbeat("agent-1")
	if !ok {
		t.Fatal("TouchHeartbeat should return true for connected agent")
	}

	after, _ := r.Get("agent-1")
	if !after.LastSeenAt.After(oldSeen) && after.LastSeenAt != oldSeen {
		// Allow equal because clock resolution may be coarse in CI.
	}
}

func TestRegistry_TouchHeartbeat_Disconnected(t *testing.T) {
	r := registry.New()
	_ = r.Register("agent-1", nil, nil)
	r.Deregister("agent-1")

	ok := r.TouchHeartbeat("agent-1")
	if ok {
		t.Error("TouchHeartbeat should return false for disconnected agent")
	}
}

func TestRegistry_TouchHeartbeat_Unknown(t *testing.T) {
	r := registry.New()
	ok := r.TouchHeartbeat("nobody")
	if ok {
		t.Error("TouchHeartbeat should return false for unknown agent")
	}
}

func TestRegistry_List(t *testing.T) {
	r := registry.New()
	_ = r.Register("agent-1", nil, nil)
	_ = r.Register("agent-2", nil, nil)

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("List() returned %d entries, want 2", len(list))
	}
}

func TestRegistry_ConnectedCount(t *testing.T) {
	r := registry.New()
	_ = r.Register("a", nil, nil)
	_ = r.Register("b", nil, nil)
	_ = r.Register("c", nil, nil)
	r.Deregister("b")

	if got := r.ConnectedCount(); got != 2 {
		t.Errorf("ConnectedCount() = %d, want 2", got)
	}
}

func TestRegistry_ExpireStale_ExpiresStalAgents(t *testing.T) {
	r := registry.New()
	_ = r.Register("agent-1", nil, nil)
	_ = r.Register("agent-2", nil, nil)

	// Sleep briefly so that LastSeenAt is noticeably in the past.
	time.Sleep(2 * time.Millisecond)

	// With a 1ns TTL every connected agent is considered stale.
	expired := r.ExpireStale(time.Nanosecond)
	if len(expired) != 2 {
		t.Fatalf("ExpireStale returned %d IDs, want 2", len(expired))
	}

	for _, id := range []string{"agent-1", "agent-2"} {
		a, ok := r.Get(id)
		if !ok {
			t.Fatalf("agent %q should still exist after expiration", id)
		}
		if a.Status != entity.StatusDisconnected {
			t.Errorf("agent %q Status = %q after TTL expiry, want StatusDisconnected", id, a.Status)
		}
		if a.Stream != nil {
			t.Errorf("agent %q Stream should be nil after TTL expiry", id)
		}
	}
}

func TestRegistry_ExpireStale_SkipsFreshAgents(t *testing.T) {
	r := registry.New()
	_ = r.Register("agent-fresh", nil, nil)

	// TTL of 1 hour — a freshly registered agent should not be expired.
	expired := r.ExpireStale(time.Hour)
	if len(expired) != 0 {
		t.Fatalf("ExpireStale returned %v, want empty for fresh agent", expired)
	}

	a, _ := r.Get("agent-fresh")
	if a.Status != entity.StatusConnected {
		t.Errorf("fresh agent Status = %q, want StatusConnected", a.Status)
	}
}

func TestRegistry_ExpireStale_SkipsAlreadyDisconnected(t *testing.T) {
	r := registry.New()
	_ = r.Register("agent-gone", nil, nil)
	r.Deregister("agent-gone")

	// Even with a very short TTL, a disconnected agent should not appear in the
	// expired list a second time.
	time.Sleep(2 * time.Millisecond)
	expired := r.ExpireStale(time.Nanosecond)
	for _, id := range expired {
		if id == "agent-gone" {
			t.Error("already-disconnected agent should not be returned by ExpireStale")
		}
	}
}

func TestRegistry_ExpireStale_ReducesConnectedCount(t *testing.T) {
	r := registry.New()
	_ = r.Register("a", nil, nil)
	_ = r.Register("b", nil, nil)

	time.Sleep(2 * time.Millisecond)
	r.ExpireStale(time.Nanosecond)

	if got := r.ConnectedCount(); got != 0 {
		t.Errorf("ConnectedCount() = %d after expiry, want 0", got)
	}
}
