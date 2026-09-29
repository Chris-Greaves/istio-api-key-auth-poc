package httpapi

import (
	"net/http"
	"time"

	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/metricsquery"
)

// defaultUsageRange and defaultUsageStep are used when the caller omits
// start/end/step, giving the Web UI a graph without it having to compute a
// sensible default range itself.
const (
	defaultUsageRange = time.Hour
	defaultUsageStep  = time.Minute
)

// maxUsagePoints bounds how many samples a single request can ask Prometheus
// for. Required because this endpoint, like the rest of the management API
// (ADR-0003: no Web UI auth for the PoC), is unauthenticated — without a
// cap, a caller could request an arbitrarily long range at an arbitrarily
// fine step and force expensive Prometheus queries.
const maxUsagePoints = 1440 // a day at one-minute resolution

type usagePointResponse struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// usageHandler serves a key's usage as a time series sourced live from
// Prometheus via MetricsQuerier — Postgres is never consulted on this path
// (ADR-0004), so keyID is never checked against the store and must not be
// forwarded as a metric label the way other handlers' confirmed Key IDs are
// (see recordEvent): an unauthenticated caller could otherwise mint
// arbitrary key_id label values through this endpoint.
//
// If Prometheus can't be queried (unreachable, erroring, or an invalid
// response), the handler responds 503 with a generic "unavailable" message
// rather than surfacing the underlying error, so the Web UI can render a
// degraded state instead of failing hard — resolving ticket 06's open
// question on this behavior.
func usageHandler(querier metricsquery.MetricsQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), "management_api.get_usage")
		defer span.End()

		keyID := r.PathValue("keyID")

		usageRange, ok := parseUsageRange(r)
		if !ok {
			failManagement(ctx, w, http.StatusBadRequest, "invalid start, end, or step", "get_usage", "invalid_request")
			return
		}

		points, err := querier.QueryRange(ctx, keyID, usageRange)
		if err != nil {
			failManagement(ctx, w, http.StatusServiceUnavailable, "usage data is temporarily unavailable", "get_usage", "prometheus_unreachable")
			return
		}

		resp := make([]usagePointResponse, 0, len(points))
		for _, p := range points {
			resp = append(resp, usagePointResponse{Timestamp: p.Time, Value: p.Value})
		}

		recordManagementRequest(ctx, "get_usage", "success", "")
		writeJSON(w, http.StatusOK, resp)
	}
}

// parseUsageRange reads start/end/step from the request's query parameters,
// defaulting to the last hour at one-minute resolution, and reports ok=false
// if any parameter is malformed, end doesn't come after start, or the
// resulting number of samples would exceed maxUsagePoints.
func parseUsageRange(r *http.Request) (usageRange metricsquery.TimeRange, ok bool) {
	q := r.URL.Query()

	end := time.Now().UTC()
	start := end.Add(-defaultUsageRange)
	step := defaultUsageStep

	if v := q.Get("start"); v != "" {
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return metricsquery.TimeRange{}, false
		}
		start = parsed
	}
	if v := q.Get("end"); v != "" {
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return metricsquery.TimeRange{}, false
		}
		end = parsed
	}
	if v := q.Get("step"); v != "" {
		parsed, err := time.ParseDuration(v)
		if err != nil || parsed <= 0 {
			return metricsquery.TimeRange{}, false
		}
		step = parsed
	}

	if !end.After(start) {
		return metricsquery.TimeRange{}, false
	}
	if end.Sub(start)/step > maxUsagePoints {
		return metricsquery.TimeRange{}, false
	}

	return metricsquery.TimeRange{Start: start, End: end, Step: step}, true
}
