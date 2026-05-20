// Package cache provides the Redis client for kd-store.
//
// Client lifecycle is managed by UberFX: the client is created on application
// start and cleanly closed on stop. All configuration is read from the
// CacheConfig struct that is populated by Viper at boot time.
package cache

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"github.com/kubediscovery/kd-store/configs"
)

// Client is a type alias so callers can depend on *redis.Client directly while
// keeping a clear import boundary.
type Client = redis.Client

// Params groups the FX-injected inputs for NewClient.
type Params struct {
	fx.In

	LC     fx.Lifecycle
	Config *configs.Config
	Logger *slog.Logger
}

// NewClient constructs a *redis.Client from the injected CacheConfig,
// registers FX lifecycle hooks to ping on start and close on stop.
func NewClient(p Params) (*Client, error) {
	cfg := p.Config.Cache

	opts, err := buildOptions(cfg)
	if err != nil {
		return nil, fmt.Errorf("cache: build options: %w", err)
	}

	client := redis.NewClient(opts)

	p.LC.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := client.Ping(ctx).Err(); err != nil {
				return fmt.Errorf("cache: ping failed: %w", err)
			}
			p.Logger.Info("redis client connected",
				slog.String("addr", cfg.Addr),
				slog.Int("db", cfg.DB),
				slog.Int("pool_size", cfg.PoolSize),
			)
			return nil
		},
		OnStop: func(_ context.Context) error {
			if err := client.Close(); err != nil {
				return fmt.Errorf("cache: close failed: %w", err)
			}
			p.Logger.Info("redis client closed")
			return nil
		},
	})

	return client, nil
}

// buildOptions assembles redis.Options from CacheConfig fields.
func buildOptions(cfg configs.CacheConfig) (*redis.Options, error) {
	if cfg.Addr == "" {
		return nil, errors.New("cache addr is required")
	}

	opts := &redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	}

	if cfg.MaxRetries > 0 {
		opts.MaxRetries = cfg.MaxRetries
	}
	if cfg.PoolSize > 0 {
		opts.PoolSize = cfg.PoolSize
	}
	if cfg.MinIdleConns > 0 {
		opts.MinIdleConns = cfg.MinIdleConns
	}

	setters := []struct {
		raw   string
		field string
		set   func(time.Duration)
	}{
		{cfg.MinRetryBackoff, "min_retry_backoff", func(d time.Duration) { opts.MinRetryBackoff = d }},
		{cfg.MaxRetryBackoff, "max_retry_backoff", func(d time.Duration) { opts.MaxRetryBackoff = d }},
		{cfg.DialTimeout, "dial_timeout", func(d time.Duration) { opts.DialTimeout = d }},
		{cfg.ReadTimeout, "read_timeout", func(d time.Duration) { opts.ReadTimeout = d }},
		{cfg.WriteTimeout, "write_timeout", func(d time.Duration) { opts.WriteTimeout = d }},
		{cfg.PoolTimeout, "pool_timeout", func(d time.Duration) { opts.PoolTimeout = d }},
	}

	for _, s := range setters {
		if err := parseDuration(s.raw, s.field, s.set); err != nil {
			return nil, err
		}
	}

	return opts, nil
}

// parseDuration parses a non-empty duration string and assigns it via the
// provided setter. It is a no-op when raw is empty.
func parseDuration(raw, field string, set func(time.Duration)) error {
	if raw == "" {
		return nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("invalid %s %q: %w", field, raw, err)
	}
	set(d)
	return nil
}
