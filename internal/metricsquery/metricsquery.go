// Package metricsquery wraps querying Prometheus for a key's usage over
// time (ADR-0004). It exists as an interface — unlike the Postgres store,
// which has no equivalent interface — specifically because Prometheus is an
// external system outside this service's control and isn't practical to
// include in the primary HTTP-seam test environment (ticket 06); this is not
// a general "add an interface per dependency" pattern.
package metricsquery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Point is a single sample: a value at a point in time.
type Point struct {
	Time  time.Time
	Value float64
}

// TimeRange is a query window: samples are taken every Step across
// [Start, End].
type TimeRange struct {
	Start time.Time
	End   time.Time
	Step  time.Duration
}

// MetricsQuerier reports a key's usage rate, sampled at regular intervals
// across a time range.
type MetricsQuerier interface {
	// QueryRange returns keyID's request rate, one point per r.Step, across
	// [r.Start, r.End].
	QueryRange(ctx context.Context, keyID string, r TimeRange) ([]Point, error)
}

// PrometheusQuerier is the concrete MetricsQuerier backed by Prometheus's
// HTTP query API, querying the check_service_decisions_total counter that
// ticket 05 introduced.
type PrometheusQuerier struct {
	baseURL string
	client  *http.Client
}

// NewPrometheusQuerier builds a PrometheusQuerier against the Prometheus
// server at baseURL (e.g. "http://prometheus:9090").
func NewPrometheusQuerier(baseURL string) *PrometheusQuerier {
	return &PrometheusQuerier{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// promRangeResponse is the subset of Prometheus's query_range response this
// package needs.
// See https://prometheus.io/docs/prometheus/latest/querying/api/#range-queries.
type promRangeResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Data   struct {
		Result []struct {
			Values [][2]any `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

// QueryRange reports keyID's total per-second decision rate — allowed and
// denied alike, since a denial is still a usage attempt — sampled every step
// across [start, end]. A key with no matching traffic in range returns an
// empty, non-error result; an error return specifically means Prometheus
// itself could not be queried (unreachable, erroring, or an invalid
// response) — the distinction the Web UI backend needs to tell "no usage
// yet" apart from "usage data unavailable".
func (q *PrometheusQuerier) QueryRange(ctx context.Context, keyID string, r TimeRange) ([]Point, error) {
	query := fmt.Sprintf(`sum(rate(check_service_decisions_total{key_id=%q}[%s]))`, keyID, formatPromDuration(r.Step))

	values := url.Values{
		"query": {query},
		"start": {r.Start.UTC().Format(time.RFC3339)},
		"end":   {r.End.UTC().Format(time.RFC3339)},
		"step":  {formatPromDuration(r.Step)},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, q.baseURL+"/api/v1/query_range?"+values.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("building prometheus query: %w", err)
	}

	resp, err := q.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("querying prometheus: %w", err)
	}
	defer resp.Body.Close()

	var parsed promRangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decoding prometheus response (status %d): %w", resp.StatusCode, err)
	}
	if parsed.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed: %s", parsed.Error)
	}

	var points []Point
	for _, series := range parsed.Data.Result {
		for _, sample := range series.Values {
			point, err := parseSample(sample)
			if err != nil {
				return nil, err
			}
			points = append(points, point)
		}
	}
	return points, nil
}

// parseSample converts a single Prometheus [timestamp, "value"] pair — the
// heterogeneous shape Prometheus's JSON API uses for a sample — into a
// Point.
func parseSample(sample [2]any) (Point, error) {
	ts, ok := sample[0].(float64)
	if !ok {
		return Point{}, fmt.Errorf("unexpected prometheus sample timestamp type %T", sample[0])
	}
	raw, ok := sample[1].(string)
	if !ok {
		return Point{}, fmt.Errorf("unexpected prometheus sample value type %T", sample[1])
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return Point{}, fmt.Errorf("parsing prometheus sample value: %w", err)
	}
	return Point{Time: time.Unix(int64(ts), 0).UTC(), Value: value}, nil
}

// formatPromDuration renders d the way Prometheus's query API expects (e.g.
// "90s"), avoiding Go's "1m30s" formatting that Prometheus doesn't parse.
func formatPromDuration(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds(), 'f', -1, 64) + "s"
}
