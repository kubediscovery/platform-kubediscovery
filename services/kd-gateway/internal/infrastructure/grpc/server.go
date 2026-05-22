// Package grpc provides the gRPC server for kd-gateway with optional mTLS.
//
// When mTLS is enabled (GRPC_MTLS=1, the default) the server:
//   - Loads the server certificate and private key
//   - Loads the CA certificate used to verify client certificates
//   - Sets tls.RequireAndVerifyClientCert so every connecting kd-agent must
//     present a certificate signed by the trusted CA
//
// When mTLS is disabled (GRPC_MTLS=0) only the server certificate is loaded;
// clients are not required to present a certificate.
//
// Server lifecycle (start / graceful-stop) is managed by UberFX.
package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/kubediscovery/kd-gateway/configs"
	"github.com/kubediscovery/kd-gateway/internal/infrastructure/middleware"
	"github.com/kubediscovery/kd-gateway/internal/infrastructure/observability"
)

const (
	gracefulStopTimeout = 15 * time.Second
)

// Server wraps a *grpc.Server together with its network listener so that
// the FX lifecycle hooks can start and stop the server cleanly.
type Server struct {
	server   *grpc.Server
	listener net.Listener
	log      *slog.Logger
}

// Params groups the FX-injected inputs for New.
type Params struct {
	fx.In

	LC      fx.Lifecycle
	Config  *configs.Config
	Log     *slog.Logger
	Metrics *observability.Metrics
}

// New constructs the gRPC server, configures TLS/mTLS, chains all interceptors
// and registers the FX lifecycle hooks that start and stop the server.
//
// The returned *Server exposes GRPCServer() so that domain modules can
// register their service implementations against the same underlying
// *grpc.Server instance.
func New(p Params) (*Server, error) {
	cfg := p.Config.GRPC

	creds, err := BuildCredentials(cfg)
	if err != nil {
		return nil, fmt.Errorf("grpc server: build credentials: %w", err)
	}

	opts := buildServerOptions(cfg, creds, p.Metrics)

	grpcSrv := grpc.NewServer(opts...)

	healthSrv := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcSrv, healthSrv)

	lis, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("grpc server: listen %s: %w", cfg.Addr, err)
	}

	srv := &Server{
		server:   grpcSrv,
		listener: lis,
		log:      p.Log,
	}

	p.LC.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			p.Log.Info("grpc server starting",
				slog.String("addr", cfg.Addr),
				slog.Bool("mtls", cfg.MTLS),
				slog.Bool("debug", cfg.Debug),
			)
			go func() {
				if err := grpcSrv.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
					p.Log.Error("grpc server stopped with error", slog.Any("error", err))
				}
			}()
			return nil
		},
		OnStop: func(_ context.Context) error {
			p.Log.Info("grpc server stopping")
			stopped := make(chan struct{})
			go func() {
				grpcSrv.GracefulStop()
				close(stopped)
			}()
			select {
			case <-stopped:
				p.Log.Info("grpc server stopped gracefully")
			case <-time.After(gracefulStopTimeout):
				p.Log.Warn("grpc server graceful stop timed out, forcing stop")
				grpcSrv.Stop()
			}
			return nil
		},
	})

	return srv, nil
}

// GRPCServer returns the underlying *grpc.Server so that service handlers
// from other FX modules can register their implementations.
func (s *Server) GRPCServer() *grpc.Server {
	return s.server
}

// BuildCredentials constructs the appropriate TransportCredentials depending
// on whether mTLS is enabled.
//
// It is exported so that tests can validate credential construction without
// spinning up a full FX application.
func BuildCredentials(cfg configs.GRPCConfig) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load server certificate: %w", err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	if cfg.MTLS {
		if cfg.ClientCAFile == "" {
			return nil, errors.New("grpc.client_ca_file is required when mTLS is enabled")
		}

		caCert, err := os.ReadFile(cfg.ClientCAFile)
		if err != nil {
			return nil, fmt.Errorf("read client CA file %q: %w", cfg.ClientCAFile, err)
		}

		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("parse client CA certificate from %q: no valid certificate found", cfg.ClientCAFile)
		}

		tlsCfg.ClientCAs = caPool
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return credentials.NewTLS(tlsCfg), nil
}

// buildServerOptions assembles the gRPC server options: transport credentials
// and the interceptor chains (always-on + optional debug interceptors).
func buildServerOptions(cfg configs.GRPCConfig, creds credentials.TransportCredentials, metrics *observability.Metrics) []grpc.ServerOption {
	unary, stream := middleware.BaseInterceptors()

	if metrics != nil {
		mu, ms := middleware.MetricsInterceptors(metrics)
		unary = append(mu, unary...)
		stream = append(ms, stream...)
	}

	if cfg.Debug {
		du, ds := middleware.DebugInterceptors()
		unary = append(unary, du...)
		stream = append(stream, ds...)
	}

	return []grpc.ServerOption{
		grpc.Creds(creds),
		grpc.ChainUnaryInterceptor(unary...),
		grpc.ChainStreamInterceptor(stream...),
	}
}
