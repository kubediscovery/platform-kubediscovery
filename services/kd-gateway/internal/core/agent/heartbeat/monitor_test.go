package heartbeat_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/entity"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/heartbeat"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/registry"
)

// newTestMonitor builds a Monitor with a short TTL and interval suitable for
// unit tests.  It returns the monitor and a cancel function that stops the
// Run loop.
func newTestMonitor(reg *registry.Registry, ttl, interval time.Duration) (*heartbeat.Monitor, context.CancelFunc) {
	m := heartbeat.New(reg, ttl, interval, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	go m.Run(ctx)
	return m, cancel
}

func TestMonitor_ExpiresStalAgent(t *testing.T) {
	reg := registry.New()
	_ = reg.Register("stale-agent", nil, nil)

	// Sleep so the agent's LastSeenAt is comfortably in the past.
	time.Sleep(5 * time.Millisecond)

	// TTL=1ms, check every 2ms — the monitor should expire the agent quickly.
	_, cancel := newTestMonitor(reg, time.Millisecond, 2*time.Millisecond)
	defer cancel()

	// Allow several check cycles to complete.
	time.Sleep(20 * time.Millisecond)

	a, ok := reg.Get("stale-agent")
	if !ok {
		t.Fatal("agent entry should still exist after TTL expiry")
	}
	if a.Status != entity.StatusDisconnected {
		t.Errorf("Status = %q after TTL expiry, want StatusDisconnected", a.Status)
	}
}

func TestMonitor_DoesNotExpireFreshAgent(t *testing.T) {
	reg := registry.New()

	// TTL=1s, check every 5ms — fresh agent should survive multiple ticks.
	_, cancel := newTestMonitor(reg, time.Second, 5*time.Millisecond)
	defer cancel()

	// Register *after* the monitor is running so LastSeenAt is recent.
	_ = reg.Register("fresh-agent", nil, nil)

	time.Sleep(30 * time.Millisecond)

	a, ok := reg.Get("fresh-agent")
	if !ok {
		t.Fatal("fresh agent entry should exist")
	}
	if a.Status != entity.StatusConnected {
		t.Errorf("Status = %q, want StatusConnected for fresh agent", a.Status)
	}
}

func TestMonitor_StopsOnContextCancel(t *testing.T) {
	reg := registry.New()
	_ = reg.Register("agent-x", nil, nil)

	// TTL=1ms so it would expire quickly if the monitor were still running.
	_, cancel := newTestMonitor(reg, time.Millisecond, 2*time.Millisecond)

	// Cancel immediately, before any tick fires.
	cancel()

	// Give the goroutine time to notice the cancellation.
	time.Sleep(20 * time.Millisecond)

	// Register a new agent *after* cancel and verify the monitor no longer
	// expires it (it is stopped, so nothing can deregister this agent).
	_ = reg.Register("agent-y", nil, nil)
	time.Sleep(10 * time.Millisecond)

	a, _ := reg.Get("agent-y")
	// agent-y was registered after the monitor stopped, so it should remain
	// connected regardless of TTL.
	if a.Status != entity.StatusConnected {
		t.Errorf("agent registered after cancel: Status = %q, want StatusConnected", a.Status)
	}
}

func TestMonitor_HeartbeatResetsExpiry(t *testing.T) {
	reg := registry.New()
	_ = reg.Register("hb-agent", nil, nil)

	// TTL=10ms, check every 5ms.
	_, cancel := newTestMonitor(reg, 10*time.Millisecond, 5*time.Millisecond)
	defer cancel()

	// Keep touching the heartbeat every 3ms so the agent never goes stale.
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(3 * time.Millisecond)
		defer ticker.Stop()
		deadline := time.After(50 * time.Millisecond)
		for {
			select {
			case <-deadline:
				return
			case <-ticker.C:
				reg.TouchHeartbeat("hb-agent")
			}
		}
	}()
	<-done

	a, _ := reg.Get("hb-agent")
	if a.Status != entity.StatusConnected {
		t.Errorf("agent with active heartbeat: Status = %q, want StatusConnected", a.Status)
	}
}
