// Package main boots the kd-agent gRPC client process using Uber Fx modules.
package main

import (
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-agent/configs"
	"github.com/kubediscovery/kd-agent/internal/core"
	"github.com/kubediscovery/kd-agent/internal/infrastructure"
)

func main() {
	fx.New(
		configs.Module,
		infrastructure.Module,
		core.Module,
	).Run()
}
