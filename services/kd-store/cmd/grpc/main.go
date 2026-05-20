// Package main boots the kd-store gRPC process using Uber Fx modules.
package main

import (
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-store/configs"
	"github.com/kubediscovery/kd-store/internal/infrastructure"
)

func main() {
	fx.New(
		configs.Module,
		infrastructure.Module,
	).Run()
}
