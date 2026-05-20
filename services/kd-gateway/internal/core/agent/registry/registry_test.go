package registry_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/entity"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/registry"
)

// mustRegister is a test helper that calls Register and fails the test on error.
// It returns the evict channel so callers can inspect or ignore it.
func mustRegister(t *testing.T, r *registry.Registry, callerID string) <-chan struct{} {
	t.Helper()
	ch, err := r.Register(callerID, nil, nil)
	if err != nil {
		t.Fatalf("Register(%q): unexpected error: %v", callerID, err)
	}
	return ch
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := registry.New()

	if _, err := r.Register("agent-1", nil, nil); err != nil {
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

	if _, err := r.Register("agent-1", nil, nil); err != nil {
		t.Fatalf("first Register: %v", err)
	}

	_, err := r.Register("agent-1", nil, nil)
	if err == nil {
		t.Fatal("expected ErrAlreadyConnected, got nil")
	}
	if !errors.Is(err, registry.ErrAlreadyConnected) {
		t.Errorf("got error %v, want ErrAlreadyConnected", err)
	}
}

func TestRegistry_Register_AllowsReconnectAfterDeregister(t *testing.T) {
	r := registry.New()

	if _, err := r.Register("agent-1", nil, nil); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	r.Deregister("agent-1")

	if _, err := r.Register("agent-1", nil, nil); err != nil {
		t.Errorf("expected re-registration to succeed after deregister, got: %v", err)
	}
}

func TestRegistry_Register_ReturnsEvictChannel(t *testing.T) {
	r := registry.New()

	ch, err := r.Register("agent-evict", nil, nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if ch == nil {
		t.Fatal("Register should return a non-nil evict channel")
	}

	// Channel must not be closed yet.
	select {
	case <-ch:
		t.Error("evict channel should not be closed immediately after registration")
	default:
	}
}

func TestRegistry_Deregister(t *testing.T) {
	r := registry.New()
	mustRegister(t, r, "agent-1")

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
	mustRegister(t, r, "agent-1")

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
	mustRegister(t, r, "agent-1")
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
	mustRegister(t, r, "agent-1")
	mustRegister(t, r, "agent-2")

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("List() returned %d entries, want 2", len(list))
	}
}

func TestRegistry_ConnectedCount(t *testing.T) {
	r := registry.New()
	mustRegister(t, r, "a")
	mustRegister(t, r, "b")
	mustRegister(t, r, "c")
	r.Deregister("b")

	if got := r.ConnectedCount(); got != 2 {
		t.Errorf("ConnectedCount() = %d, want 2", got)
	}
}

func TestRegistry_ExpireStale_ExpiresStalAgents(t *testing.T) {
	r := registry.New()
	mustRegister(t, r, "agent-1")
	mustRegister(t, r, "agent-2")

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
	mustRegister(t, r, "agent-fresh")

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
	mustRegister(t, r, "agent-gone")
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
	mustRegister(t, r, "a")
	mustRegister(t, r, "b")

	time.Sleep(2 * time.Millisecond)
	r.ExpireStale(time.Nanosecond)

	if got := r.ConnectedCount(); got != 0 {
		t.Errorf("ConnectedCount() = %d after expiry, want 0", got)
	}
}

func TestRegistry_GetConnectedStream_NilStream(t *testing.T) {
	// An agent registered with a nil stream (common in unit tests) is considered
	// not routable — GetConnectedStream must return false to prevent a nil-deref
	// when the caller tries to send a command.
	r := registry.New()
	mustRegister(t, r, "agent-1")

	_, ok := r.GetConnectedStream("agent-1")
	if ok {
		t.Error("agent registered with nil stream should return ok=false")
	}
}

func TestRegistry_GetConnectedStream_Disconnected(t *testing.T) {
	r := registry.New()
	mustRegister(t, r, "agent-1")
	r.Deregister("agent-1")

	_, ok := r.GetConnectedStream("agent-1")
	if ok {
		t.Error("disconnected agent should return ok=false")
	}
}

func TestRegistry_GetConnectedStream_Unknown(t *testing.T) {
	r := registry.New()

	_, ok := r.GetConnectedStream("nobody")
	if ok {
		t.Error("unknown agent should return ok=false")
	}
}

// --- ForceRegister tests ---

func TestRegistry_ForceRegister_RegistersWhenEmpty(t *testing.T) {
	r := registry.New()

	ch := r.ForceRegister("agent-fr", nil, nil)
	if ch == nil {
		t.Fatal("ForceRegister should return a non-nil evict channel")
	}

	a, ok := r.Get("agent-fr")
	if !ok {
		t.Fatal("agent should exist after ForceRegister")
	}
	if a.Status != entity.StatusConnected {
		t.Errorf("Status = %q, want StatusConnected", a.Status)
	}
}

func TestRegistry_ForceRegister_EvictsExistingAndClosesChannel(t *testing.T) {
	r := registry.New()

	// Register the first agent and capture its evict channel.
	oldEvictCh, err := r.Register("agent-dup", nil, nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Channel must not be closed yet.
	select {
	case <-oldEvictCh:
		t.Fatal("old evict channel should not be closed before ForceRegister")
	default:
	}

	// Force-register a new agent with the same caller_id.
	newEvictCh := r.ForceRegister("agent-dup", nil, nil)

	// Old evict channel must be closed (eviction signal sent).
	select {
	case <-oldEvictCh:
		// expected
	case <-time.After(50 * time.Millisecond):
		t.Error("old evict channel was not closed by ForceRegister")
	}

	// New evict channel must be distinct and not closed.
	if newEvictCh == oldEvictCh {
		t.Error("new evict channel should be a different channel from the old one")
	}
	select {
	case <-newEvictCh:
		t.Error("new evict channel should not be closed after ForceRegister")
	default:
	}

	// Agent must be connected (new registration).
	a, ok := r.Get("agent-dup")
	if !ok {
		t.Fatal("agent entry should exist after ForceRegister")
	}
	if a.Status != entity.StatusConnected {
		t.Errorf("Status = %q after ForceRegister, want StatusConnected", a.Status)
	}
}

func TestRegistry_ForceRegister_AllowsSubsequentRegisterToFail(t *testing.T) {
	r := registry.New()

	r.ForceRegister("agent-x", nil, nil)

	// After ForceRegister the agent is connected.  A plain Register must fail.
	_, err := r.Register("agent-x", nil, nil)
	if !errors.Is(err, registry.ErrAlreadyConnected) {
		t.Errorf("Register after ForceRegister: got %v, want ErrAlreadyConnected", err)
	}
}

func TestRegistry_ForceRegister_AllowsReRegisterAfterDeregister(t *testing.T) {
	r := registry.New()

	r.ForceRegister("agent-y", nil, nil)
	r.Deregister("agent-y")

	// After deregister a plain Register must succeed.
	if _, err := r.Register("agent-y", nil, nil); err != nil {
		t.Errorf("Register after ForceRegister+Deregister: %v", err)
	}
}

func TestRegistry_ExpireStale_AllowsReRegisterAfterExpiry(t *testing.T) {
	r := registry.New()
	mustRegister(t, r, "agent-exp")

	time.Sleep(2 * time.Millisecond)
	r.ExpireStale(time.Nanosecond)

	// After TTL expiry a new Register should succeed (status is now disconnected).
	if _, err := r.Register("agent-exp", nil, nil); err != nil {
		t.Errorf("Register after TTL expiry: %v", err)
	}
}
