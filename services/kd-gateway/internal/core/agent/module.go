// Package agent wires all providers for the agent core domain.
package agent

import (
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/handler"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/heartbeat"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/registry"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/router"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/service"
	"github.com/kubediscovery/kd-gateway/internal/infrastructure/observability"
)

// routerAsSink adapts *router.Router to handler.ResultSink so FX can wire
// the two packages without creating an import cycle.
func routerAsSink(r *router.Router) handler.ResultSink {
	return r
}

// Module is the FX module that provides the in-memory Registry, the command
// Router, the GatewayService gRPC handler, and the heartbeat TTL monitor.
// All lifecycle hooks (start/stop) are registered automatically.
var Module = fx.Module("core.agent",
	fx.Provide(
		fx.Annotate(
			func(r *registry.Registry) observability.ActiveAgentsSource { return r },
			fx.As(new(observability.ActiveAgentsSource)),
		),
	),
	fx.Provide(registry.New),
	fx.Provide(router.New),   // provides *router.Router
	fx.Provide(routerAsSink), // adapts *router.Router → handler.ResultSink
	fx.Provide(handler.New),  // gRPC GatewayService handler
	fx.Provide(service.New),
	fx.Provide(handler.NewHTTP),
	fx.Provide(heartbeat.NewFX),
	fx.Invoke(registerGatewayService),
	fx.Invoke(registerHTTPRoutes),
	// Ensure the Monitor is instantiated even if nothing else depends on it.
	fx.Invoke(func(*heartbeat.Monitor) {}),
)
