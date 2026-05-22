// Package main boots the kd-agent gRPC process using Uber Fx modules.
package main

import (
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-agent/configs"
)

func main() {
	fx.New(
		configs.Module,
	).Run()
}
