// Package configs provides configuration loading for kd-gateway via Viper.
package configs

// GRPCConfig holds all gRPC server settings, including mTLS material paths.
type GRPCConfig struct {
	// Addr is the listen address for the gRPC server (e.g. "0.0.0.0:50051").
	Addr string `mapstructure:"addr"`

	// CertFile is the path to the server TLS certificate (PEM).
	CertFile string `mapstructure:"cert_file"`

	// KeyFile is the path to the server private key (PEM).
	KeyFile string `mapstructure:"key_file"`

	// MTLS controls whether mutual TLS is enforced.
	// When true the server loads ClientCAFile and sets
	// tls.RequireAndVerifyClientCert, so every connecting agent must
	// present a certificate signed by that CA.
	MTLS bool `mapstructure:"mtls"`

	// ClientCAFile is the path to the CA certificate (PEM) used to validate
	// client certificates.  Required when MTLS is true.
	ClientCAFile string `mapstructure:"client_ca_file"`

	// Debug enables per-call duration interceptors when set to true.
	// Corresponds to the GRPC_DEBUG environment variable.
	Debug bool `mapstructure:"debug"`
}
