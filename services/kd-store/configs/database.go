package configs

// DatabaseConfig holds PostgreSQL connection settings read from environment
// variables (or .env in development) via Viper.
//
// Environment variable mapping (Viper key → env var with prefix DB_):
//
//	database.host     → DB_HOST     (default: localhost)
//	database.port     → DB_PORT     (default: 5432)
//	database.name     → DB_NAME
//	database.user     → DB_USER
//	database.password → DB_PASSWORD
//	database.ssl_mode → DB_SSL_MODE (default: disable)
//	database.max_conns           → DB_MAX_CONNS           (default: 10)
//	database.min_conns           → DB_MIN_CONNS           (default: 2)
//	database.max_conn_lifetime   → DB_MAX_CONN_LIFETIME   (default: 1h)
//	database.max_conn_idle_time  → DB_MAX_CONN_IDLE_TIME  (default: 30m)
//	database.health_check_period → DB_HEALTH_CHECK_PERIOD (default: 1m)
type DatabaseConfig struct {
	Host              string `mapstructure:"host"`
	Port              int    `mapstructure:"port"`
	Name              string `mapstructure:"name"`
	User              string `mapstructure:"user"`
	Password          string `mapstructure:"password"`
	SSLMode           string `mapstructure:"ssl_mode"`
	MaxConns          int32  `mapstructure:"max_conns"`
	MinConns          int32  `mapstructure:"min_conns"`
	MaxConnLifetime   string `mapstructure:"max_conn_lifetime"`
	MaxConnIdleTime   string `mapstructure:"max_conn_idle_time"`
	HealthCheckPeriod string `mapstructure:"health_check_period"`
}
