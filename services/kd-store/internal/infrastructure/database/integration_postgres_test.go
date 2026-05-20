package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kubediscovery/kd-store/configs"
)

// Environment variable names used to configure the integration-test database.
// These hold variable NAMES (keys), not credentials.
const (
	itEnvHost    = "KD_STORE_IT_DB_HOST"
	itEnvPort    = "KD_STORE_IT_DB_PORT"
	itEnvName    = "KD_STORE_IT_DB_NAME"
	itEnvUser    = "KD_STORE_IT_DB_USER"
	itEnvSSLMode = "KD_STORE_IT_DB_SSLMODE"
)

// itPasswordEnvKey is the environment variable that carries the DB password for
// integration tests. Kept separate from the other constants to satisfy gosec
// (G101 treats consts whose names contain "password" as potential hardcoded creds).
var itPasswordEnvKey = "KD_STORE_IT_DB_PASSWORD" //nolint:gosec // holds an env-var KEY, not a credential value

// itDBConfig holds the resolved integration-test database config alongside the
// pgxpool DSN so callers do not have to reconstruct it.
type itDBConfig struct {
	database configs.DatabaseConfig
	dsn      string
}

// TestPostgresMigrationsAndBasicCRUD is the single entry point for integration
// tests that require a live PostgreSQL instance.
// The test is skipped automatically when the required KD_STORE_IT_DB_* env
// vars are absent so that it never blocks ordinary unit-test runs.
func TestPostgresMigrationsAndBasicCRUD(t *testing.T) {
	t.Parallel()

	cfg, ok := loadITConfig(t)
	if !ok {
		t.Skip("skipping postgres integration test: set " + itEnvHost + ", " + itEnvPort + ", " + itEnvName + ", " + itEnvUser)
	}

	ctx := context.Background()
	mustApplyITMigrations(t, cfg.database)

	pool := mustOpenITPool(t, cfg.dsn)
	t.Cleanup(func() { pool.Close() })

	uid := mustITInsertCluster(ctx, t, pool)
	mustITUpdateClusterStatus(ctx, t, pool, uid)
	mustITDeleteCluster(ctx, t, pool, uid)

	mustITInsertAndReadEvent(ctx, t, pool)
	mustITInsertAndReadAnalysisMemory(ctx, t, pool)
}

// loadITConfig reads integration-test DB settings from environment variables
// and returns the populated config. The bool result is false when any required
// variable is absent or the port cannot be parsed.
func loadITConfig(t *testing.T) (itDBConfig, bool) {
	t.Helper()

	host := os.Getenv(itEnvHost)
	portStr := os.Getenv(itEnvPort)
	name := os.Getenv(itEnvName)
	user := os.Getenv(itEnvUser)

	if host == "" || portStr == "" || name == "" || user == "" {
		return itDBConfig{}, false
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		return itDBConfig{}, false
	}

	dbCfg := configs.DatabaseConfig{
		Host:     host,
		Port:     port,
		Name:     name,
		User:     user,
		Password: os.Getenv(itPasswordEnvKey),
		SSLMode:  os.Getenv(itEnvSSLMode),
	}

	dsn, err := buildConnString(dbCfg)
	if err != nil {
		t.Logf("loadITConfig: buildConnString error: %v", err)
		return itDBConfig{}, false
	}

	return itDBConfig{database: dbCfg, dsn: dsn}, true
}

// mustApplyITMigrations runs all pending migrations against the integration DB
// and registers a cleanup hook to close the migrator.
func mustApplyITMigrations(t *testing.T, cfg configs.DatabaseConfig) {
	t.Helper()

	m, err := buildMigrate(cfg, slog.Default())
	if err != nil {
		t.Fatalf("buildMigrate error: %v", err)
	}

	t.Cleanup(func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			t.Errorf("close migration source: %v", srcErr)
		}
		if dbErr != nil {
			t.Errorf("close migration db: %v", dbErr)
		}
	})

	migrator := &Migrator{m: m, logger: slog.Default()}
	if err := migrator.RunMigrations(); err != nil {
		t.Fatalf("RunMigrations error: %v", err)
	}
}

// mustOpenITPool opens a pgxpool connection and registers a cleanup hook.
func mustOpenITPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("pgxpool.New error: %v", err)
	}

	return pool
}

// mustITInsertCluster inserts a test cluster row and returns the generated UID.
func mustITInsertCluster(ctx context.Context, t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()

	uid := itRandHex()
	_, err := pool.Exec(
		ctx,
		"INSERT INTO clusters (uid, name, environment, status) VALUES ($1, $2, $3, $4)",
		uid, "it-cluster", "it", "active",
	)
	if err != nil {
		t.Fatalf("insert cluster error: %v", err)
	}

	return uid
}

// mustITUpdateClusterStatus updates the cluster status and verifies the returned value.
func mustITUpdateClusterStatus(ctx context.Context, t *testing.T, pool *pgxpool.Pool, uid string) {
	t.Helper()

	const wantStatus = "inactive"
	var got string
	err := pool.QueryRow(
		ctx,
		"UPDATE clusters SET status = $1 WHERE uid = $2 RETURNING status",
		wantStatus, uid,
	).Scan(&got)
	if err != nil {
		t.Fatalf("update cluster status error: %v", err)
	}
	if got != wantStatus {
		t.Fatalf("cluster status: got %q, want %q", got, wantStatus)
	}
}

// mustITDeleteCluster deletes the cluster row and verifies exactly one row was affected.
func mustITDeleteCluster(ctx context.Context, t *testing.T, pool *pgxpool.Pool, uid string) {
	t.Helper()

	res, err := pool.Exec(ctx, "DELETE FROM clusters WHERE uid = $1", uid)
	if err != nil {
		t.Fatalf("delete cluster error: %v", err)
	}
	if res.RowsAffected() != 1 {
		t.Fatalf("delete cluster rows affected = %d, want 1", res.RowsAffected())
	}
}

// mustITInsertAndReadEvent inserts an event row and verifies the returned fields.
func mustITInsertAndReadEvent(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	memKey := "mk-" + itRandHex()
	const wantSeverity = "warning"
	var gotKey, gotSeverity string
	err := pool.QueryRow(
		ctx,
		`INSERT INTO events (cluster_uid, memory_key, severity, diagnosis, recommendations)
		 VALUES ($1, $2, $3, $4, $5) RETURNING memory_key, severity`,
		"cluster-a", memKey, wantSeverity, "unschedulable", "check taints",
	).Scan(&gotKey, &gotSeverity)
	if err != nil {
		t.Fatalf("insert event error: %v", err)
	}

	if gotKey != memKey {
		t.Fatalf("event memory_key: got %q, want %q", gotKey, memKey)
	}
	if gotSeverity != wantSeverity {
		t.Fatalf("event severity: got %q, want %q", gotSeverity, wantSeverity)
	}
}

// mustITInsertAndReadAnalysisMemory inserts an analysis_memory row and verifies
// the returned fields. The embedding column requires a pgvector literal string.
func mustITInsertAndReadAnalysisMemory(ctx context.Context, t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	memKey := "mk-" + itRandHex()
	const wantNamespace = "kube-system"
	var gotKey, gotNS string
	err := pool.QueryRow(
		ctx,
		`INSERT INTO analysis_memory (cluster_uid, memory_key, namespace, analysis, embedding)
		 VALUES ($1, $2, $3, $4, $5) RETURNING memory_key, namespace`,
		"cluster-a", memKey, wantNamespace, "cpu pressure", itVectorLiteral(1536),
	).Scan(&gotKey, &gotNS)
	if err != nil {
		t.Fatalf("insert analysis_memory error: %v", err)
	}

	if gotKey != memKey {
		t.Fatalf("analysis_memory memory_key: got %q, want %q", gotKey, memKey)
	}
	if gotNS != wantNamespace {
		t.Fatalf("analysis_memory namespace: got %q, want %q", gotNS, wantNamespace)
	}
}

// itRandHex returns a 32-char hex string from 16 random bytes, suitable for
// use as a unique test identifier without requiring external dependencies.
func itRandHex() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("itRandHex: rand.Read: %v", err))
	}

	return hex.EncodeToString(b[:])
}

// itVectorLiteral builds a pgvector-compatible string literal of dimension `size`
// filled with zeros (e.g. "[0,0,...,0]").
func itVectorLiteral(size int) string {
	if size <= 0 {
		return "[]"
	}

	buf := make([]byte, 0, size*2+2)
	buf = append(buf, '[')
	for i := range size {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, '0')
	}
	buf = append(buf, ']')

	return string(buf)
}
