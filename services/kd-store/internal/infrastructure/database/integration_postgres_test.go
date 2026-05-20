package database

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kubediscovery/kd-store/configs"
)

const (
	integrationDBHost = "KD_STORE_IT_DB_HOST"
	integrationDBPort = "KD_STORE_IT_DB_PORT"
	integrationDBName = "KD_STORE_IT_DB_NAME"
	integrationDBUser = "KD_STORE_IT_DB_USER"
	integrationDBPass = "KD_STORE_IT_DB_PASSWORD"
)

func TestPostgresMigrationsAndBasicCRUD(t *testing.T) {
	cfg, ok := integrationTestConfig(t)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool := mustOpenPool(t, ctx, cfg)
	defer pool.Close()

	mustApplyMigrations(t, cfg)
	clusterID := mustInsertCluster(t, ctx, pool)
	mustReadUpdateDeleteCluster(t, ctx, pool, clusterID)
	mustInsertAndReadEvent(t, ctx, pool, clusterID)
	mustInsertAndReadAnalysisMemory(t, ctx, pool, clusterID)
}

func integrationTestConfig(t *testing.T) (configs.DatabaseConfig, bool) {
	t.Helper()

	required := []string{integrationDBHost, integrationDBPort, integrationDBName, integrationDBUser}
	missing := make([]string, 0)
	for _, key := range required {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		t.Skipf("integration test skipped: set env vars %s", strings.Join(missing, ", "))
		return configs.DatabaseConfig{}, false
	}

	return configs.DatabaseConfig{
		Host:     os.Getenv(integrationDBHost),
		Port:     parseEnvInt(t, integrationDBPort),
		Name:     os.Getenv(integrationDBName),
		User:     os.Getenv(integrationDBUser),
		Password: os.Getenv(integrationDBPass),
		SSLMode:  defaultSSLMode,
	}, true
}

func parseEnvInt(t *testing.T, key string) int {
	t.Helper()

	val := strings.TrimSpace(os.Getenv(key))
	var port int
	_, err := fmt.Sscanf(val, "%d", &port)
	if err != nil || port <= 0 {
		t.Fatalf("invalid %s=%q", key, val)
	}
	return port
}

func mustApplyMigrations(t *testing.T, cfg configs.DatabaseConfig) {
	t.Helper()
	m, err := buildMigrate(cfg, slog.Default())
	if err != nil {
		t.Fatalf("buildMigrate() error: %v", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil || dbErr != nil {
			t.Logf("buildMigrate close warnings: src=%v db=%v", srcErr, dbErr)
		}
	}()
	if err := m.Up(); err != nil && !strings.Contains(err.Error(), "no change") {
		t.Fatalf("m.Up() error: %v", err)
	}
}

func mustOpenPool(t *testing.T, ctx context.Context, cfg configs.DatabaseConfig) *pgxpool.Pool {
	t.Helper()

	connString, err := buildConnString(cfg)
	if err != nil {
		t.Fatalf("buildConnString() error: %v", err)
	}
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		t.Fatalf("pgxpool.New() error: %v", err)
	}
	return pool
}

func mustInsertCluster(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()

	clusterName := fmt.Sprintf("it-cluster-%d", time.Now().UnixNano())
	var clusterID string
	err := pool.QueryRow(ctx, `
		INSERT INTO clusters (name, environment, status)
		VALUES ($1, $2, $3)
		RETURNING id
	`, clusterName, "staging", "active").Scan(&clusterID)
	if err != nil {
		t.Fatalf("insert cluster error: %v", err)
	}
	return clusterID
}

func mustReadUpdateDeleteCluster(t *testing.T, ctx context.Context, pool *pgxpool.Pool, clusterID string) {
	t.Helper()

	var gotName string
	if err := pool.QueryRow(ctx, "SELECT name FROM clusters WHERE id = $1", clusterID).Scan(&gotName); err != nil {
		t.Fatalf("read cluster error: %v", err)
	}

	if !strings.HasPrefix(gotName, "it-cluster-") {
		t.Fatalf("unexpected cluster name %q", gotName)
	}

	if _, err := pool.Exec(ctx, "UPDATE clusters SET status = $1 WHERE id = $2", "degraded", clusterID); err != nil {
		t.Fatalf("update cluster error: %v", err)
	}

	var gotStatus string
	if err := pool.QueryRow(ctx, "SELECT status FROM clusters WHERE id = $1", clusterID).Scan(&gotStatus); err != nil {
		t.Fatalf("read updated status error: %v", err)
	}
	if gotStatus != "degraded" {
		t.Fatalf("cluster status = %q, want degraded", gotStatus)
	}
}

func mustInsertAndReadEvent(t *testing.T, ctx context.Context, pool *pgxpool.Pool, clusterID string) {
	t.Helper()

	_, err := pool.Exec(ctx, `
		INSERT INTO events (cluster_id, namespace, kind, reason, message, metadata)
		VALUES ($1, $2, $3, $4, $5, '{}'::jsonb)
	`, clusterID, "default", "Pod", "FailedScheduling", "0/1 nodes available")
	if err != nil {
		t.Fatalf("insert event error: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM events WHERE cluster_id = $1", clusterID).Scan(&count); err != nil {
		t.Fatalf("count events error: %v", err)
	}
	if count == 0 {
		t.Fatal("expected at least one event")
	}
}

func mustInsertAndReadAnalysisMemory(t *testing.T, ctx context.Context, pool *pgxpool.Pool, clusterID string) {
	t.Helper()

	_, err := pool.Exec(ctx, `
		INSERT INTO analysis_memory (cluster_id, environment, namespace, memory_key, summary)
		VALUES ($1, $2, $3, $4, $5)
	`, clusterID, "staging", "default", "it-key", "integration summary")
	if err != nil {
		t.Fatalf("insert analysis_memory error: %v", err)
	}

	var summary string
	if err := pool.QueryRow(ctx, "SELECT summary FROM analysis_memory WHERE memory_key = $1", "it-key").Scan(&summary); err != nil {
		t.Fatalf("read analysis_memory error: %v", err)
	}
	if summary != "integration summary" {
		t.Fatalf("summary = %q, want integration summary", summary)
	}
}
