package database

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kubediscovery/kd-store/configs"
)

func TestBuildConnString(t *testing.T) {
	tests := []struct {
		name    string
		cfg     configs.DatabaseConfig
		want    string
		wantErr bool
	}{
		{
			name: "all required fields",
			cfg: configs.DatabaseConfig{
				Host:    testHost,
				Port:    5432,
				Name:    "testdb",
				User:    "testuser",
				SSLMode: defaultSSLMode,
			},
			want:    "host=localhost port=5432 dbname=testdb user=testuser sslmode=disable",
			wantErr: false,
		},
		{
			name: "with password",
			cfg: configs.DatabaseConfig{
				Host:     "db.example.com",
				Port:     5432,
				Name:     "proddb",
				User:     "produser",
				Password: "secret",
				SSLMode:  testSSLRequire,
			},
			want:    "host=db.example.com port=5432 dbname=proddb user=produser sslmode=require password=secret",
			wantErr: false,
		},
		{
			name: "empty ssl_mode defaults to disable",
			cfg: configs.DatabaseConfig{
				Host:    testHost,
				Port:    5432,
				Name:    "mydb",
				User:    "myuser",
				SSLMode: "",
			},
			want:    "host=localhost port=5432 dbname=mydb user=myuser sslmode=disable",
			wantErr: false,
		},
		{
			name:    "missing host returns error",
			cfg:     configs.DatabaseConfig{Name: "db", User: testUser},
			wantErr: true,
		},
		{
			name:    "missing name returns error",
			cfg:     configs.DatabaseConfig{Host: testHost, User: testUser},
			wantErr: true,
		},
		{
			name:    "missing user returns error",
			cfg:     configs.DatabaseConfig{Host: testHost, Name: "db"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildConnString(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("buildConnString() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("buildConnString() unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("buildConnString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplyPoolSettings(t *testing.T) {
	tests := []struct {
		name    string
		cfg     configs.DatabaseConfig
		check   func(t *testing.T, pc *pgxpool.Config)
		wantErr bool
	}{
		{
			name: "default values applied",
			cfg: configs.DatabaseConfig{
				MaxConns:          10,
				MinConns:          2,
				MaxConnLifetime:   "1h",
				MaxConnIdleTime:   "30m",
				HealthCheckPeriod: "1m",
			},
			check: func(t *testing.T, pc *pgxpool.Config) {
				t.Helper()
				if pc.MaxConns != 10 {
					t.Errorf("MaxConns = %d, want 10", pc.MaxConns)
				}
				if pc.MinConns != 2 {
					t.Errorf("MinConns = %d, want 2", pc.MinConns)
				}
				if pc.MaxConnLifetime != time.Hour {
					t.Errorf("MaxConnLifetime = %v, want 1h", pc.MaxConnLifetime)
				}
				if pc.MaxConnIdleTime != 30*time.Minute {
					t.Errorf("MaxConnIdleTime = %v, want 30m", pc.MaxConnIdleTime)
				}
				if pc.HealthCheckPeriod != time.Minute {
					t.Errorf("HealthCheckPeriod = %v, want 1m", pc.HealthCheckPeriod)
				}
			},
		},
		{
			name: "invalid max_conn_lifetime returns error",
			cfg: configs.DatabaseConfig{
				MaxConnLifetime: "not-a-duration",
			},
			wantErr: true,
		},
		{
			name: "invalid max_conn_idle_time returns error",
			cfg: configs.DatabaseConfig{
				MaxConnIdleTime: "bad",
			},
			wantErr: true,
		},
		{
			name: "invalid health_check_period returns error",
			cfg: configs.DatabaseConfig{
				HealthCheckPeriod: "x",
			},
			wantErr: true,
		},
		{
			name: "zero MaxConns is skipped",
			cfg:  configs.DatabaseConfig{},
			check: func(t *testing.T, pc *pgxpool.Config) {
				t.Helper()
				// pgxpool defaults should remain untouched
				if pc.MaxConns < 0 {
					t.Errorf("unexpected negative MaxConns: %d", pc.MaxConns)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := &pgxpool.Config{}
			err := applyPoolSettings(pc, tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("applyPoolSettings() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("applyPoolSettings() unexpected error: %v", err)
				return
			}
			if tt.check != nil {
				tt.check(t, pc)
			}
		})
	}
}

func TestSSLMode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", defaultSSLMode},
		{defaultSSLMode, defaultSSLMode},
		{testSSLRequire, testSSLRequire},
		{"verify-full", "verify-full"},
	}
	for _, tt := range tests {
		got := sslMode(tt.input)
		if got != tt.want {
			t.Errorf("sslMode(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
