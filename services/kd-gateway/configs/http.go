// Package configs provides configuration loading for kd-gateway via Viper.
package configs

import "time"

// HTTPConfig holds all HTTP server settings for the REST API.
type HTTPConfig struct {
	// Addr is the listen address for the HTTP server (e.g. "0.0.0.0:8080").
	// Must be different from the gRPC address.
	Addr string `mapstructure:"addr"`

	// ReadHeaderTimeout guards against Slowloris attacks (CWE-400).
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`

	// ReadTimeout is the maximum duration for reading the entire request.
	ReadTimeout time.Duration `mapstructure:"read_timeout"`

	// WriteTimeout is the maximum duration before the response times out.
	WriteTimeout time.Duration `mapstructure:"write_timeout"`

	// IdleTimeout is the maximum time to wait for the next request when
	// keep-alive connections are enabled.
	IdleTimeout time.Duration `mapstructure:"idle_timeout"`

	// TrustedProxies is the list of CIDR ranges trusted as reverse proxies.
	// Used to correctly extract the client IP from X-Forwarded-For headers.
	TrustedProxies []string `mapstructure:"trusted_proxies"`
}
