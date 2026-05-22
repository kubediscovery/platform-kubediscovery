// Package grpc provides the gRPC client connection builder for kd-agent,
// including mTLS credential construction and a StreamOpener implementation
// that wraps the live gateway connection.
package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"os"

	gatewayv1 "github.com/kubediscovery/kd-libs/core/v1/gateway"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/kubediscovery/kd-agent/configs"
)

// BuildCredentials creates transport credentials from the agent gRPC config.
//
// When InsecureSkipVerify is true, plain-text (insecure) credentials are
// returned so the agent can dial without TLS — useful in development.
//
// Otherwise, a tls.Config is built:
//   - RootCAs is loaded from CAFile (when set).
//   - A client certificate is loaded when both ClientCertFile and
//     ClientKeyFile are set (mTLS).
//   - ServerName is resolved from the config or from the host portion of Addr.
func BuildCredentials(cfg configs.GRPCConfig) (credentials.TransportCredentials, error) {
	if cfg.InsecureSkipVerify {
		return insecure.NewCredentials(), nil
	}

	tlsCfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: resolveServerName(cfg),
	}

	if cfg.CAFile != "" {
		caPEM, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file %q: %w", cfg.CAFile, err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("parse CA certificate from %q: no valid certificate found", cfg.CAFile)
		}
		tlsCfg.RootCAs = pool
	}

	if cfg.ClientCertFile != "" && cfg.ClientKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.ClientCertFile, cfg.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate from %q / %q: %w",
				cfg.ClientCertFile, cfg.ClientKeyFile, err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return credentials.NewTLS(tlsCfg), nil
}

// resolveServerName returns the TLS server name for the connection.
// Priority order:
//  1. cfg.ServerName (explicit override)
//  2. The host portion of cfg.Addr, if it is not a bare IP address
//  3. Empty string (Go TLS will use the connection host)
func resolveServerName(cfg configs.GRPCConfig) string {
	if cfg.ServerName != "" {
		return cfg.ServerName
	}
	host, _, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return ""
	}
	if net.ParseIP(host) != nil {
		return ""
	}
	return host
}

// Opener implements service.StreamOpener by dialing the gateway and opening
// a new AgentStream on each call to OpenStream.
type Opener struct {
	cfg configs.GRPCConfig
	log *slog.Logger
}

// NewOpener constructs an Opener from the given gRPC config.
func NewOpener(cfg configs.GRPCConfig, log *slog.Logger) *Opener {
	return &Opener{cfg: cfg, log: log}
}

// OpenStream dials the gateway and returns a live bidirectional stream.
// The connection is created fresh for each call; the caller owns the stream.
func (o *Opener) OpenStream(ctx context.Context) (gatewayv1.GatewayService_AgentStreamClient, error) {
	creds, err := BuildCredentials(o.cfg)
	if err != nil {
		return nil, fmt.Errorf("build credentials: %w", err)
	}

	conn, err := grpc.NewClient(o.cfg.Addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("dial %q: %w", o.cfg.Addr, err)
	}

	o.log.Debug("dialed gateway", slog.String("addr", o.cfg.Addr))

	client := gatewayv1.NewGatewayServiceClient(conn)
	stream, err := client.AgentStream(ctx)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open agent stream: %w", err)
	}

	return stream, nil
}
