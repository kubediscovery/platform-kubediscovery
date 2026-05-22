package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewPrometheusHandler returns an HTTP handler that exposes all metrics
// registered on prometheus.DefaultRegisterer at the standard /metrics path.
func NewPrometheusHandler() http.Handler {
	return promhttp.Handler()
}
