package observability

import (
	"fmt"
	"net/http"
)

// NewPrometheusHandler creates a minimal Prometheus text endpoint at /metrics.
func NewPrometheusHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = fmt.Fprintln(w, "# HELP kd_gateway_up Whether kd-gateway process is running")
		_, _ = fmt.Fprintln(w, "# TYPE kd_gateway_up gauge")
		_, _ = fmt.Fprintln(w, "kd_gateway_up 1")
	})
}
