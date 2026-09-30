package main

import (
	"fmt"
	"strings"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

// PromSample is a single Prometheus exposition-format sample: a metric's
// label set and the value recorded for it.
type PromSample struct {
	Labels map[string]string
	Value  float64
}

// ParsePrometheusMetric extracts every sample for metricName out of
// Prometheus exposition-format text (as served by the service's /metrics
// endpoint), ignoring every other metric family. It's a pure function of
// text, independent of any running service, so the final summary's diff
// logic is unit-testable against fixed sample text. Parsing is delegated to
// prometheus/common/expfmt's TextParser rather than a hand-rolled parser, so
// it correctly handles exposition-format edge cases (e.g. an escaped quote
// followed by a comma inside a label value) that a naive comma/quote scan
// gets wrong.
func ParsePrometheusMetric(text, metricName string) ([]PromSample, error) {
	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(text))
	if err != nil {
		return nil, fmt.Errorf("parsing prometheus exposition text: %w", err)
	}

	family, ok := families[metricName]
	if !ok {
		return nil, nil
	}

	samples := make([]PromSample, 0, len(family.GetMetric()))
	for _, m := range family.GetMetric() {
		labels := make(map[string]string, len(m.GetLabel()))
		for _, lp := range m.GetLabel() {
			labels[lp.GetName()] = lp.GetValue()
		}
		samples = append(samples, PromSample{Labels: labels, Value: metricValue(m)})
	}
	return samples, nil
}

// metricValue extracts a parsed Metric's numeric value regardless of its
// declared type. Every metric this tool reads is a counter, but falling back
// to gauge/untyped costs nothing and avoids a silent zero if a family's
// # TYPE line is ever missing or wrong.
func metricValue(m *dto.Metric) float64 {
	switch {
	case m.Counter != nil:
		return m.GetCounter().GetValue()
	case m.Gauge != nil:
		return m.GetGauge().GetValue()
	case m.Untyped != nil:
		return m.GetUntyped().GetValue()
	default:
		return 0
	}
}

// SumByLabels sums the Value of every sample in samples whose labels match
// every key/value pair in filter exactly. Labels a sample has that filter
// doesn't mention (e.g. key_id, which differs per key) are ignored, so this
// sums a counter's value across every key sharing the same result/reason/
// operation.
func SumByLabels(samples []PromSample, filter map[string]string) float64 {
	var total float64
	for _, sample := range samples {
		if labelsMatch(sample.Labels, filter) {
			total += sample.Value
		}
	}
	return total
}

func labelsMatch(labels, filter map[string]string) bool {
	for key, value := range filter {
		if labels[key] != value {
			return false
		}
	}
	return true
}
