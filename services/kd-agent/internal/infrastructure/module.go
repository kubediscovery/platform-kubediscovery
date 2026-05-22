// Package infrastructure wires all infrastructure-layer providers for kd-agent.
package infrastructure

import (
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-agent/internal/infrastructure/observability"
)

// Module is the FX module that registers all infrastructure providers.
var Module = fx.Module("infrastructure",
	observability.Module,
)
