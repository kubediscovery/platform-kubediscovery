package grpc_test

import (
	"testing"

	"github.com/kubediscovery/kd-agent/configs"
	grpcclient "github.com/kubediscovery/kd-agent/internal/infrastructure/grpc"
)

func TestBuildCredentials_InsecureSkipVerify(t *testing.T) {
	cfg := configs.GRPCConfig{
		Addr:               "localhost:50051",
		InsecureSkipVerify: true,
	}

	creds, err := grpcclient.BuildCredentials(cfg)
	if err != nil {
		t.Fatalf("BuildCredentials error: %v", err)
	}
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
	// Insecure credentials have info "insecure".
	if creds.Info().SecurityProtocol != "insecure" {
		t.Errorf("SecurityProtocol = %q, want %q", creds.Info().SecurityProtocol, "insecure")
	}
}

func TestBuildCredentials_TLS_NoFiles(t *testing.T) {
	// When no cert files are provided, BuildCredentials should return TLS
	// credentials (not insecure) without error.
	cfg := configs.GRPCConfig{
		Addr:               "gateway.example.com:50051",
		InsecureSkipVerify: false,
	}

	creds, err := grpcclient.BuildCredentials(cfg)
	if err != nil {
		t.Fatalf("BuildCredentials error: %v", err)
	}
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
	if creds.Info().SecurityProtocol != "tls" {
		t.Errorf("SecurityProtocol = %q, want %q", creds.Info().SecurityProtocol, "tls")
	}
}

func TestBuildCredentials_MissingCAFile_ReturnsError(t *testing.T) {
	cfg := configs.GRPCConfig{
		Addr:   "gateway.example.com:50051",
		CAFile: "/nonexistent/ca.crt",
	}

	_, err := grpcclient.BuildCredentials(cfg)
	if err == nil {
		t.Fatal("expected error when CA file does not exist, got nil")
	}
}

func TestBuildCredentials_MissingClientCert_ReturnsError(t *testing.T) {
	cfg := configs.GRPCConfig{
		Addr:           "gateway.example.com:50051",
		ClientCertFile: "/nonexistent/client.crt",
		ClientKeyFile:  "/nonexistent/client.key",
	}

	_, err := grpcclient.BuildCredentials(cfg)
	if err == nil {
		t.Fatal("expected error when client cert file does not exist, got nil")
	}
}

func TestResolveServerName_ExplicitOverride(t *testing.T) {
	// ResolveServerName is tested indirectly via BuildCredentials by checking
	// that credentials are returned without error when a server name is set.
	cfg := configs.GRPCConfig{
		Addr:               "10.0.0.1:50051",
		ServerName:         "gateway.cluster.local",
		InsecureSkipVerify: false,
	}

	creds, err := grpcclient.BuildCredentials(cfg)
	if err != nil {
		t.Fatalf("BuildCredentials error: %v", err)
	}
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
}
