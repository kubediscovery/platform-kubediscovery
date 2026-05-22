package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

// ActiveAgentsSource reports how many agents are currently connected.
type ActiveAgentsSource interface {
	ConnectedCount() int
}

// Metrics holds kd-gateway Prometheus collectors for RED-style gRPC metrics.
type Metrics struct {
	GRPCRequestsTotal          *prometheus.CounterVec
	GRPCRequestDurationSeconds *prometheus.HistogramVec
}

// NewMetrics registers gateway metrics on the default Prometheus registerer.
// When agents is non-nil, active_agents_total is exposed as a GaugeFunc that
// reads the current connected count on each scrape.
func NewMetrics(agents ActiveAgentsSource) *Metrics {
	m := &Metrics{
		GRPCRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "grpc_requests_total",
			Help: "Total number of gRPC requests handled by kd-gateway.",
		}, []string{"grpc_service", "grpc_method", "grpc_code"}),
		GRPCRequestDurationSeconds: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "grpc_request_duration_seconds",
			Help:    "gRPC request latency in seconds.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		}, []string{"grpc_service", "grpc_method", "grpc_code"}),
	}

	collectors := []prometheus.Collector{
		m.GRPCRequestsTotal,
		m.GRPCRequestDurationSeconds,
	}
	if agents != nil {
		collectors = append(collectors, prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "active_agents_total",
			Help: "Number of kd-agent instances currently connected to kd-gateway.",
		}, func() float64 {
			return float64(agents.ConnectedCount())
		}))
	}
	prometheus.MustRegister(collectors...)

	return m
}
