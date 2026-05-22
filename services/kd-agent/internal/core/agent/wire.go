package agent

import (
	"context"
	"fmt"
	"log/slog"

	"go.uber.org/fx"

	"github.com/kubediscovery/kd-agent/configs"
	"github.com/kubediscovery/kd-agent/internal/core/agent/executor"
	"github.com/kubediscovery/kd-agent/internal/core/agent/service"
	grpcclient "github.com/kubediscovery/kd-agent/internal/infrastructure/grpc"
)

type serviceParams struct {
	fx.In

	Config   *configs.Config
	Dispatch executor.Dispatcher
	Log      *slog.Logger
}

// newAgentService constructs the service.Service from FX-provided dependencies.
func newAgentService(p serviceParams) (*service.Service, error) {
	agentID := p.Config.App.AgentID
	if agentID == "" {
		return nil, fmt.Errorf("AGENT_ID env var must not be empty")
	}

	opener := grpcclient.NewOpener(p.Config.GRPC, p.Log)

	return service.New(agentID, service.DefaultRetryConfig(), opener, p.Dispatch, p.Log)
}

type invokeParams struct {
	fx.In

	LC  fx.Lifecycle
	Svc *service.Service
	Log *slog.Logger
}

// startAgentService registers the FX lifecycle hook that runs the agent
// connection loop in a background goroutine.
func startAgentService(p invokeParams) {
	p.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := p.Svc.Run(context.Background()); err != nil {
					p.Log.Error("agent run ended with error", slog.Any("error", err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return nil
		},
	})
}
