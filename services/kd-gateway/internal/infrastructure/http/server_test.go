// Package http_test exercises the HTTP server constructor and its middleware
// without requiring the full FX application to be wired up.
package http_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kubediscovery/kd-gateway/configs"
	httpserver "github.com/kubediscovery/kd-gateway/internal/infrastructure/http"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestMain(m *testing.M) {
	// Force release mode so gin doesn't print debug output in tests.
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

// freePort returns an available TCP port on loopback.
func freePort(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	addr := lis.Addr().String()
	lis.Close()
	return addr
}

// testConfig builds a minimal *configs.Config with the HTTP server listening
// on the given addr and all other fields at safe defaults.
func testConfig(addr string) *configs.Config {
	return &configs.Config{
		App: configs.AppConfig{
			Name:        "kd-gateway-test",
			Environment: "test",
			LogLevel:    "error",
		},
		HTTP: configs.HTTPConfig{
			Addr:              addr,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       30 * time.Second,
			TrustedProxies:    []string{"127.0.0.1"},
		},
	}
}

// newTestServer starts a real HTTP server via FX lifecycle, returning both the
// *httpserver.Server and a stop function the caller must invoke.
func newTestServer(t *testing.T, addr string) (*httpserver.Server, func()) {
	t.Helper()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := testConfig(addr)

	var srv *httpserver.Server

	app := fxtest.New(t,
		fx.Provide(func() *configs.Config { return cfg }),
		fx.Provide(func() *slog.Logger { return log }),
		fx.Provide(httpserver.New),
		fx.Populate(&srv),
	)
	app.RequireStart()

	stop := func() {
		app.RequireStop()
	}

	return srv, stop
}

// TestNew_EngineNotNil verifies that New returns a non-nil Server with an
// accessible *gin.Engine.
func TestNew_EngineNotNil(t *testing.T) {
	addr := freePort(t)
	srv, stop := newTestServer(t, addr)
	defer stop()

	if srv == nil {
		t.Fatal("expected non-nil *Server")
	}
	if srv.Engine() == nil {
		t.Fatal("expected non-nil *gin.Engine from Engine()")
	}
}

// TestHealthzEndpoint verifies the built-in /healthz route returns HTTP 200
// with body {"status":"ok"}.
func TestHealthzEndpoint(t *testing.T) {
	addr := freePort(t)
	srv, stop := newTestServer(t, addr)
	defer stop()

	// Give the listener a moment to be ready.
	time.Sleep(20 * time.Millisecond)

	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", body["status"])
	}

	_ = srv // used via the test server lifecycle
}

// TestEngineAcceptsExtraRoute verifies that routes added to the Engine()
// after construction are reachable via HTTP.
func TestEngineAcceptsExtraRoute(t *testing.T) {
	addr := freePort(t)
	srv, stop := newTestServer(t, addr)
	defer stop()

	srv.Engine().GET("/api/v1/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"pong": true})
	})

	time.Sleep(20 * time.Millisecond)

	resp, err := http.Get("http://" + addr + "/api/v1/ping")
	if err != nil {
		t.Fatalf("GET /api/v1/ping: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// TestRecoveryMiddleware verifies that a handler that panics is caught by the
// recovery middleware and returns HTTP 500 with the expected JSON shape.
func TestRecoveryMiddleware(t *testing.T) {
	// Use httptest.NewRecorder to exercise the middleware without a live port.
	gin.SetMode(gin.TestMode)

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := testConfig("127.0.0.1:0")

	var srv *httpserver.Server

	app := fxtest.New(t,
		fx.Provide(func() *configs.Config { return cfg }),
		fx.Provide(func() *slog.Logger { return log }),
		fx.Provide(httpserver.New),
		fx.Populate(&srv),
	)
	app.RequireStart()
	defer app.RequireStop()

	srv.Engine().GET("/panic", func(c *gin.Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/panic", nil)
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode panic response: %v", err)
	}
	if body["error"] != "internal server error" {
		t.Errorf("unexpected error field: %q", body["error"])
	}
	if body["code"] != "INTERNAL_ERROR" {
		t.Errorf("unexpected code field: %q", body["code"])
	}
}

// TestUnknownRouteReturns404 verifies that the default Gin 404 handler is
// active for unknown paths.
func TestUnknownRouteReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := testConfig("127.0.0.1:0")

	var srv *httpserver.Server

	app := fxtest.New(t,
		fx.Provide(func() *configs.Config { return cfg }),
		fx.Provide(func() *slog.Logger { return log }),
		fx.Provide(httpserver.New),
		fx.Populate(&srv),
	)
	app.RequireStart()
	defer app.RequireStop()

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/does-not-exist", nil)
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
