// Package configs provides configuration loading for kd-agent via Viper.
package configs

// GRPCConfig holds all gRPC client settings, including mTLS material paths.
type GRPCConfig struct {
	// Addr is the kd-gateway address to connect to (e.g. "localhost:50051").
	Addr string `mapstructure:"addr"`

	// CAFile is the path to the CA certificate (PEM) used to validate the
	// server certificate.
	CAFile string `mapstructure:"ca_file"`

	// ClientCertFile is the path to the client TLS certificate (PEM).
	// Required for mTLS.
	ClientCertFile string `mapstructure:"client_cert_file"`

	// ClientKeyFile is the path to the client private key (PEM).
	// Required for mTLS.
	ClientKeyFile string `mapstructure:"client_key_file"`

	// ServerName overrides the TLS server name used during verification.
	// When empty the hostname from Addr is used (if it is not an IP).
	ServerName string `mapstructure:"server_name"`

	// InsecureSkipVerify disables TLS server certificate verification.
	// Should only be used in development environments.
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`
}
