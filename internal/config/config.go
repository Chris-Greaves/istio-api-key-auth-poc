// Package config loads service configuration from the environment.
package config

import (
	"errors"
	"os"
)

// Config holds the settings the service needs to start.
type Config struct {
	DatabaseURL string
	ListenAddr  string
	// PrometheusURL is the base URL of the Prometheus server the Web UI
	// queries for per-key usage graphs (ADR-0004). It's optional: unlike
	// DatabaseURL, an unset or unreachable Prometheus only degrades the
	// usage-graph feature (see internal/httpapi's usage handler), not the
	// service's core auth path, and standing up Prometheus itself is out of
	// scope for this service.
	PrometheusURL string
}

// Load reads configuration from environment variables.
func Load() (Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return Config{}, errors.New("DATABASE_URL environment variable is required")
	}

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	return Config{
		DatabaseURL:   dbURL,
		ListenAddr:    addr,
		PrometheusURL: os.Getenv("PROMETHEUS_URL"),
	}, nil
}
