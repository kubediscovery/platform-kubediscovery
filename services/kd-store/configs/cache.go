package configs

// CacheConfig holds Redis connection settings read from environment variables
// (or .env in development) via Viper.
//
// Environment variable mapping (Viper key → env var):
//
//	cache.addr              → CACHE_ADDR              (default: localhost:6379)
//	cache.password          → CACHE_PASSWORD          (default: "")
//	cache.db                → CACHE_DB                (default: 0)
//	cache.max_retries       → CACHE_MAX_RETRIES       (default: 3)
//	cache.min_retry_backoff → CACHE_MIN_RETRY_BACKOFF (default: 8ms)
//	cache.max_retry_backoff → CACHE_MAX_RETRY_BACKOFF (default: 512ms)
//	cache.dial_timeout      → CACHE_DIAL_TIMEOUT      (default: 5s)
//	cache.read_timeout      → CACHE_READ_TIMEOUT      (default: 3s)
//	cache.write_timeout     → CACHE_WRITE_TIMEOUT     (default: 3s)
//	cache.pool_size         → CACHE_POOL_SIZE         (default: 10)
//	cache.min_idle_conns    → CACHE_MIN_IDLE_CONNS    (default: 2)
//	cache.pool_timeout      → CACHE_POOL_TIMEOUT      (default: 4s)
type CacheConfig struct {
	Addr            string `mapstructure:"addr"`
	Password        string `mapstructure:"password"`
	DB              int    `mapstructure:"db"`
	MaxRetries      int    `mapstructure:"max_retries"`
	MinRetryBackoff string `mapstructure:"min_retry_backoff"`
	MaxRetryBackoff string `mapstructure:"max_retry_backoff"`
	DialTimeout     string `mapstructure:"dial_timeout"`
	ReadTimeout     string `mapstructure:"read_timeout"`
	WriteTimeout    string `mapstructure:"write_timeout"`
	PoolSize        int    `mapstructure:"pool_size"`
	MinIdleConns    int    `mapstructure:"min_idle_conns"`
	PoolTimeout     string `mapstructure:"pool_timeout"`
}
