// Package configs provides configuration loading for kd-gateway via Viper.
package configs

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// DuplicatePolicy controls what happens when an agent attempts to connect with
// a caller_id that is already registered as connected.
type DuplicatePolicy string

const (
	// DuplicatePolicyRejectNew rejects the incoming stream with codes.AlreadyExists.
	// This is the default and safest policy.
	DuplicatePolicyRejectNew DuplicatePolicy = "reject_new"

	// DuplicatePolicyEvictPrevious terminates the existing stream and accepts the
	// new one.  The old stream receives codes.Aborted so the agent can reconnect.
	DuplicatePolicyEvictPrevious DuplicatePolicy = "evict_previous"
)

// Config is the root configuration struct for kd-gateway.
type Config struct {
	App       AppConfig       `mapstructure:"app"`
	GRPC      GRPCConfig      `mapstructure:"grpc"`
	HTTP      HTTPConfig      `mapstructure:"http"`
	Heartbeat HeartbeatConfig `mapstructure:"heartbeat"`
	Agent     AgentConfig     `mapstructure:"agent"`
}

// AgentConfig holds settings that govern connected agent behaviour.
type AgentConfig struct {
	// DuplicatePolicy defines what the gateway does when a second connection
	// arrives with the same caller_id as an already-connected agent.
	// Valid values: "reject_new" (default) | "evict_previous".
	DuplicatePolicy DuplicatePolicy `mapstructure:"duplicate_policy"`
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

	v.SetDefault("http.addr", "0.0.0.0:8080")
	v.SetDefault("http.read_header_timeout", 10*time.Second)
	v.SetDefault("http.read_timeout", 30*time.Second)
	v.SetDefault("http.write_timeout", 30*time.Second)
	v.SetDefault("http.idle_timeout", 120*time.Second)
	v.SetDefault("http.trusted_proxies", []string{"127.0.0.1"})

	v.SetDefault("heartbeat.ttl", 30*time.Second)
	v.SetDefault("heartbeat.check_interval", 10*time.Second)

	v.SetDefault("agent.duplicate_policy", string(DuplicatePolicyRejectNew))
}
