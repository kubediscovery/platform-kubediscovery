package agent

import (
	gatewayv1 "github.com/kubediscovery/kd-libs/core/v1/gateway"
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/handler"
	grpcserver "github.com/kubediscovery/kd-gateway/internal/infrastructure/grpc"
)

type registerParams struct {
	fx.In

	Server  *grpcserver.Server
	Handler *handler.Handler
}

// registerGatewayService wires the Handler into the shared *grpc.Server.
// FX calls this as an Invoke, so it runs during application start.
func registerGatewayService(p registerParams) {
	gatewayv1.RegisterGatewayServiceServer(p.Server.GRPCServer(), p.Handler)
}
