// Package app wires the service's dependencies into a runnable application.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/cjgreaves97/istio-api-key-auth-poc/internal/config"
	"github.com/cjgreaves97/istio-api-key-auth-poc/internal/database"
	"github.com/cjgreaves97/istio-api-key-auth-poc/internal/httpapi"
)

// App holds the fully wired, ready-to-serve application.
type App struct {
	db      *sql.DB
	handler http.Handler
}

// New connects to Postgres, applies schema migrations, and wires up the HTTP handler.
// It returns an error rather than a partially-started App if the database is
// unreachable or migrations fail, so callers never serve traffic against a broken schema.
func New(ctx context.Context, cfg config.Config) (*App, error) {
	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	if err := database.Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("applying migrations: %w", err)
	}

	return &App{
		db:      db,
		handler: httpapi.NewRouter(db),
	}, nil
}

// Handler returns the application's top-level HTTP handler.
func (a *App) Handler() http.Handler {
	return a.handler
}

// Close releases the application's resources.
func (a *App) Close() error {
	return a.db.Close()
}
