package configs

import "time"

// ObservabilityConfig holds metrics/tracing configuration.
type ObservabilityConfig struct {
	ServiceName    string     `mapstructure:"service_name"`
	ServiceVersion string     `mapstructure:"service_version"`
	Environment    string     `mapstructure:"environment"`
	OTLP           OTLPConfig `mapstructure:"otlp"`
}

// OTLPConfig holds OpenTelemetry OTLP exporter configuration.
type OTLPConfig struct {
	Endpoint string        `mapstructure:"endpoint"`
	Insecure bool          `mapstructure:"insecure"`
	Timeout  time.Duration `mapstructure:"timeout"`
}
