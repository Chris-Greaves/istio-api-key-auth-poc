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

	return Config{DatabaseURL: dbURL, ListenAddr: addr}, nil
}
