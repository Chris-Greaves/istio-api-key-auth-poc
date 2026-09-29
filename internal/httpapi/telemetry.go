package httpapi

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// instrumentationName identifies this package's tracer and meter. The global
// providers export through whatever telemetry.Setup installed at startup, so
// every handler in this package — check and management alike — shares one
// exporter pipeline without needing package-specific wiring.
const instrumentationName = "github.com/Chris-Greaves/istio-api-key-auth-poc/internal/httpapi"

var tracer trace.Tracer = otel.Tracer(instrumentationName)

var checkDecisions metric.Int64Counter = mustInt64Counter(
	"check_service.decisions",
	"Number of ext_authz check decisions made by the check endpoint, by result.",
)

var managementRequests metric.Int64Counter = mustInt64Counter(
	"management_api.requests",
	"Number of management API requests handled, by operation and result.",
)

// recordEvent tags the current span and increments counter with attrs, plus
// a key_id label when keyID is non-empty. keyID must already be a value the
// store has confirmed as real (e.g. a freshly created, validated, or revoked
// key) — never unverified caller input — so callers never let an attacker
// mint arbitrary key_id label values.
func recordEvent(ctx context.Context, counter metric.Int64Counter, keyID string, attrs ...attribute.KeyValue) {
	if keyID != "" {
		attrs = append(attrs, attribute.String("key_id", keyID))
	}

	trace.SpanFromContext(ctx).SetAttributes(attrs...)
	counter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// recordManagementRequest records a create/list/revoke call's outcome on the
// management-requests counter.
func recordManagementRequest(ctx context.Context, operation, result, keyID string) {
	recordEvent(ctx, managementRequests, keyID,
		attribute.String("operation", operation),
		attribute.String("result", result),
	)
}

// failManagement writes a JSON error response and records the failed
// management request for observability in one call, mirroring denyCheck's
// pairing of the same two concerns for the check endpoint. It never accepts
// a keyID, since every caller of failManagement is on a path where the
// store hasn't confirmed one (see recordEvent) — a successful call records
// its own telemetry directly via recordManagementRequest instead.
func failManagement(ctx context.Context, w http.ResponseWriter, status int, message, operation, result string) {
	writeJSONError(w, status, message)
	recordManagementRequest(ctx, operation, result, "")
}

func mustInt64Counter(name, description string) metric.Int64Counter {
	counter, err := otel.Meter(instrumentationName).Int64Counter(name, metric.WithDescription(description))
	if err != nil {
		// Only fails on invalid instrument configuration, which is a
		// programmer error caught immediately at startup.
		panic(err)
	}
	return counter
}
