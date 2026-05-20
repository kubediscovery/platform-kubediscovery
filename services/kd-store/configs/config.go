package configs

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// Config is the root configuration struct for kd-store.
type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Database DatabaseConfig `mapstructure:"database"`
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

// New reads configuration from environment variables (and optionally a file)
// and returns a fully-populated Config.
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
	v.SetDefault("app.name", "kd-store")
	v.SetDefault("app.environment", "development")
	v.SetDefault("app.log_level", "info")

	// Provide empty-string defaults so Viper registers the keys and picks up
	// their values from environment variables via AutomaticEnv.
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.name", "")
	v.SetDefault("database.user", "")
	v.SetDefault("database.password", "")
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.max_conns", 10)
	v.SetDefault("database.min_conns", 2)
	v.SetDefault("database.max_conn_lifetime", "1h")
	v.SetDefault("database.max_conn_idle_time", "30m")
	v.SetDefault("database.health_check_period", "1m")
}
