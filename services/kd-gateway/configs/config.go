// Package configs provides configuration loading for kd-gateway via Viper.
package configs

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// Config is the root configuration struct for kd-gateway.
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	GRPC      GRPCConfig      `mapstructure:"grpc"`
	Heartbeat HeartbeatConfig `mapstructure:"heartbeat"`
}

// AppConfig holds generic application-level settings.
type AppConfig struct {
	Name        string `mapstructure:"name"`
	Environment string `mapstructure:"environment"`
	LogLevel    string `mapstructure:"log_level"`
}

// HeartbeatConfig controls the TTL-based disconnection detection for agents.
type HeartbeatConfig struct {
	// TTL is how long an agent can go without sending a heartbeat before it is
	// marked as disconnected.  Defaults to 30s.
	TTL time.Duration `mapstructure:"ttl"`

	// CheckInterval is how often the background monitor scans for stale agents.
	// Defaults to 10s.
	CheckInterval time.Duration `mapstructure:"check_interval"`
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

	v.SetDefault("heartbeat.ttl", 30*time.Second)
	v.SetDefault("heartbeat.check_interval", 10*time.Second)
}
