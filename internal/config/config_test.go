package config_test

import (
	"testing"

	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/config"
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

func TestLoad_ReadsPrometheusURLFromEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("PROMETHEUS_URL", "http://prometheus:9090")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PrometheusURL != "http://prometheus:9090" {
		t.Fatalf("expected PrometheusURL %q, got %q", "http://prometheus:9090", cfg.PrometheusURL)
	}
}

func TestLoad_DoesNotRequirePrometheusURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("PROMETHEUS_URL", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PrometheusURL != "" {
		t.Fatalf("expected empty PrometheusURL by default, got %q", cfg.PrometheusURL)
	}
}
