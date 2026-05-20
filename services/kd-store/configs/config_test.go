package configs

import (
	"testing"
)

func TestNew_Defaults(t *testing.T) {
	cfg, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if cfg.App.Name != "kd-store" {
		t.Errorf("App.Name = %q, want %q", cfg.App.Name, "kd-store")
	}
	if cfg.App.Environment != "development" {
		t.Errorf("App.Environment = %q, want %q", cfg.App.Environment, "development")
	}
	if cfg.Database.Host != "localhost" {
		t.Errorf("Database.Host = %q, want %q", cfg.Database.Host, "localhost")
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("Database.Port = %d, want 5432", cfg.Database.Port)
	}
	if cfg.Database.SSLMode != "disable" {
		t.Errorf("Database.SSLMode = %q, want %q", cfg.Database.SSLMode, "disable")
	}
	if cfg.Database.MaxConns != 10 {
		t.Errorf("Database.MaxConns = %d, want 10", cfg.Database.MaxConns)
	}
	if cfg.Database.MinConns != 2 {
		t.Errorf("Database.MinConns = %d, want 2", cfg.Database.MinConns)
	}
}

func TestNew_EnvOverride(t *testing.T) {
	t.Setenv("DATABASE_HOST", "mydbhost")
	t.Setenv("DATABASE_PORT", "5433")
	t.Setenv("DATABASE_NAME", "override_db")

	cfg, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if cfg.Database.Host != "mydbhost" {
		t.Errorf("Database.Host = %q, want %q", cfg.Database.Host, "mydbhost")
	}
	if cfg.Database.Port != 5433 {
		t.Errorf("Database.Port = %d, want 5433", cfg.Database.Port)
	}
	if cfg.Database.Name != "override_db" {
		t.Errorf("Database.Name = %q, want %q", cfg.Database.Name, "override_db")
	}
}
