package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kubediscovery/kd-store/configs"
)

func TestPostgresMigrationsAndBasicCRUD(t *testing.T) {
	t.Parallel()

	cfg, ok := integrationDatabaseConfigFromEnv(t)
	if !ok {
		t.Skip("set KD_STORE_IT_DB_HOST, KD_STORE_IT_DB_PORT, KD_STORE_IT_DB_NAME, KD_STORE_IT_DB_USER and optionally KD_STORE_IT_DB_PASSWORD/KD_STORE_IT_DB_SSLMODE to run integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	m, err := buildMigrate(cfg, slog.Default())
	if err != nil {
		t.Fatalf("build migrate: %v", err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("run migrations: %v", err)
	}

	connStr, err := buildConnString(cfg)
	if err != nil {
		t.Fatalf("build conn string: %v", err)
	}
	poolCfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	defer pool.Close()

	clusterID := "8c45a6ce-35ca-432f-85c6-38be1ffc98de"
	if _, err := pool.Exec(ctx, `INSERT INTO clusters (uid, name, environment, status) VALUES ($1, $2, $3, $4)`, clusterID, "cluster-a", "staging", "active"); err != nil {
		t.Fatalf("insert clusters: %v", err)
	}
	var gotName, gotEnv, gotStatus string
	if err := pool.QueryRow(ctx, `SELECT name, environment, status FROM clusters WHERE uid = $1`, clusterID).Scan(&gotName, &gotEnv, &gotStatus); err != nil {
		t.Fatalf("select clusters: %v", err)
	}
	if gotName != "cluster-a" || gotEnv != "staging" || gotStatus != "active" {
		t.Fatalf("unexpected cluster row: name=%q env=%q status=%q", gotName, gotEnv, gotStatus)
	}
	if _, err := pool.Exec(ctx, `UPDATE clusters SET status = $1 WHERE uid = $2`, "inactive", clusterID); err != nil {
		t.Fatalf("update clusters: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM clusters WHERE uid = $1`, clusterID).Scan(&gotStatus); err != nil {
		t.Fatalf("select updated status: %v", err)
	}
	if gotStatus != "inactive" {
		t.Fatalf("status not updated, got=%q", gotStatus)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM clusters WHERE uid = $1`, clusterID); err != nil {
		t.Fatalf("delete clusters: %v", err)
	}

	var eventID int64
	if err := pool.QueryRow(ctx, `INSERT INTO events (cluster_uid, memory_key, severity, diagnosis, recommendations) VALUES ($1,$2,$3,$4,$5) RETURNING id`, clusterID, "c1-staging-default", "warning", "pod pending", "check scheduler").Scan(&eventID); err != nil {
		t.Fatalf("insert events: %v", err)
	}
	var diagnosis string
	if err := pool.QueryRow(ctx, `SELECT diagnosis FROM events WHERE id = $1`, eventID).Scan(&diagnosis); err != nil {
		t.Fatalf("select events: %v", err)
	}
	if diagnosis != "pod pending" {
		t.Fatalf("unexpected diagnosis: %q", diagnosis)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM events WHERE id = $1`, eventID); err != nil {
		t.Fatalf("delete events: %v", err)
	}

	vector := "[0.1,0.2,0.3" + strings.Repeat(",0", 1533) + "]"
	var memoryID int64
	if err := pool.QueryRow(ctx, `INSERT INTO analysis_memory (cluster_uid, memory_key, namespace, analysis, embedding) VALUES ($1,$2,$3,$4,$5::vector) RETURNING id`, clusterID, "c1-staging-default", "default", "analysis text", vector).Scan(&memoryID); err != nil {
		t.Fatalf("insert analysis_memory: %v", err)
	}
	var analysis string
	if err := pool.QueryRow(ctx, `SELECT analysis FROM analysis_memory WHERE id = $1`, memoryID).Scan(&analysis); err != nil {
		t.Fatalf("select analysis_memory: %v", err)
	}
	if analysis != "analysis text" {
		t.Fatalf("unexpected analysis: %q", analysis)
	}
	if _, err := pool.Exec(ctx, `UPDATE analysis_memory SET analysis = $1 WHERE id = $2`, "analysis updated", memoryID); err != nil {
		t.Fatalf("update analysis_memory: %v", err)
	}
}

func integrationDatabaseConfigFromEnv(t *testing.T) (configs.DatabaseConfig, bool) {
	t.Helper()

	host := os.Getenv("KD_STORE_IT_DB_HOST")
	port := os.Getenv("KD_STORE_IT_DB_PORT")
	name := os.Getenv("KD_STORE_IT_DB_NAME")
	user := os.Getenv("KD_STORE_IT_DB_USER")
	if host == "" || port == "" || name == "" || user == "" {
		return configs.DatabaseConfig{}, false
	}

	portInt := 5432
	if _, err := fmt.Sscanf(port, "%d", &portInt); err != nil {
		t.Fatalf("invalid KD_STORE_IT_DB_PORT %q: %v", port, err)
	}

	return configs.DatabaseConfig{
		Host:     host,
		Port:     portInt,
		Name:     name,
		User:     user,
		Password: os.Getenv("KD_STORE_IT_DB_PASSWORD"),
		SSLMode:  os.Getenv("KD_STORE_IT_DB_SSLMODE"),
	}, true
}
