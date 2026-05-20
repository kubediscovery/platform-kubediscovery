// Package infrastructure wires all infrastructure-layer providers for kd-store.
package infrastructure

import (
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-store/internal/infrastructure/cache"
	"github.com/kubediscovery/kd-store/internal/infrastructure/database"
)

// Module is the FX module that registers all infrastructure providers.
// NewMigrator is provided before NewPool so migrations run before the
// connection pool starts serving application queries.
var Module = fx.Module("infrastructure",
	fx.Provide(database.NewMigrator),
	fx.Provide(database.NewPool),
	fx.Provide(cache.NewClient),
)
