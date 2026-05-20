package configs

// GRPCConfig holds gRPC client settings for connecting to kd-gateway.
type GRPCConfig struct {
	// Addr is the kd-gateway address (e.g. "gateway.example.com:50051").
	Addr string `mapstructure:"addr"`

	// CAFile is the path to the CA certificate (PEM) used to validate the
	// gateway's server certificate.
	CAFile string `mapstructure:"ca_file"`

	// ClientCertFile is the path to the client TLS certificate (PEM) used for
	// mTLS authentication with the gateway.
	ClientCertFile string `mapstructure:"client_cert_file"`

	// ClientKeyFile is the path to the client private key (PEM).
	ClientKeyFile string `mapstructure:"client_key_file"`
}
