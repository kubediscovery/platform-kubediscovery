// Package database provides PostgreSQL connection and migration support for kd-store.
//
// Migration SQL files are embedded into the binary at compile time via the embed
// package.  On application startup golang-migrate reads the embedded FS and applies
// all pending migrations in version order, before any other component is available.
package database

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-store/configs"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// MigratorParams groups FX-injected inputs for NewMigrator.
type MigratorParams struct {
	fx.In

	LC     fx.Lifecycle
	Config *configs.Config
	Logger *slog.Logger
}

// Migrator wraps a golang-migrate instance and exposes RunMigrations.
// Its lifecycle is managed by FX: RunMigrations is called in OnStart so that
// the schema is always current before application components accept traffic.
type Migrator struct {
	m      *migrate.Migrate
	logger *slog.Logger
}

// NewMigrator constructs a Migrator from the injected configuration and
// registers an FX OnStart hook that applies pending migrations.
// Schema rollback is intentionally omitted from OnStop — it is an explicit
// operator action and must not happen automatically on every shutdown.
func NewMigrator(p MigratorParams) (*Migrator, error) {
	m, err := buildMigrate(p.Config.Database, p.Logger)
	if err != nil {
		return nil, fmt.Errorf("migrations: initialise: %w", err)
	}

	migrator := &Migrator{m: m, logger: p.Logger}

	p.LC.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			return migrator.RunMigrations()
		},
		OnStop: func(_ context.Context) error {
			srcErr, dbErr := migrator.m.Close()
			if srcErr != nil {
				p.Logger.Warn("migrations: error closing source", slog.Any("error", srcErr))
			}
			if dbErr != nil {
				p.Logger.Warn("migrations: error closing database connection", slog.Any("error", dbErr))
			}
			return nil
		},
	})

	return migrator, nil
}

// RunMigrations applies all pending migrations to the database.
// The call is idempotent: if the schema is already at the latest version it
// returns nil immediately.  A dirty database (failed mid-migration) causes an
// error so the operator can intervene manually.
func (mr *Migrator) RunMigrations() error {
	version, dirty, err := mr.m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("migrations: read current version: %w", err)
	}

	if dirty {
		return fmt.Errorf(
			"migrations: database is in dirty state at version %d — resolve manually and re-deploy",
			version,
		)
	}

	if err := mr.m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			mr.logger.Info("migrations: schema already up to date", slog.Uint64("version", uint64(version)))
			return nil
		}
		return fmt.Errorf("migrations: apply up: %w", err)
	}

	newVersion, _, _ := mr.m.Version()
	mr.logger.Info("migrations: applied successfully", slog.Uint64("version", uint64(newVersion)))
	return nil
}

// Version delegates to the underlying migrate.Migrate instance, returning the
// current schema version and dirty flag.
func (mr *Migrator) Version() (uint, bool, error) {
	return mr.m.Version()
}

// slogMigrateLogger adapts *slog.Logger to the migrate.Logger interface so that
// golang-migrate log output is routed through the application's structured logger.
type slogMigrateLogger struct {
	logger *slog.Logger
}

func (l slogMigrateLogger) Printf(format string, v ...interface{}) {
	l.logger.Debug(fmt.Sprintf(format, v...))
}

func (l slogMigrateLogger) Verbose() bool { return false }

// buildMigrate constructs a *migrate.Migrate using the embedded SQL files as
// source and the pgx/v5 driver for the database connection.
func buildMigrate(cfg configs.DatabaseConfig, logger *slog.Logger) (*migrate.Migrate, error) {
	src, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("create iofs source: %w", err)
	}

	dsn, err := buildMigrateDSN(cfg)
	if err != nil {
		return nil, err
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return nil, fmt.Errorf("create migrate instance: %w", err)
	}

	m.Log = slogMigrateLogger{logger: logger}

	return m, nil
}

// buildMigrateDSN constructs a DSN string for the golang-migrate pgx/v5 driver.
// The scheme must be "pgx5" — the driver rewrites it to "postgres" internally
// before opening the sql.DB connection.
func buildMigrateDSN(cfg configs.DatabaseConfig) (string, error) {
	if cfg.Host == "" {
		return "", fmt.Errorf("migrations: database host is required")
	}
	if cfg.Name == "" {
		return "", fmt.Errorf("migrations: database name is required")
	}
	if cfg.User == "" {
		return "", fmt.Errorf("migrations: database user is required")
	}

	dsn := fmt.Sprintf(
		"pgx5://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Name,
		sslMode(cfg.SSLMode),
	)

	return dsn, nil
}
