// Package core wires all domain (core) modules for kd-gateway.
package core

import (
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-gateway/internal/core/agent"
)

// Module is the FX module that registers every core domain sub-module.
var Module = fx.Module("core",
	agent.Module,
)
