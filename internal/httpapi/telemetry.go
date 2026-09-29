package httpapi

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// instrumentationName identifies this package's tracer and meter. Global
// providers are no-ops until an exporter is registered (ticket 05 wires up
// the Prometheus exporter); emitting against them now means the check
// endpoint's instrumentation needs no changes once that exporter lands.
const instrumentationName = "github.com/cjgreaves97/istio-api-key-auth-poc/internal/httpapi"

var checkTracer trace.Tracer = otel.Tracer(instrumentationName)

var checkDecisions metric.Int64Counter = mustInt64Counter(
	"check_service.decisions",
	"Number of ext_authz check decisions made by the check endpoint, by result.",
)

func mustInt64Counter(name, description string) metric.Int64Counter {
	counter, err := otel.Meter(instrumentationName).Int64Counter(name, metric.WithDescription(description))
	if err != nil {
		// Only fails on invalid instrument configuration, which is a
		// programmer error caught immediately at startup.
		panic(err)
	}
	return counter
}
