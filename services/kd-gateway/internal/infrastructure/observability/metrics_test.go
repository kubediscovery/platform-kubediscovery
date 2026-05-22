package observability_test

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/kubediscovery/kd-gateway/internal/infrastructure/observability"
)

type stubAgents struct{ count int }

func (s stubAgents) ConnectedCount() int { return s.count }

func withIsolatedRegistry(t *testing.T) *prometheus.Registry {
	t.Helper()
	reg := prometheus.NewRegistry()
	origReg := prometheus.DefaultRegisterer
	origGatherer := prometheus.DefaultGatherer
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg
	t.Cleanup(func() {
		prometheus.DefaultRegisterer = origReg
		prometheus.DefaultGatherer = origGatherer
	})
	return reg
}

func TestNewMetricsRegistersRequiredCollectors(t *testing.T) {
	reg := withIsolatedRegistry(t)
	m := observability.NewMetrics(stubAgents{count: 3})
	// Vec metrics appear in exposition only after at least one label set is used.
	m.GRPCRequestsTotal.WithLabelValues("svc", "method", "OK").Add(0)
	m.GRPCRequestDurationSeconds.WithLabelValues("svc", "method", "OK").Observe(0)

	names := make(map[string]struct{})
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range mfs {
		names[mf.GetName()] = struct{}{}
	}

	for _, want := range []string{
		"grpc_requests_total",
		"grpc_request_duration_seconds",
		"active_agents_total",
	} {
		if _, ok := names[want]; !ok {
			t.Errorf("missing metric %q; got %v", want, names)
		}
	}
}

func TestActiveAgentsGaugeReflectsConnectedCount(t *testing.T) {
	reg := withIsolatedRegistry(t)
	agents := stubAgents{count: 5}
	_ = observability.NewMetrics(agents)

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	var got float64
	found := false
	for _, mf := range mfs {
		if mf.GetName() != "active_agents_total" {
			continue
		}
		found = true
		for _, m := range mf.GetMetric() {
			got = m.GetGauge().GetValue()
		}
	}
	if !found {
		t.Fatal("active_agents_total not found in registry")
	}
	if got != 5 {
		t.Errorf("active_agents_total = %v, want 5", got)
	}
}

func TestGRPCRequestCounterIncrements(t *testing.T) {
	withIsolatedRegistry(t)
	m := observability.NewMetrics(nil)
	m.GRPCRequestsTotal.WithLabelValues("kubediscovery.v1.GatewayService", "ListAgents", "OK").Inc()

	if err := testutil.CollectAndCompare(m.GRPCRequestsTotal, strings.NewReader(`
		# HELP grpc_requests_total Total number of gRPC requests handled by kd-gateway.
		# TYPE grpc_requests_total counter
		grpc_requests_total{grpc_code="OK",grpc_method="ListAgents",grpc_service="kubediscovery.v1.GatewayService"} 1
	`)); err != nil {
		t.Fatalf("counter mismatch: %v", err)
	}
}
