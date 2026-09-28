package config_test

import (
	"testing"

	"github.com/cjgreaves97/istio-api-key-auth-poc/internal/config"
)

func TestLoad_RequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when DATABASE_URL is not set")
	}
}

func TestLoad_ReadsDatabaseURLFromEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://example" {
		t.Fatalf("expected DatabaseURL %q, got %q", "postgres://example", cfg.DatabaseURL)
	}
}

func TestLoad_DefaultsListenAddr(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("LISTEN_ADDR", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Fatalf("expected default ListenAddr %q, got %q", ":8080", cfg.ListenAddr)
	}
}
