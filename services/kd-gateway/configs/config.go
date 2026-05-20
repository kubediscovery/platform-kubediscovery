// Package configs provides configuration loading for kd-gateway via Viper.
package configs

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// Config is the root configuration struct for kd-gateway.
type Config struct {
	App  AppConfig  `mapstructure:"app"`
	GRPC GRPCConfig `mapstructure:"grpc"`
}

// AppConfig holds generic application-level settings.
type AppConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
	LogLevel    string `mapstructure:"log_level"`
}

// Module is the FX module that provides Config to the application.
var Module = fx.Module("configs",
	fx.Provide(New),
)

// New reads configuration from environment variables and returns a
// fully-populated Config.  Viper's AutomaticEnv maps env vars such as
// GRPC_CERT_FILE → grpc.cert_file via the dot-to-underscore replacer.
func New() (*Config, error) {
	v := viper.New()

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	setDefaults(v)

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("configs: unmarshal: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "kd-gateway")
	v.SetDefault("app.environment", "development")
	v.SetDefault("app.log_level", "info")

	v.SetDefault("grpc.addr", "0.0.0.0:50051")
	v.SetDefault("grpc.cert_file", "")
	v.SetDefault("grpc.key_file", "")
	v.SetDefault("grpc.mtls", true)
	v.SetDefault("grpc.client_ca_file", "~/.kubediscovery/certs/staging/ca.crt")
	v.SetDefault("grpc.debug", false)
}
