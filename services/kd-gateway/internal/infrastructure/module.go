// Package infrastructure wires all infrastructure-layer providers for kd-gateway.
package infrastructure

import (
	"go.uber.org/fx"

	grpcserver "github.com/kubediscovery/kd-gateway/internal/infrastructure/grpc"
	httpserver "github.com/kubediscovery/kd-gateway/internal/infrastructure/http"
	"github.com/kubediscovery/kd-gateway/internal/infrastructure/observability"
)

// Module is the FX module that registers all infrastructure providers.
var Module = fx.Module("infrastructure",
	observability.Module,
	fx.Provide(grpcserver.New),
	fx.Provide(httpserver.New),
)
