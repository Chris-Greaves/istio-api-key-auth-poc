// Package httpapi wires up the service's HTTP surfaces.
package httpapi

import (
	"database/sql"
	"net/http"

	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/keys"
	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/metricsquery"
	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/webui"
)

// NewRouter builds the top-level HTTP handler for the service. metrics is
// served at /metrics in Prometheus exposition format (ticket 05). querier
// sources the per-key usage-graph endpoint's data live from Prometheus
// (ticket 06) — Postgres is never consulted on that path.
func NewRouter(db *sql.DB, metrics http.Handler, querier metricsquery.MetricsQuerier) http.Handler {
	store := keys.NewStore(db)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler(db))
	mux.Handle("GET /metrics", metrics)
	mux.HandleFunc("POST /api/keys", createKeyHandler(store))
	mux.HandleFunc("GET /api/keys", listKeysHandler(store))
	mux.HandleFunc("DELETE /api/keys/{keyID}", revokeKeyHandler(store))
	mux.HandleFunc("GET /api/keys/{keyID}/usage", usageHandler(querier))
	mux.HandleFunc("POST /api/keys/validate", validateKeyHandler(store))
	mux.HandleFunc("/authz/check", checkAuthzHandler(store))
	mux.Handle("/", webui.Handler())
	return mux
}
