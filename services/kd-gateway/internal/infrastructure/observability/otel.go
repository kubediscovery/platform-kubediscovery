package observability

import (
	"context"
	"fmt"
	"log/slog"

	"go.uber.org/fx"

	"github.com/kubediscovery/kd-gateway/configs"
)

// OTLPExporter represents configured OTLP export settings for traces.
type OTLPExporter struct {
	Endpoint string
	Insecure bool
}

// NewOTLPExporter validates OTLP settings and exposes them for other modules.
func NewOTLPExporter(lc fx.Lifecycle, cfg *configs.Config, log *slog.Logger) (*OTLPExporter, error) {
	if cfg.Obs.OTLP.Endpoint == "" {
		return nil, fmt.Errorf("observability: otlp endpoint is required")
	}

	exporter := &OTLPExporter{
		Endpoint: cfg.Obs.OTLP.Endpoint,
		Insecure: cfg.Obs.OTLP.Insecure,
	}

	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			log.Info("otel exporter configured",
				slog.String("endpoint", exporter.Endpoint),
				slog.Bool("insecure", exporter.Insecure),
			)
			return nil
		},
	})

	return exporter, nil
}
