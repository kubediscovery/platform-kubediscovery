// Package infrastructure wires all infrastructure-layer providers for kd-store.
package infrastructure

import (
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-store/internal/infrastructure/database"
)

// Module is the FX module that registers all infrastructure providers.
var Module = fx.Module("infrastructure",
	fx.Provide(database.NewPool),
)
