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

	if cfg.MinRetryBackoff != "" {
		d, err := time.ParseDuration(cfg.MinRetryBackoff)
		if err != nil {
			return nil, fmt.Errorf("invalid min_retry_backoff %q: %w", cfg.MinRetryBackoff, err)
		}
		opts.MinRetryBackoff = d
	}

	if cfg.MaxRetryBackoff != "" {
		d, err := time.ParseDuration(cfg.MaxRetryBackoff)
		if err != nil {
			return nil, fmt.Errorf("invalid max_retry_backoff %q: %w", cfg.MaxRetryBackoff, err)
		}
		opts.MaxRetryBackoff = d
	}

	if cfg.DialTimeout != "" {
		d, err := time.ParseDuration(cfg.DialTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid dial_timeout %q: %w", cfg.DialTimeout, err)
		}
		opts.DialTimeout = d
	}

	if cfg.ReadTimeout != "" {
		d, err := time.ParseDuration(cfg.ReadTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid read_timeout %q: %w", cfg.ReadTimeout, err)
		}
		opts.ReadTimeout = d
	}

	if cfg.WriteTimeout != "" {
		d, err := time.ParseDuration(cfg.WriteTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid write_timeout %q: %w", cfg.WriteTimeout, err)
		}
		opts.WriteTimeout = d
	}

	if cfg.PoolTimeout != "" {
		d, err := time.ParseDuration(cfg.PoolTimeout)
		if err != nil {
			return nil, fmt.Errorf("invalid pool_timeout %q: %w", cfg.PoolTimeout, err)
		}
		opts.PoolTimeout = d
	}

	return opts, nil
}
