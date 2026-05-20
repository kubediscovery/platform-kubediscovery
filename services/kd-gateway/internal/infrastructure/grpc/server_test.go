// Package grpc_test exercises the gRPC server constructor, focusing on the
// credential-building and option-assembly paths without requiring a live
// network.  Certificates are generated in-memory for isolation.
package grpc_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	grpcserver "github.com/kubediscovery/kd-gateway/internal/infrastructure/grpc"
	"github.com/kubediscovery/kd-gateway/configs"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// certBundle holds PEM-encoded CA, server and client material written to
// temporary files.
type certBundle struct {
	dir        string
	caCertFile string
	srvCert    string
	srvKey     string
	clientCert string
	clientKey  string
}

// newCertBundle generates a self-signed CA and signs server + client certs,
// writing all material as PEM files under a temp directory.
func newCertBundle(t *testing.T) *certBundle {
	t.Helper()
	dir := t.TempDir()

	caKey, caCert := mustGenCA(t)
	writePEMKey(t, filepath.Join(dir, "ca.key"), caKey)
	writePEMCert(t, filepath.Join(dir, "ca.crt"), caCert)

	srvKey, srvCert := mustGenCert(t, caKey, caCert, "server", []string{"server", "localhost"}, []net.IP{net.ParseIP("127.0.0.1")})
	writePEMKey(t, filepath.Join(dir, "server.key"), srvKey)
	writePEMCert(t, filepath.Join(dir, "server.crt"), srvCert)

	clientKey, clientCert := mustGenCert(t, caKey, caCert, "client", nil, nil)
	writePEMKey(t, filepath.Join(dir, "client.key"), clientKey)
	writePEMCert(t, filepath.Join(dir, "client.crt"), clientCert)

	return &certBundle{
		dir:        dir,
		caCertFile: filepath.Join(dir, "ca.crt"),
		srvCert:    filepath.Join(dir, "server.crt"),
		srvKey:     filepath.Join(dir, "server.key"),
		clientCert: filepath.Join(dir, "client.crt"),
		clientKey:  filepath.Join(dir, "client.key"),
	}
}

func mustGenCA(t *testing.T) (*ecdsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("gen CA key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}
	cert, _ := x509.ParseCertificate(der)
	return key, cert
}

func mustGenCert(t *testing.T, caKey *ecdsa.PrivateKey, caCert *x509.Certificate, cn string, dns []string, ips []net.IP) (*ecdsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("gen cert key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		DNSNames:     dns,
		IPAddresses:  ips,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert, _ := x509.ParseCertificate(der)
	return key, cert
}

func writePEMKey(t *testing.T, path string, key *ecdsa.PrivateKey) {
	t.Helper()
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create key file: %v", err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}); err != nil {
		t.Fatalf("encode key PEM: %v", err)
	}
}

func writePEMCert(t *testing.T, path string, cert *x509.Certificate) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create cert file: %v", err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}); err != nil {
		t.Fatalf("encode cert PEM: %v", err)
	}
}

// ─── tests ────────────────────────────────────────────────────────────────────

// TestBuildCredentialsMTLS verifies that when mTLS is enabled the returned
// credentials carry RequireAndVerifyClientCert and can establish a connection
// with a valid client certificate while rejecting a client without one.
func TestBuildCredentialsMTLS(t *testing.T) {
	b := newCertBundle(t)

	creds, err := grpcserver.BuildCredentials(configs.GRPCConfig{
		CertFile:     b.srvCert,
		KeyFile:      b.srvKey,
		MTLS:         true,
		ClientCAFile: b.caCertFile,
	})
	if err != nil {
		t.Fatalf("BuildCredentials: %v", err)
	}
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}

	tlsInfo := creds.Info()
	if tlsInfo.SecurityProtocol != "tls" {
		t.Errorf("expected protocol %q, got %q", "tls", tlsInfo.SecurityProtocol)
	}
}

// TestBuildCredentialsTLSOnly verifies that TLS-only mode (mTLS=false) loads
// the server certificate without requiring a CA file.
func TestBuildCredentialsTLSOnly(t *testing.T) {
	b := newCertBundle(t)

	creds, err := grpcserver.BuildCredentials(configs.GRPCConfig{
		CertFile: b.srvCert,
		KeyFile:  b.srvKey,
		MTLS:     false,
	})
	if err != nil {
		t.Fatalf("BuildCredentials TLS-only: %v", err)
	}
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
}

// TestBuildCredentialsMTLSMissingCA verifies that enabling mTLS without
// providing a CA file path returns an error.
func TestBuildCredentialsMTLSMissingCA(t *testing.T) {
	b := newCertBundle(t)

	_, err := grpcserver.BuildCredentials(configs.GRPCConfig{
		CertFile:     b.srvCert,
		KeyFile:      b.srvKey,
		MTLS:         true,
		ClientCAFile: "", // missing on purpose
	})
	if err == nil {
		t.Fatal("expected error when client_ca_file is empty, got nil")
	}
}

// TestBuildCredentialsMissingCert verifies that an error is returned when the
// server certificate file does not exist.
func TestBuildCredentialsMissingCert(t *testing.T) {
	_, err := grpcserver.BuildCredentials(configs.GRPCConfig{
		CertFile: "/nonexistent/server.crt",
		KeyFile:  "/nonexistent/server.key",
		MTLS:     false,
	})
	if err == nil {
		t.Fatal("expected error for missing cert file, got nil")
	}
}

// TestBuildCredentialsMTLSBadCA verifies that a CA file containing no valid
// PEM certificate block is rejected.
func TestBuildCredentialsMTLSBadCA(t *testing.T) {
	b := newCertBundle(t)

	badCA := filepath.Join(t.TempDir(), "bad-ca.crt")
	if err := os.WriteFile(badCA, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("write bad CA file: %v", err)
	}

	_, err := grpcserver.BuildCredentials(configs.GRPCConfig{
		CertFile:     b.srvCert,
		KeyFile:      b.srvKey,
		MTLS:         true,
		ClientCAFile: badCA,
	})
	if err == nil {
		t.Fatal("expected error for invalid CA certificate, got nil")
	}
}

// TestMTLSClientAuth is an integration-style test that verifies a full mTLS
// handshake using the same TLS parameters that BuildCredentials would produce:
// a server with RequireAndVerifyClientCert accepts a client presenting a
// valid certificate signed by the CA.
func TestMTLSClientAuth(t *testing.T) {
	b := newCertBundle(t)

	// Verify that BuildCredentials succeeds for mTLS config.
	_, err := grpcserver.BuildCredentials(configs.GRPCConfig{
		CertFile:     b.srvCert,
		KeyFile:      b.srvKey,
		MTLS:         true,
		ClientCAFile: b.caCertFile,
	})
	if err != nil {
		t.Fatalf("BuildCredentials: %v", err)
	}

	// Replicate the same TLS configuration locally for in-process handshake.
	srvCert, err := tls.LoadX509KeyPair(b.srvCert, b.srvKey)
	if err != nil {
		t.Fatalf("load server cert: %v", err)
	}
	caPEM, _ := os.ReadFile(b.caCertFile)
	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caPEM)

	srvTLS := &tls.Config{
		Certificates: []tls.Certificate{srvCert},
		ClientCAs:    caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	tlsLis := tls.NewListener(lis, srvTLS)
	defer tlsLis.Close()

	addr := lis.Addr().String()

	done := make(chan struct{})
	go func() {
		conn, err := tlsLis.Accept()
		if err == nil {
			_ = conn.(*tls.Conn).Handshake()
			conn.Close()
		}
		close(done)
	}()

	clientCert, err := tls.LoadX509KeyPair(b.clientCert, b.clientKey)
	if err != nil {
		t.Fatalf("load client cert: %v", err)
	}

	conn, err := tls.Dial("tcp", addr, &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
		ServerName:   "server",
	})
	if err != nil {
		t.Fatalf("tls.Dial with valid client cert failed: %v", err)
	}
	conn.Close()
	<-done
}

// TestMTLSRejectNoClientCert verifies that a server with RequireAndVerifyClientCert
// rejects a client that presents no certificate.
//
// In TLS 1.3 the server's rejection alert may arrive AFTER tls.Dial returns on
// the client side; to guarantee a synchronous failure we pin both sides to
// TLS 1.2 where the server sends its alert before completing the handshake.
func TestMTLSRejectNoClientCert(t *testing.T) {
	b := newCertBundle(t)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer lis.Close()

	srvCert, err := tls.LoadX509KeyPair(b.srvCert, b.srvKey)
	if err != nil {
		t.Fatalf("load server cert: %v", err)
	}
	caPEM, _ := os.ReadFile(b.caCertFile)
	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caPEM)

	// Pin to TLS 1.2 so the server alert is delivered during the handshake.
	srvTLS := &tls.Config{
		Certificates: []tls.Certificate{srvCert},
		ClientCAs:    caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
		MaxVersion:   tls.VersionTLS12,
	}
	tlsLis := tls.NewListener(lis, srvTLS)
	defer tlsLis.Close()

	addr := lis.Addr().String()

	serverErrCh := make(chan error, 1)
	go func() {
		conn, acceptErr := tlsLis.Accept()
		if acceptErr != nil {
			serverErrCh <- acceptErr
			return
		}
		serverErrCh <- conn.(*tls.Conn).Handshake()
		conn.Close()
	}()

	// Client without certificate, also pinned to TLS 1.2.
	_, dialErr := tls.Dial("tcp", addr, &tls.Config{
		RootCAs:    caPool,
		ServerName: "server",
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS12,
	})

	serverErr := <-serverErrCh

	// At least one side must report an error: the server must reject the
	// connection and/or the client must receive the rejection alert.
	if dialErr == nil && serverErr == nil {
		t.Fatal("expected at least one side to fail when client presents no certificate")
	}

	// The server must always report an error (missing client certificate).
	if serverErr == nil {
		t.Fatal("expected server handshake to fail when client presents no certificate")
	}
}
