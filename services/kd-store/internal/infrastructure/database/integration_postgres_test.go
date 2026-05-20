package database

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kubediscovery/kd-store/configs"
)

type integrationDBConfig struct {
	database configs.DatabaseConfig
	dsn      string
}

func TestPostgresMigrationsAndBasicCRUD(t *testing.T) {
	t.Parallel()

	cfg, ok := loadIntegrationDBConfig()
	if !ok {
		t.Skip("skipping postgres integration test: set KD_STORE_IT_DB_HOST, KD_STORE_IT_DB_PORT, KD_STORE_IT_DB_NAME, KD_STORE_IT_DB_USER")
	}

	ctx := context.Background()
	mustApplyMigrations(t, cfg.database)
	pool := mustOpenPool(t, cfg.dsn)
	t.Cleanup(func() { pool.Close() })

	clusterUID := mustInsertCluster(t, ctx, pool)
	mustUpdateClusterStatus(t, ctx, pool, clusterUID)
	mustDeleteCluster(t, ctx, pool, clusterUID)

	mustInsertAndReadEvent(t, ctx, pool)
	mustInsertAndReadAnalysisMemory(t, ctx, pool)
}

func loadIntegrationDBConfig() (integrationDBConfig, bool) {
	host := os.Getenv("KD_STORE_IT_DB_HOST")
	portStr := os.Getenv("KD_STORE_IT_DB_PORT")
	name := os.Getenv("KD_STORE_IT_DB_NAME")
	user := os.Getenv("KD_STORE_IT_DB_USER")

	if host == "" || portStr == "" || name == "" || user == "" {
		return integrationDBConfig{}, false
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		return integrationDBConfig{}, false
	}

	cfg := configs.DatabaseConfig{
		Host:     host,
		Port:     port,
		Name:     name,
		User:     user,
		Password: os.Getenv("KD_STORE_IT_DB_PASSWORD"),
		SSLMode:  os.Getenv("KD_STORE_IT_DB_SSLMODE"),
	}

	dsn, err := buildConnString(cfg)
	if err != nil {
		return integrationDBConfig{}, false
	}

	return integrationDBConfig{database: cfg, dsn: dsn}, true
}

func mustApplyMigrations(t *testing.T, cfg configs.DatabaseConfig) {
	t.Helper()

	m, err := buildMigrate(cfg, slog.Default())
	if err != nil {
		t.Fatalf("buildMigrate() error: %v", err)
	}
	t.Cleanup(func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			t.Errorf("close migration source error: %v", srcErr)
		}
		if dbErr != nil {
			t.Errorf("close migration database error: %v", dbErr)
		}
	})

	migrator := &Migrator{m: m, logger: slog.Default()}
	if err := migrator.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations() error: %v", err)
	}
}

func mustOpenPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New() error: %v", err)
	}
	return pool
}

func mustInsertCluster(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()

	uid := uuid.NewString()
	_, err := pool.Exec(
		ctx,
		"INSERT INTO clusters (uid, name, environment, status) VALUES ($1, $2, $3, $4)",
		uid,
		"it-cluster",
		"it",
		"active",
	)
	if err != nil {
		t.Fatalf("insert cluster error: %v", err)
	}

	return uid
}

func mustUpdateClusterStatus(t *testing.T, ctx context.Context, pool *pgxpool.Pool, clusterUID string) {
	t.Helper()

	var status string
	err := pool.QueryRow(
		ctx,
		"UPDATE clusters SET status = $1 WHERE uid = $2 RETURNING status",
		"inactive",
		clusterUID,
	).Scan(&status)
	if err != nil {
		t.Fatalf("update cluster error: %v", err)
	}
	if status != "inactive" {
		t.Fatalf("cluster status mismatch: got %q, want %q", status, "inactive")
	}
}

func mustDeleteCluster(t *testing.T, ctx context.Context, pool *pgxpool.Pool, clusterUID string) {
	t.Helper()

	res, err := pool.Exec(ctx, "DELETE FROM clusters WHERE uid = $1", clusterUID)
	if err != nil {
		t.Fatalf("delete cluster error: %v", err)
	}
	if res.RowsAffected() != 1 {
		t.Fatalf("delete cluster rows affected = %d, want 1", res.RowsAffected())
	}
}

func mustInsertAndReadEvent(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	memoryKey := fmt.Sprintf("mk-%s", uuid.NewString())
	var gotMemoryKey, gotSeverity string
	err := pool.QueryRow(
		ctx,
		`INSERT INTO events (cluster_uid, memory_key, severity, diagnosis, recommendations)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING memory_key, severity`,
		"cluster-a",
		memoryKey,
		"warning",
		"unschedulable",
		"check taints",
	).Scan(&gotMemoryKey, &gotSeverity)
	if err != nil {
		t.Fatalf("insert/read event error: %v", err)
	}

	if gotMemoryKey != memoryKey {
		t.Fatalf("event memory_key mismatch: got %q, want %q", gotMemoryKey, memoryKey)
	}
	if gotSeverity != "warning" {
		t.Fatalf("event severity mismatch: got %q, want %q", gotSeverity, "warning")
	}
}

func mustInsertAndReadAnalysisMemory(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	memoryKey := fmt.Sprintf("mk-%s", uuid.NewString())
	var gotMemoryKey, gotNamespace string
	err := pool.QueryRow(
		ctx,
		`INSERT INTO analysis_memory (cluster_uid, memory_key, namespace, analysis, embedding)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING memory_key, namespace`,
		"cluster-a",
		memoryKey,
		"kube-system",
		"cpu pressure",
		vectorLiteral(1536),
	).Scan(&gotMemoryKey, &gotNamespace)
	if err != nil {
		t.Fatalf("insert/read analysis_memory error: %v", err)
	}

	if gotMemoryKey != memoryKey {
		t.Fatalf("analysis_memory memory_key mismatch: got %q, want %q", gotMemoryKey, memoryKey)
	}
	if gotNamespace != "kube-system" {
		t.Fatalf("analysis_memory namespace mismatch: got %q, want %q", gotNamespace, "kube-system")
	}
}

func vectorLiteral(size int) string {
	if size <= 0 {
		return "[]"
	}

	buf := make([]byte, 0, size*2+2)
	buf = append(buf, '[')
	for i := 0; i < size; i++ {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '0')
	}
	buf = append(buf, ']')

	return string(buf)
}
