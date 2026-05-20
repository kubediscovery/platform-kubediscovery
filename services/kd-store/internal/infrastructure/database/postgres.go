// Package database provides the PostgreSQL connection pool for kd-store.
//
// Pool lifecycle is managed by UberFX: the pool is created on application
// start and cleanly closed on stop.  All configuration is read from the
// DatabaseConfig struct that is populated by Viper at boot time.
package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-store/configs"
)

// defaultSSLMode is the SSL mode applied when none is specified in config.
const defaultSSLMode = "disable"

// Pool is a type alias so callers can depend on the concrete *pgxpool.Pool
// directly while still having a clear import boundary.
type Pool = pgxpool.Pool

// Params groups the FX-injected inputs for NewPool.
// Migrator is included as a dependency so that FX runs its OnStart lifecycle
// hook (which applies pending migrations) before running the pool's OnStart hook.
type Params struct {
	fx.In

	LC       fx.Lifecycle
	Config   *configs.Config
	Logger   *slog.Logger
	Migrator *Migrator
}

// NewPool constructs a *pgxpool.Pool from the injected DatabaseConfig,
// registers FX lifecycle hooks to connect on start and close on stop, and
// performs an initial ping to fail fast if the database is unreachable.
func NewPool(p Params) (*Pool, error) {
	cfg := p.Config.Database

	connStr, err := buildConnString(cfg)
	if err != nil {
		return nil, fmt.Errorf("database: build connection string: %w", err)
	}

	poolCfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("database: parse pool config: %w", err)
	}

	if err := applyPoolSettings(poolCfg, cfg); err != nil {
		return nil, fmt.Errorf("database: apply pool settings: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("database: create pool: %w", err)
	}

	p.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := pool.Ping(ctx); err != nil {
				return fmt.Errorf("database: ping failed: %w", err)
			}
			p.Logger.Info("database pool connected",
				slog.String("host", cfg.Host),
				slog.Int("port", cfg.Port),
				slog.String("dbname", cfg.Name),
				slog.Int("max_conns", int(cfg.MaxConns)),
			)
			return nil
		},
		OnStop: func(_ context.Context) error {
			pool.Close()
			p.Logger.Info("database pool closed")
			return nil
		},
	})

	return pool, nil
}

// buildConnString assembles a PostgreSQL DSN from DatabaseConfig fields.
func buildConnString(cfg configs.DatabaseConfig) (string, error) {
	if cfg.Host == "" {
		return "", errors.New("database host is required")
	}
	if cfg.Name == "" {
		return "", errors.New("database name is required")
	}
	if cfg.User == "" {
		return "", errors.New("database user is required")
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.Name, cfg.User, sslMode(cfg.SSLMode),
	)
	if cfg.Password != "" {
		dsn += " password=" + cfg.Password
	}
	return dsn, nil
}

// applyPoolSettings configures pool-level tuning parameters from cfg.
func applyPoolSettings(poolCfg *pgxpool.Config, cfg configs.DatabaseConfig) error {
	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		poolCfg.MinConns = cfg.MinConns
	}

	if cfg.MaxConnLifetime != "" {
		d, err := time.ParseDuration(cfg.MaxConnLifetime)
		if err != nil {
			return fmt.Errorf("invalid max_conn_lifetime %q: %w", cfg.MaxConnLifetime, err)
		}
		poolCfg.MaxConnLifetime = d
	}

	if cfg.MaxConnIdleTime != "" {
		d, err := time.ParseDuration(cfg.MaxConnIdleTime)
		if err != nil {
			return fmt.Errorf("invalid max_conn_idle_time %q: %w", cfg.MaxConnIdleTime, err)
		}
		poolCfg.MaxConnIdleTime = d
	}

	if cfg.HealthCheckPeriod != "" {
		d, err := time.ParseDuration(cfg.HealthCheckPeriod)
		if err != nil {
			return fmt.Errorf("invalid health_check_period %q: %w", cfg.HealthCheckPeriod, err)
		}
		poolCfg.HealthCheckPeriod = d
	}

	return nil
}

// sslMode returns defaultSSLMode when the configured value is empty.
func sslMode(s string) string {
	if s == "" {
		return defaultSSLMode
	}
	return s
}
