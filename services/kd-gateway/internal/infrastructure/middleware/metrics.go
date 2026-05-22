package middleware

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"github.com/kubediscovery/kd-gateway/internal/infrastructure/observability"
)

// MetricsInterceptors returns unary and stream interceptors that record
// grpc_requests_total and grpc_request_duration_seconds.
func MetricsInterceptors(m *observability.Metrics) ([]grpc.UnaryServerInterceptor, []grpc.StreamServerInterceptor) {
	return []grpc.UnaryServerInterceptor{metricsUnaryInterceptor(m)},
		[]grpc.StreamServerInterceptor{metricsStreamInterceptor(m)}
}

func metricsUnaryInterceptor(m *observability.Metrics) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		recordGRPCMetrics(m, info.FullMethod, time.Since(start), err)
		return resp, err
	}
}

func metricsStreamInterceptor(m *observability.Metrics) grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()
		err := handler(srv, ss)
		recordGRPCMetrics(m, info.FullMethod, time.Since(start), err)
		return err
	}
}

func recordGRPCMetrics(m *observability.Metrics, fullMethod string, duration time.Duration, err error) {
	if m == nil {
		return
	}
	service, method := splitGRPCMethod(fullMethod)
	code := status.Code(err).String()
	labels := []string{service, method, code}
	m.GRPCRequestsTotal.WithLabelValues(labels...).Inc()
	m.GRPCRequestDurationSeconds.WithLabelValues(labels...).Observe(duration.Seconds())
}

func splitGRPCMethod(fullMethod string) (service, method string) {
	fullMethod = strings.TrimPrefix(fullMethod, "/")
	if i := strings.LastIndex(fullMethod, "/"); i >= 0 {
		return fullMethod[:i], fullMethod[i+1:]
	}
	if fullMethod == "" {
		return "unknown", "unknown"
	}
	return "unknown", fullMethod
}
