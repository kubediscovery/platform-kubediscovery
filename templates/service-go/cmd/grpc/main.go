package main

import (
	"go.uber.org/fx"

	"github.com/kubediscovery/example-service/configs"
	core "github.com/kubediscovery/example-service/internal/core/example"
	"github.com/kubediscovery/example-service/internal/infrastructure"
)

func main() {
	fx.New(
		configs.Module,
		infrastructure.Module,
		core.Module,
	).Run()
}
