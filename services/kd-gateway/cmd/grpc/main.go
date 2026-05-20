// Package main boots the kd-gateway gRPC process using Uber Fx modules.
package main

import (
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-gateway/configs"
	"github.com/kubediscovery/kd-gateway/internal/infrastructure"
)

func main() {
	fx.New(
		configs.Module,
		infrastructure.Module,
	).Run()
}
