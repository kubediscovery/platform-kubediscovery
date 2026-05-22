// Package agent wires all providers for the agent core domain.
package agent

import (
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-agent/internal/core/agent/executor"
)

// Module is the FX module that provides the agent service and wires the
// connection lifecycle into the application start hook.
var Module = fx.Module("core.agent",
	fx.Provide(newExecutorDispatcher),
	fx.Provide(newAgentService),
	fx.Invoke(startAgentService),
)

// newExecutorDispatcher provides an executor.Dispatcher. When a real
// kd-executor is not wired (current state), the UnavailableDispatcher is used
// so the agent still starts and reports UNAVAILABLE for every command.
func newExecutorDispatcher() executor.Dispatcher {
	return executor.UnavailableDispatcher{}
}
