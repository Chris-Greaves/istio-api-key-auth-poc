package metricsquery_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Chris-Greaves/istio-api-key-auth-poc/internal/metricsquery"
)

func TestPrometheusQuerier_QueryRangeParsesAMatrixResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query_range" {
			t.Fatalf("expected request to /api/v1/query_range, got %s", r.URL.Path)
		}
		if !strings.Contains(r.URL.Query().Get("query"), `key_id="abcd1234"`) {
			t.Fatalf("expected query to reference the key id, got %q", r.URL.Query().Get("query"))
		}
		fmt.Fprint(w, `{
			"status": "success",
			"data": {
				"resultType": "matrix",
				"result": [
					{"metric": {}, "values": [[1000, "0.5"], [1060, "1.5"]]}
				]
			}
		}`)
	}))
	defer server.Close()

	querier := metricsquery.NewPrometheusQuerier(server.URL)

	r := metricsquery.TimeRange{Start: time.Unix(1000, 0), End: time.Unix(1060, 0), Step: time.Minute}
	points, err := querier.QueryRange(context.Background(), "abcd1234", r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(points) != 2 {
		t.Fatalf("expected 2 points, got %d: %+v", len(points), points)
	}
	if !points[0].Time.Equal(time.Unix(1000, 0)) || points[0].Value != 0.5 {
		t.Fatalf("unexpected first point: %+v", points[0])
	}
	if !points[1].Time.Equal(time.Unix(1060, 0)) || points[1].Value != 1.5 {
		t.Fatalf("unexpected second point: %+v", points[1])
	}
}

func TestPrometheusQuerier_QueryRangeReturnsEmptyForNoMatchingSeries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"status": "success", "data": {"resultType": "matrix", "result": []}}`)
	}))
	defer server.Close()

	querier := metricsquery.NewPrometheusQuerier(server.URL)

	points, err := querier.QueryRange(context.Background(), "unused123", metricsquery.TimeRange{Start: time.Unix(0, 0), End: time.Unix(60, 0), Step: time.Minute})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(points) != 0 {
		t.Fatalf("expected no points for a key with no traffic, got %+v", points)
	}
}

func TestPrometheusQuerier_QueryRangeReturnsAnErrorOnAPrometheusErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"status": "error", "errorType": "bad_data", "error": "invalid query"}`)
	}))
	defer server.Close()

	querier := metricsquery.NewPrometheusQuerier(server.URL)

	_, err := querier.QueryRange(context.Background(), "abcd1234", metricsquery.TimeRange{Start: time.Unix(0, 0), End: time.Unix(60, 0), Step: time.Minute})
	if err == nil {
		t.Fatal("expected an error for a Prometheus error-status response")
	}
}

func TestPrometheusQuerier_QueryRangeReturnsAnErrorWhenPrometheusIsUnreachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unreachableURL := server.URL
	server.Close() // closed before use, so nothing is listening on this address

	querier := metricsquery.NewPrometheusQuerier(unreachableURL)

	_, err := querier.QueryRange(context.Background(), "abcd1234", metricsquery.TimeRange{Start: time.Unix(0, 0), End: time.Unix(60, 0), Step: time.Minute})
	if err == nil {
		t.Fatal("expected an error when Prometheus is unreachable")
	}
}
