package httpapi

import (
	"context"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/keys"
)

// ownerHeader is the response header the check endpoint injects on a
// successful auth, per ADR-0001, so the upstream service gets key attribution
// for free.
const ownerHeader = "X-API-Key-Owner"

// checkAuthzHandler is the Envoy ext_authz HTTP check service endpoint
// (ADR-0001): Istio calls it for every request the mesh wants authenticated,
// presenting the caller's key via the X-API-Key header. A 2xx response
// allows the request through; anything else denies it. Istio is configured
// to fail closed, so an internal error here also denies traffic rather than
// letting it through unchecked.
func checkAuthzHandler(store *keys.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := checkTracer.Start(r.Context(), "check_service.check")
		defer span.End()

		key := strings.TrimSpace(r.Header.Get("X-API-Key"))
		if key == "" {
			denyCheck(ctx, w, http.StatusUnauthorized, "missing key", "")
			return
		}

		result, err := keys.Validate(ctx, store, key)
		if err != nil {
			denyCheck(ctx, w, http.StatusInternalServerError, "internal error", "")
			return
		}

		if !result.Valid {
			denyCheck(ctx, w, http.StatusUnauthorized, result.Reason, result.KeyID)
			return
		}

		allowCheck(ctx, result.KeyID)
		w.Header().Set(ownerHeader, result.Owner)
		w.WriteHeader(http.StatusOK)
	}
}

// denyCheck records a denied decision for observability and writes status as
// the response. keyID is only ever a value keys.Validate has verified
// against a real record (e.g. an expired key) — never unverified caller
// input — so it's safe to surface in telemetry without letting an attacker
// mint arbitrary key_id label values.
func denyCheck(ctx context.Context, w http.ResponseWriter, status int, reason, keyID string) {
	recordDecision(ctx, "denied", reason, keyID)
	w.WriteHeader(status)
}

func allowCheck(ctx context.Context, keyID string) {
	recordDecision(ctx, "allowed", "", keyID)
}

// recordDecision tags the current span and increments the decisions counter
// with enough context — result, deny reason, and Key ID when known — to
// debug a denial from telemetry alone, without reading application logs.
func recordDecision(ctx context.Context, result, reason, keyID string) {
	attrs := []attribute.KeyValue{attribute.String("result", result)}
	if reason != "" {
		attrs = append(attrs, attribute.String("reason", reason))
	}
	if keyID != "" {
		attrs = append(attrs, attribute.String("key_id", keyID))
	}

	trace.SpanFromContext(ctx).SetAttributes(attrs...)
	checkDecisions.Add(ctx, 1, metric.WithAttributes(attrs...))
}
