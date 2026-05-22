package observability_test

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kubediscovery/kd-gateway/internal/infrastructure/observability"
)

func TestPrometheusHandlerExposesRegisteredMetrics(t *testing.T) {
	_ = withIsolatedRegistry(t)

	m := observability.NewMetrics(nil)
	m.GRPCRequestsTotal.WithLabelValues("svc", "method", "OK").Add(0)
	m.GRPCRequestDurationSeconds.WithLabelValues("svc", "method", "OK").Observe(0)

	rec := httptest.NewRecorder()
	observability.NewPrometheusHandler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))

	body := rec.Body.String()
	for _, name := range []string{
		"grpc_requests_total",
		"grpc_request_duration_seconds",
	} {
		if !strings.Contains(body, name) {
			t.Errorf("metrics exposition missing %q", name)
		}
	}
}
