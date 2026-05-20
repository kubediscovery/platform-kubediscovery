// Package middleware provides gRPC interceptors for kd-gateway.
//
// BaseInterceptors returns the always-on logging interceptors.
// DebugInterceptors returns additional per-call duration interceptors that
// are only activated when GRPC_DEBUG=1.
package middleware

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

// BaseInterceptors returns the default unary and stream interceptors that
// are always active.  They log the method name, peer address and any error.
func BaseInterceptors() ([]grpc.UnaryServerInterceptor, []grpc.StreamServerInterceptor) {
	return []grpc.UnaryServerInterceptor{loggingUnaryInterceptor},
		[]grpc.StreamServerInterceptor{loggingStreamInterceptor}
}

// DebugInterceptors returns additional interceptors that log per-call
// durations.  Activate with GRPC_DEBUG=1.
func DebugInterceptors() ([]grpc.UnaryServerInterceptor, []grpc.StreamServerInterceptor) {
	return []grpc.UnaryServerInterceptor{unaryDebugInterceptor},
		[]grpc.StreamServerInterceptor{streamDebugInterceptor}
}

// loggingUnaryInterceptor logs each unary RPC method and any returned error.
func loggingUnaryInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	p, _ := peer.FromContext(ctx)
	resp, err := handler(ctx, req)
	slog.Info("grpc unary",
		slog.String("method", info.FullMethod),
		slog.String("peer", peerAddr(p)),
		slog.Any("error", err),
	)
	return resp, err
}

// loggingStreamInterceptor logs each streaming RPC: peer, stream flags and
// any error returned when the handler completes.
func loggingStreamInterceptor(
	srv any,
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	p, _ := peer.FromContext(ss.Context())
	start := time.Now()
	err := handler(srv, ss)
	slog.Info("grpc stream",
		slog.String("method", info.FullMethod),
		slog.String("peer", peerAddr(p)),
		slog.Bool("client_stream", info.IsClientStream),
		slog.Bool("server_stream", info.IsServerStream),
		slog.Duration("duration", time.Since(start)),
		slog.Any("error", err),
	)
	return err
}

// unaryDebugInterceptor logs per-call duration for unary RPCs.
func unaryDebugInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	p, _ := peer.FromContext(ctx)
	slog.Debug("grpc unary debug",
		slog.String("method", info.FullMethod),
		slog.String("peer", peerAddr(p)),
		slog.Duration("duration", time.Since(start)),
		slog.Any("error", err),
	)
	return resp, err
}

// streamDebugInterceptor logs per-call duration for streaming RPCs.
func streamDebugInterceptor(
	srv any,
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	start := time.Now()
	err := handler(srv, ss)
	p, _ := peer.FromContext(ss.Context())
	slog.Debug("grpc stream debug",
		slog.String("method", info.FullMethod),
		slog.String("peer", peerAddr(p)),
		slog.Bool("client_stream", info.IsClientStream),
		slog.Bool("server_stream", info.IsServerStream),
		slog.Duration("duration", time.Since(start)),
		slog.Any("error", err),
	)
	return err
}

func peerAddr(p *peer.Peer) string {
	if p != nil && p.Addr != nil {
		return p.Addr.String()
	}
	return "unknown"
}
