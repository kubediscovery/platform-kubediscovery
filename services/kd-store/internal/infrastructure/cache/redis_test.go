package cache

import (
	"testing"
	"time"

	"github.com/kubediscovery/kd-store/configs"
)

func TestBuildOptions_RequiredAddr(t *testing.T) {
	_, err := buildOptions(configs.CacheConfig{Addr: ""})
	if err == nil {
		t.Error("buildOptions() expected error for empty addr, got nil")
	}
}

func TestBuildOptions_Defaults(t *testing.T) {
	cfg := configs.CacheConfig{
		Addr:     "localhost:6379",
		Password: "secret",
		DB:       1,
	}
	opts, err := buildOptions(cfg)
	if err != nil {
		t.Fatalf("buildOptions() unexpected error: %v", err)
	}
	if opts.Addr != "localhost:6379" {
		t.Errorf("Addr = %q, want %q", opts.Addr, "localhost:6379")
	}
	if opts.Password != "secret" {
		t.Errorf("Password = %q, want %q", opts.Password, "secret")
	}
	if opts.DB != 1 {
		t.Errorf("DB = %d, want 1", opts.DB)
	}
}

func TestBuildOptions_Timeouts(t *testing.T) {
	cfg := configs.CacheConfig{
		Addr:            "localhost:6379",
		MaxRetries:      5,
		MinRetryBackoff: "16ms",
		MaxRetryBackoff: "1s",
		DialTimeout:     "10s",
		ReadTimeout:     "5s",
		WriteTimeout:    "5s",
		PoolSize:        20,
		MinIdleConns:    4,
		PoolTimeout:     "8s",
	}
	opts, err := buildOptions(cfg)
	if err != nil {
		t.Fatalf("buildOptions() unexpected error: %v", err)
	}
	if opts.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", opts.MaxRetries)
	}
	if opts.MinRetryBackoff != 16*time.Millisecond {
		t.Errorf("MinRetryBackoff = %v, want 16ms", opts.MinRetryBackoff)
	}
	if opts.MaxRetryBackoff != time.Second {
		t.Errorf("MaxRetryBackoff = %v, want 1s", opts.MaxRetryBackoff)
	}
	if opts.DialTimeout != 10*time.Second {
		t.Errorf("DialTimeout = %v, want 10s", opts.DialTimeout)
	}
	if opts.ReadTimeout != 5*time.Second {
		t.Errorf("ReadTimeout = %v, want 5s", opts.ReadTimeout)
	}
	if opts.WriteTimeout != 5*time.Second {
		t.Errorf("WriteTimeout = %v, want 5s", opts.WriteTimeout)
	}
	if opts.PoolSize != 20 {
		t.Errorf("PoolSize = %d, want 20", opts.PoolSize)
	}
	if opts.MinIdleConns != 4 {
		t.Errorf("MinIdleConns = %d, want 4", opts.MinIdleConns)
	}
	if opts.PoolTimeout != 8*time.Second {
		t.Errorf("PoolTimeout = %v, want 8s", opts.PoolTimeout)
	}
}

func TestBuildOptions_InvalidDurations(t *testing.T) {
	cases := []struct {
		name string
		cfg  configs.CacheConfig
	}{
		{
			name: "invalid min_retry_backoff",
			cfg:  configs.CacheConfig{Addr: "localhost:6379", MinRetryBackoff: "notaduration"},
		},
		{
			name: "invalid max_retry_backoff",
			cfg:  configs.CacheConfig{Addr: "localhost:6379", MaxRetryBackoff: "bad"},
		},
		{
			name: "invalid dial_timeout",
			cfg:  configs.CacheConfig{Addr: "localhost:6379", DialTimeout: "x"},
		},
		{
			name: "invalid read_timeout",
			cfg:  configs.CacheConfig{Addr: "localhost:6379", ReadTimeout: "y"},
		},
		{
			name: "invalid write_timeout",
			cfg:  configs.CacheConfig{Addr: "localhost:6379", WriteTimeout: "z"},
		},
		{
			name: "invalid pool_timeout",
			cfg:  configs.CacheConfig{Addr: "localhost:6379", PoolTimeout: "bad-duration"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildOptions(tc.cfg)
			if err == nil {
				t.Errorf("buildOptions() expected error for %s, got nil", tc.name)
			}
		})
	}
}
