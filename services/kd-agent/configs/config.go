// Package configs provides configuration loading for kd-agent via Viper.
package configs

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// Config is the root configuration struct for kd-agent.
type Config struct {
	App  AppConfig  `mapstructure:"app"`
	GRPC GRPCConfig `mapstructure:"grpc"`
}

// AppConfig holds application-level settings.
type AppConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
	LogLevel    string `mapstructure:"log_level"`
	AgentID     string `mapstructure:"agent_id"`
}

// Module is the FX module that provides Config to the application.
var Module = fx.Module("configs",
	fx.Provide(New),
)

// New reads configuration from environment variables and returns a
// fully-populated Config.
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
	v.SetDefault("app.name", "kd-agent")
	v.SetDefault("app.environment", "development")
	v.SetDefault("app.log_level", "info")
	v.SetDefault("app.agent_id", "")

	v.SetDefault("grpc.addr", "localhost:50051")
	v.SetDefault("grpc.ca_file", "")
	v.SetDefault("grpc.client_cert_file", "")
	v.SetDefault("grpc.client_key_file", "")
}
