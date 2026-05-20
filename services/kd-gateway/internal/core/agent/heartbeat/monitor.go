// Package heartbeat implements TTL-based disconnection detection for agents.
//
// The Monitor runs a background goroutine that periodically scans the
// in-memory registry for agents whose last heartbeat is older than the
// configured TTL.  When an agent's TTL expires the Monitor calls
// registry.ExpireStale, which atomically marks the agent as
// StatusDisconnected and clears its stream reference.
//
// Configuration (environment variables, Viper key → env var):
//
//	heartbeat.ttl            → HEARTBEAT_TTL            (default: 30s)
//	heartbeat.check_interval → HEARTBEAT_CHECK_INTERVAL (default: 10s)
package heartbeat

import (
	"context"
	"log/slog"
	"time"

	"go.uber.org/fx"

	"github.com/kubediscovery/kd-gateway/configs"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/registry"
)

// Monitor periodically scans the registry for agents whose heartbeat TTL has
// expired and marks them as disconnected.
type Monitor struct {
	reg      *registry.Registry
	ttl      time.Duration
	interval time.Duration
	log      *slog.Logger
}

// New returns a Monitor configured with explicit parameters.
// It is the preferred constructor for unit tests; production code uses NewFX.
func New(reg *registry.Registry, ttl, interval time.Duration, log *slog.Logger) *Monitor {
	return &Monitor{
		reg:      reg,
		ttl:      ttl,
		interval: interval,
		log:      log,
	}
}

// Params groups the FX-injected dependencies required by the Monitor.
type Params struct {
	fx.In

	LC       fx.Lifecycle
	Registry *registry.Registry
	Config   *configs.Config
	Log      *slog.Logger
}

// NewFX is the FX constructor for Monitor.  It creates the monitor, registers
// OnStart/OnStop lifecycle hooks, and returns the monitor so other FX
// components can depend on it if needed.
func NewFX(p Params) *Monitor {
	m := New(
		p.Registry,
		p.Config.Heartbeat.TTL,
		p.Config.Heartbeat.CheckInterval,
		p.Log,
	)

	ctx, cancel := context.WithCancel(context.Background())

	p.LC.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			p.Log.Info("heartbeat monitor starting",
				slog.Duration("ttl", p.Config.Heartbeat.TTL),
				slog.Duration("check_interval", p.Config.Heartbeat.CheckInterval),
			)
			go m.Run(ctx)
			return nil
		},
		OnStop: func(_ context.Context) error {
			cancel()
			p.Log.Info("heartbeat monitor stopped")
			return nil
		},
	})

	return m
}

// Run starts the monitoring loop and blocks until ctx is cancelled.
// Production code calls this via the FX lifecycle; tests may call it directly.
func (m *Monitor) Run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.expireStale()
		}
	}
}

// expireStale delegates to the registry and logs every agent that was expired.
func (m *Monitor) expireStale() {
	expired := m.reg.ExpireStale(m.ttl)
	for _, id := range expired {
		m.log.Warn("agent heartbeat TTL expired, marking as disconnected",
			slog.String("caller_id", id),
			slog.Duration("ttl", m.ttl),
		)
	}
}
