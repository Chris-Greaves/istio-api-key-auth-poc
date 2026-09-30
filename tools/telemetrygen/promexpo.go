package main

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// PromSample is a single Prometheus exposition-format sample: a metric's
// label set and the value recorded for it.
type PromSample struct {
	Labels map[string]string
	Value  float64
}

// ParsePrometheusMetric extracts every sample for metricName out of
// Prometheus exposition-format text (as served by the service's /metrics
// endpoint), ignoring comment/HELP/TYPE lines and every other metric family.
// It's a pure function of text, independent of any running service, so the
// final summary's diff logic is unit-testable against fixed sample text.
func ParsePrometheusMetric(text, metricName string) ([]PromSample, error) {
	var samples []PromSample

	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || !strings.HasPrefix(line, metricName) {
			continue
		}

		rest := line[len(metricName):]
		if rest == "" || (rest[0] != '{' && rest[0] != ' ') {
			// A different metric that happens to share this prefix, e.g.
			// metricName="check_service_decisions_total" must not match a
			// line for "check_service_decisions_total_created".
			continue
		}

		sample, err := parsePromSampleLine(rest)
		if err != nil {
			return nil, fmt.Errorf("parsing sample line %q: %w", line, err)
		}
		samples = append(samples, sample)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning prometheus exposition text: %w", err)
	}

	return samples, nil
}

// parsePromSampleLine parses the portion of an exposition-format line after
// its metric name: an optional {label="value",...} set, followed by the
// sample's value and an optional trailing timestamp (ignored).
func parsePromSampleLine(rest string) (PromSample, error) {
	rest = strings.TrimSpace(rest)

	labels := map[string]string{}
	if strings.HasPrefix(rest, "{") {
		end := strings.Index(rest, "}")
		if end == -1 {
			return PromSample{}, fmt.Errorf("unterminated label set")
		}

		for _, kv := range splitPromLabels(rest[1:end]) {
			key, value, err := parsePromLabel(kv)
			if err != nil {
				return PromSample{}, err
			}
			labels[key] = value
		}
		rest = strings.TrimSpace(rest[end+1:])
	}

	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return PromSample{}, fmt.Errorf("missing value")
	}
	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return PromSample{}, fmt.Errorf("parsing value %q: %w", fields[0], err)
	}

	return PromSample{Labels: labels, Value: value}, nil
}

// splitPromLabels splits a label-set's interior (the text between { and })
// on top-level commas, respecting quoted label values so a comma inside one
// (which Prometheus label values, unlike ours, are technically allowed to
// contain) doesn't split it in two.
func splitPromLabels(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}

	var parts []string
	var current strings.Builder
	inQuotes := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQuotes = !inQuotes
			current.WriteByte(c)
		case c == ',' && !inQuotes:
			parts = append(parts, current.String())
			current.Reset()
		default:
			current.WriteByte(c)
		}
	}
	parts = append(parts, current.String())
	return parts
}

// parsePromLabel parses a single label="value" pair, unescaping the value's
// surrounding quotes and \" / \\ escapes.
func parsePromLabel(kv string) (key, value string, err error) {
	rawKey, rawValue, ok := strings.Cut(kv, "=")
	if !ok {
		return "", "", fmt.Errorf("malformed label %q", kv)
	}

	key = strings.TrimSpace(rawKey)
	value = strings.TrimSpace(rawValue)
	value = strings.TrimPrefix(value, `"`)
	value = strings.TrimSuffix(value, `"`)
	value = strings.ReplaceAll(value, `\"`, `"`)
	value = strings.ReplaceAll(value, `\\`, `\`)
	return key, value, nil
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
