// Package agent wires all providers for the agent core domain.
package agent

import (
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/handler"
	"github.com/kubediscovery/kd-gateway/internal/core/agent/registry"
)

// Module is the FX module that provides the in-memory Registry and the
// GatewayService gRPC handler, and registers the handler against the
// gRPC server on application start.
var Module = fx.Module("core.agent",
	fx.Provide(registry.New),
	fx.Provide(handler.New),
	fx.Invoke(registerGatewayService),
)
