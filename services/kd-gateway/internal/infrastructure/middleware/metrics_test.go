package middleware_test

import (
	"context"
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kubediscovery/kd-gateway/internal/infrastructure/middleware"
	"github.com/kubediscovery/kd-gateway/internal/infrastructure/observability"
)

func TestMetricsUnaryInterceptorRecordsRequest(t *testing.T) {
	reg := prometheus.NewRegistry()
	origReg := prometheus.DefaultRegisterer
	origGatherer := prometheus.DefaultGatherer
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg
	t.Cleanup(func() {
		prometheus.DefaultRegisterer = origReg
		prometheus.DefaultGatherer = origGatherer
	})

	m := observability.NewMetrics(nil)
	unary, _ := middleware.MetricsInterceptors(m)
	if len(unary) == 0 {
		t.Fatal("expected metrics unary interceptor")
	}

	sentinelErr := status.Error(codes.NotFound, "missing")
	handler := func(_ context.Context, _ any) (any, error) {
		return nil, sentinelErr
	}

	_, err := unary[0](context.Background(), nil, &grpc.UnaryServerInfo{
		FullMethod: "/kubediscovery.v1.GatewayService/ListAgents",
	}, handler)
	if !errors.Is(err, sentinelErr) {
		t.Fatalf("expected %v, got %v", sentinelErr, err)
	}

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	var count float64
	for _, mf := range mfs {
		if mf.GetName() != "grpc_requests_total" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			if metricLabel(metric, "grpc_code") == codes.NotFound.String() {
				count += metric.GetCounter().GetValue()
			}
		}
	}
	if count != 1 {
		t.Errorf("grpc_requests_total for NotFound = %v, want 1", count)
	}
}

func metricLabel(m *dto.Metric, name string) string {
	for _, lp := range m.GetLabel() {
		if lp.GetName() == name {
			return lp.GetValue()
		}
	}
	return ""
}
