package agent

import (
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-gateway/internal/core/agent/handler"
	httpserver "github.com/kubediscovery/kd-gateway/internal/infrastructure/http"
)

type registerHTTPParams struct {
	fx.In

	Server  *httpserver.Server
	Handler *handler.HTTPHandler
}

// registerHTTPRoutes wires agent REST endpoints onto the shared Gin engine.
func registerHTTPRoutes(p registerHTTPParams) {
	v1 := p.Server.Engine().Group("/api/v1")
	v1.GET("/agents", p.Handler.ListAgents)
}
