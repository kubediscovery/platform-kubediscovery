// Package core wires all domain modules for kd-agent.
package core

import (
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-agent/internal/core/agent"
)

// Module is the FX module that registers every core domain sub-module.
var Module = fx.Module("core",
	agent.Module,
)
