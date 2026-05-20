package database

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/kubediscovery/kd-store/configs"
)

// TestBuildMigrateDSN verifies that buildMigrateDSN constructs the correct
// pgx5:// DSN and returns errors when required fields are missing.
func TestBuildMigrateDSN(t *testing.T) {
	tests := []struct {
		name      string
		cfg       configs.DatabaseConfig
		wantErr   bool
		wantInDSN []string
	}{
		{
			name: "full config produces correct DSN",
			cfg: configs.DatabaseConfig{
				Host:    "localhost",
				Port:    5432,
				Name:    "testdb",
				User:    "testuser",
				SSLMode: "disable",
			},
			wantErr:   false,
			wantInDSN: []string{"pgx5://", "testuser", "localhost", "5432", "testdb", "sslmode=disable"},
		},
		{
			name: "with password included in DSN",
			cfg: configs.DatabaseConfig{
				Host:     "db.example.com",
				Port:     5433,
				Name:     "proddb",
				User:     "produser",
				Password: "s3cr3t",
				SSLMode:  "require",
			},
			wantErr:   false,
			wantInDSN: []string{"pgx5://", "produser", "s3cr3t", "db.example.com", "5433", "proddb", "sslmode=require"},
		},
		{
			name: "empty ssl_mode defaults to disable",
			cfg: configs.DatabaseConfig{
				Host:    "localhost",
				Port:    5432,
				Name:    "mydb",
				User:    "myuser",
				SSLMode: "",
			},
			wantErr:   false,
			wantInDSN: []string{"sslmode=disable"},
		},
		{
			name:    "missing host returns error",
			cfg:     configs.DatabaseConfig{Name: "db", User: "user"},
			wantErr: true,
		},
		{
			name:    "missing name returns error",
			cfg:     configs.DatabaseConfig{Host: "localhost", User: "user"},
			wantErr: true,
		},
		{
			name:    "missing user returns error",
			cfg:     configs.DatabaseConfig{Host: "localhost", Name: "db"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildMigrateDSN(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("buildMigrateDSN() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("buildMigrateDSN() unexpected error: %v", err)
			}
			for _, substr := range tt.wantInDSN {
				if !strings.Contains(got, substr) {
					t.Errorf("buildMigrateDSN() DSN %q does not contain %q", got, substr)
				}
			}
		})
	}
}

// TestSlogMigrateLogger verifies that the adapter satisfies the migrate.Logger
// interface and behaves as expected.
func TestSlogMigrateLogger(t *testing.T) {
	logger := slog.Default()
	adapter := slogMigrateLogger{logger: logger}

	t.Run("Verbose returns false", func(t *testing.T) {
		if adapter.Verbose() {
			t.Error("Verbose() should return false")
		}
	})

	t.Run("Printf does not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Printf() panicked: %v", r)
			}
		}()
		adapter.Printf("migration %d applied in %dms\n", 1, 42)
	})

	t.Run("Printf formats correctly without panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Printf() panicked with format: %v", r)
			}
		}()
		adapter.Printf("start buffering %s\n", "000001_create_clusters_table")
	})
}

// TestMigrationFilesEmbedded verifies that the embedded FS contains the
// expected migration files so that missing files are detected at test time
// rather than at runtime.
func TestMigrationFilesEmbedded(t *testing.T) {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		t.Fatalf("ReadDir(migrations) error: %v", err)
	}

	if len(entries) == 0 {
		t.Fatal("expected at least one embedded migration file, got none")
	}

	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			t.Errorf("unexpected non-SQL file in migrations: %q", name)
		}
	}
}
