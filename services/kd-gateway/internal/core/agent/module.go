// Package agent wires all providers for the agent core domain.
package agent

import (
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/handler"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/heartbeat"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/registry"
)

// Module is the FX module that provides the in-memory Registry, the
// GatewayService gRPC handler, and the heartbeat TTL monitor.
// The monitor is registered via its FX lifecycle hooks so it starts and stops
// automatically with the application.
var Module = fx.Module("core.agent",
	fx.Provide(registry.New),
	fx.Provide(handler.New),
	fx.Provide(heartbeat.NewFX),
	fx.Invoke(registerGatewayService),
	// Invoke NewFX indirectly by declaring the Monitor as a dependency so FX
	// actually instantiates it; without this the provide would be pruned.
	fx.Invoke(func(*heartbeat.Monitor) {}),
)
