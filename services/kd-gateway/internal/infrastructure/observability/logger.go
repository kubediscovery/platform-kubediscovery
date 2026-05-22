// Package observability initialises the structured logger for kd-gateway.
package observability

import (
	"log/slog"
	"os"
	"strings"

	"go.uber.org/fx"

	"github.com/kubediscovery/kd-gateway/configs"
)

// NewLogger constructs a *slog.Logger configured for the application log
// level defined in Config.  The logger writes JSON to stdout.
func NewLogger(cfg *configs.Config) *slog.Logger {
	level := parseLevel(cfg.App.LogLevel)
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}

// Module is the FX module that provides the *slog.Logger.
var Module = fx.Module("observability",
	fx.Provide(NewLogger),
	fx.Provide(NewPrometheusHandler),
	fx.Provide(NewOTLPExporter),
)

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
