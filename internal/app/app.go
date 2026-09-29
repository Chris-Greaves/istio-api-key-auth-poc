// Package app wires the service's dependencies into a runnable application.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/config"
	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/database"
	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/httpapi"
	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/metricsquery"
	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/telemetry"
)

// App holds the fully wired, ready-to-serve application.
type App struct {
	db      *sql.DB
	handler http.Handler
}

// New connects to Postgres, applies schema migrations, and wires up the HTTP handler.
// It returns an error rather than a partially-started App if the database is
// unreachable or migrations fail, so callers never serve traffic against a broken schema.
//
// If querier is nil, a Prometheus-backed MetricsQuerier is built from
// cfg.PrometheusURL; ticket 06's tests pass an in-memory fake instead, to
// stub Prometheus responses while still exercising the rest of the fully
// wired binary.
func New(ctx context.Context, cfg config.Config, querier metricsquery.MetricsQuerier) (*App, error) {
	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	if err := database.Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("applying migrations: %w", err)
	}

	metrics, err := telemetry.Setup()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("setting up telemetry: %w", err)
	}

	if querier == nil {
		querier = metricsquery.NewPrometheusQuerier(cfg.PrometheusURL)
	}

	return &App{
		db:      db,
		handler: httpapi.NewRouter(db, metrics.Handler, querier),
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
