// Package middleware_test exercises the gRPC interceptor helpers.
package middleware_test

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"

	"github.com/kubediscovery/kd-gateway/internal/infrastructure/middleware"
)

func TestBaseInterceptorsNotNil(t *testing.T) {
	unary, stream := middleware.BaseInterceptors()
	if len(unary) == 0 {
		t.Error("expected at least one unary interceptor from BaseInterceptors")
	}
	if len(stream) == 0 {
		t.Error("expected at least one stream interceptor from BaseInterceptors")
	}
}

func TestMetricsInterceptorsNotNil(t *testing.T) {
	unary, stream := middleware.MetricsInterceptors(nil)
	if len(unary) != 1 {
		t.Errorf("expected 1 unary metrics interceptor, got %d", len(unary))
	}
	if len(stream) != 1 {
		t.Errorf("expected 1 stream metrics interceptor, got %d", len(stream))
	}
}

func TestDebugInterceptorsNotNil(t *testing.T) {
	unary, stream := middleware.DebugInterceptors()
	if len(unary) == 0 {
		t.Error("expected at least one unary interceptor from DebugInterceptors")
	}
	if len(stream) == 0 {
		t.Error("expected at least one stream interceptor from DebugInterceptors")
	}
}

// TestLoggingUnaryInterceptorPassthrough verifies that the base unary
// interceptor calls the handler and propagates the response/error unchanged.
func TestLoggingUnaryInterceptorPassthrough(t *testing.T) {
	unary, _ := middleware.BaseInterceptors()
	if len(unary) == 0 {
		t.Skip("no unary interceptors")
	}

	interceptor := unary[0]
	sentinelErr := errors.New("sentinel")
	sentinelResp := struct{ v int }{v: 42}

	handler := func(_ context.Context, _ any) (any, error) {
		return sentinelResp, sentinelErr
	}

	resp, err := interceptor(context.Background(), "req", &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}, handler)

	if !errors.Is(err, sentinelErr) {
		t.Errorf("expected err %v, got %v", sentinelErr, err)
	}
	if resp != sentinelResp {
		t.Errorf("expected resp %v, got %v", sentinelResp, resp)
	}
}

// stubStream is a minimal grpc.ServerStream stub for testing stream interceptors.
type stubStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *stubStream) Context() context.Context { return s.ctx }

// TestLoggingStreamInterceptorPassthrough verifies that the base stream
// interceptor calls the handler and propagates its error.
func TestLoggingStreamInterceptorPassthrough(t *testing.T) {
	_, stream := middleware.BaseInterceptors()
	if len(stream) == 0 {
		t.Skip("no stream interceptors")
	}

	interceptor := stream[0]
	sentinelErr := errors.New("stream-sentinel")

	handler := func(_ any, _ grpc.ServerStream) error { return sentinelErr }

	err := interceptor(nil, &stubStream{ctx: context.Background()}, &grpc.StreamServerInfo{
		FullMethod:     "/test.Service/StreamMethod",
		IsClientStream: true,
		IsServerStream: true,
	}, handler)

	if !errors.Is(err, sentinelErr) {
		t.Errorf("expected err %v, got %v", sentinelErr, err)
	}
}
