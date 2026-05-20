// Package logger provides a structured JSON logger using log/slog.
package logger

import (
	"log/slog"
	"os"
)

// NewJSON creates a JSON structured logger.
func NewJSON(level slog.Leveler) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}
