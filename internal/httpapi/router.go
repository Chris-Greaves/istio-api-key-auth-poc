// Package httpapi wires up the service's HTTP surfaces.
package httpapi

import (
	"database/sql"
	"net/http"

	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/keys"
	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/webui"
)

// NewRouter builds the top-level HTTP handler for the service.
func NewRouter(db *sql.DB) http.Handler {
	store := keys.NewStore(db)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthHandler(db))
	mux.HandleFunc("POST /api/keys", createKeyHandler(store))
	mux.HandleFunc("GET /api/keys", listKeysHandler(store))
	mux.HandleFunc("DELETE /api/keys/{keyID}", revokeKeyHandler(store))
	mux.HandleFunc("POST /api/keys/validate", validateKeyHandler(store))
	mux.HandleFunc("/authz/check", checkAuthzHandler(store))
	mux.Handle("/", webui.Handler())
	return mux
}
