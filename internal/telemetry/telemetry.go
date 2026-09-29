// Package telemetry wires up the service's OpenTelemetry metrics export in
// Prometheus exposition format (ADR-0004).
package telemetry

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// Metrics serves the process's metrics in Prometheus exposition format.
type Metrics struct {
	// Handler serves the currently registered metrics in Prometheus
	// exposition format.
	Handler http.Handler
}

var (
	setupMu sync.Mutex
	metrics *Metrics
)

// Setup installs a Prometheus exporter as the global OpenTelemetry
// MeterProvider, so every otel.Meter obtained package-wide (e.g. the check
// and management API handlers' counters) exports through it, and returns the
// /metrics handler for it.
//
// The global OpenTelemetry MeterProvider can only ever be meaningfully
// installed once per process — instruments obtained via otel.Meter() before
// a later call permanently keep delegating to whichever provider was set
// first, so a second call would silently strand their data. Setup is
// therefore idempotent once it succeeds: the real pipeline is built on the
// first successful call, and every call after that (as happens when a test
// builds more than one App in the same process) returns that same instance.
// A failed attempt is never cached, so a transient error on an early call
// doesn't permanently break every later one.
func Setup() (*Metrics, error) {
	setupMu.Lock()
	defer setupMu.Unlock()

	if metrics != nil {
		return metrics, nil
	}

	registry := prometheus.NewRegistry()

	exporter, err := otelprom.New(otelprom.WithRegisterer(registry))
	if err != nil {
		return nil, fmt.Errorf("creating prometheus exporter: %w", err)
	}

	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	otel.SetMeterProvider(provider)

	metrics = &Metrics{Handler: promhttp.HandlerFor(registry, promhttp.HandlerOpts{})}
	return metrics, nil
}
