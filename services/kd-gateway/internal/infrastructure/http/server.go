// Package http provides the Gin-Gonic HTTP server for kd-gateway.
//
// The HTTP server runs on a separate port from the gRPC server and exposes
// the REST API consumed by kd-portal and kdctl.  Its lifecycle (start /
// graceful-stop) is managed by UberFX.
//
// Server setup follows the minimal-middleware principle:
//   - gin.New() instead of gin.Default() for explicit middleware control
//   - ReadHeaderTimeout set to guard against Slowloris (CWE-400)
//   - Structured request logging via log/slog
//   - Panic recovery that returns a JSON 500 instead of crashing the process
package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-gateway/configs"
)

const gracefulStopTimeout = 15 * time.Second

// Server wraps a *http.Server so that FX lifecycle hooks can start and stop
// the Gin-based REST API cleanly.
type Server struct {
	httpSrv *http.Server
	engine  *gin.Engine
	log     *slog.Logger
}

// Params groups the FX-injected inputs for New.
type Params struct {
	fx.In

	LC             fx.Lifecycle
	Config         *configs.Config
	Log            *slog.Logger
	MetricsHandler http.Handler
}

// New constructs the HTTP/Gin server, attaches the base middleware chain and
// registers the FX lifecycle hooks that start and stop the server.
//
// The returned *Server exposes Engine() so that domain modules can register
// their route handlers against the same underlying *gin.Engine.
func New(p Params) *Server {
	cfg := p.Config.HTTP

	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()

	if err := engine.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		p.Log.Warn("http server: failed to set trusted proxies, falling back to localhost only",
			slog.Any("error", err),
		)
		_ = engine.SetTrustedProxies([]string{"127.0.0.1"})
	}

	engine.Use(requestLogger(p.Log))
	engine.Use(recoveryMiddleware(p.Log))
	engine.Use(structuredErrorMiddleware())

	engine.NoRoute(notFoundHandler)
	engine.NoMethod(methodNotAllowedHandler)
	engine.GET("/healthz", healthHandler)
	engine.GET("/metrics", gin.WrapH(p.MetricsHandler))

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           engine,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	srv := &Server{
		httpSrv: httpSrv,
		engine:  engine,
		log:     p.Log,
	}

	p.LC.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			p.Log.Info("http server starting", slog.String("addr", cfg.Addr))
			go func() {
				if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					p.Log.Error("http server stopped with error", slog.Any("error", err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			p.Log.Info("http server stopping")
			stopCtx, cancel := context.WithTimeout(ctx, gracefulStopTimeout)
			defer cancel()
			if err := httpSrv.Shutdown(stopCtx); err != nil {
				return fmt.Errorf("http server: graceful shutdown: %w", err)
			}
			p.Log.Info("http server stopped gracefully")
			return nil
		},
	})

	return srv
}

// Engine returns the underlying *gin.Engine so that domain modules registered
// via FX can add their route groups to the same server instance.
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// healthHandler responds to liveness probes.
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// requestLogger returns a Gin middleware that emits one structured log line
// per request using log/slog.
func requestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		status := c.Writer.Status()
		duration := time.Since(start)

		attrs := []slog.Attr{
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("status", status),
			slog.Duration("duration", duration),
			slog.String("ip", c.ClientIP()),
		}
		if query != "" {
			attrs = append(attrs, slog.String("query", query))
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("errors", c.Errors.String()))
		}

		level := slog.LevelInfo
		if status >= 500 {
			level = slog.LevelError
		} else if status >= 400 {
			level = slog.LevelWarn
		}

		log.LogAttrs(c.Request.Context(), level, "http request", attrs...)
	}
}

// recoveryMiddleware catches panics in handlers, logs the stack trace and
// returns HTTP 500 with a generic JSON body so the process keeps running.
func recoveryMiddleware(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("http handler panic recovered",
					slog.Any("panic", r),
					slog.String("path", c.Request.URL.Path),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "internal server error",
					"code":  "INTERNAL_ERROR",
				})
			}
		}()
		c.Next()
	}
}

// structuredErrorMiddleware converts accumulated Gin errors into a standard
// JSON payload expected by API consumers.
func structuredErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if c.Writer.Written() || len(c.Errors) == 0 {
			return
		}

		status := c.Writer.Status()
		if status < http.StatusBadRequest {
			status = http.StatusInternalServerError
		}

		c.AbortWithStatusJSON(status, gin.H{
			"error": c.Errors.Last().Error(),
			"code":  http.StatusText(status),
		})
	}
}

func notFoundHandler(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
		"error": "resource not found",
		"code":  "NOT_FOUND",
	})
}

func methodNotAllowedHandler(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusMethodNotAllowed, gin.H{
		"error": "method not allowed",
		"code":  "METHOD_NOT_ALLOWED",
	})
}
