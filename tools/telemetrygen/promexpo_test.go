package main

import "testing"

const samplePrometheusText = `# HELP check_service_decisions_total Number of ext_authz check decisions made by the check endpoint, by result.
# TYPE check_service_decisions_total counter
check_service_decisions_total{key_id="aaaaaaaa",result="allowed"} 42
check_service_decisions_total{key_id="bbbbbbbb",result="allowed"} 8
check_service_decisions_total{reason="invalid key",result="denied"} 15
check_service_decisions_total{key_id="cccccccc",reason="key has expired",result="denied"} 3
check_service_decisions_total{reason="missing key",result="denied"} 7
# HELP management_api_requests_total Number of management API requests handled, by operation and result.
# TYPE management_api_requests_total counter
management_api_requests_total{operation="list_keys",result="success"} 12
management_api_requests_total{key_id="dddddddd",operation="create_key",result="success"} 5
management_api_requests_total{key_id="eeeeeeee",operation="revoke_key",result="success"} 2
management_api_requests_total{operation="create_key",result="invalid_request"} 1
# HELP some_other_metric An unrelated metric that happens to share a prefix.
# TYPE some_other_metric_total counter
check_service_decisions_totally_unrelated{result="allowed"} 999
`

func TestParsePrometheusMetric_ExtractsOnlyMatchingMetricFamily(t *testing.T) {
	samples, err := ParsePrometheusMetric(samplePrometheusText, "check_service_decisions_total")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(samples) != 5 {
		t.Fatalf("expected 5 samples, got %d: %+v", len(samples), samples)
	}
}

func TestParsePrometheusMetric_IgnoresMetricsSharingAPrefix(t *testing.T) {
	samples, err := ParsePrometheusMetric(samplePrometheusText, "check_service_decisions_total")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, s := range samples {
		if s.Value == 999 {
			t.Fatal("expected the differently-suffixed metric to be excluded")
		}
	}
}

func TestParsePrometheusMetric_ParsesLabelsAndValue(t *testing.T) {
	samples, err := ParsePrometheusMetric(samplePrometheusText, "check_service_decisions_total")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var found bool
	for _, s := range samples {
		if s.Labels["key_id"] == "cccccccc" {
			found = true
			if s.Labels["reason"] != "key has expired" {
				t.Errorf("expected reason %q, got %q", "key has expired", s.Labels["reason"])
			}
			if s.Labels["result"] != "denied" {
				t.Errorf("expected result %q, got %q", "denied", s.Labels["result"])
			}
			if s.Value != 3 {
				t.Errorf("expected value 3, got %v", s.Value)
			}
		}
	}
	if !found {
		t.Fatal("expected to find the sample for key_id=cccccccc")
	}
}

func TestParsePrometheusMetric_HandlesNoMatches(t *testing.T) {
	samples, err := ParsePrometheusMetric(samplePrometheusText, "totally_absent_metric_total")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(samples) != 0 {
		t.Fatalf("expected no samples, got %+v", samples)
	}
}

func TestParsePrometheusMetric_ErrorsOnUnterminatedLabelSet(t *testing.T) {
	_, err := ParsePrometheusMetric(`my_metric{result="allowed" 1`, "my_metric")
	if err == nil {
		t.Fatal("expected an error for an unterminated label set")
	}
}

func TestParsePrometheusMetric_ErrorsOnMissingValue(t *testing.T) {
	_, err := ParsePrometheusMetric(`my_metric{result="allowed"}`, "my_metric")
	if err == nil {
		t.Fatal("expected an error for a missing value")
	}
}

func TestSumByLabels_SumsAcrossKeyIDsIgnoringUnfilteredLabels(t *testing.T) {
	samples, err := ParsePrometheusMetric(samplePrometheusText, "check_service_decisions_total")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := SumByLabels(samples, map[string]string{"result": "allowed"})
	if got != 50 { // 42 + 8, across two different key_id values
		t.Fatalf("expected 50, got %v", got)
	}
}

func TestSumByLabels_MatchesOnMultipleLabels(t *testing.T) {
	samples, err := ParsePrometheusMetric(samplePrometheusText, "check_service_decisions_total")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := SumByLabels(samples, map[string]string{"result": "denied", "reason": "key has expired"})
	if got != 3 {
		t.Fatalf("expected 3, got %v", got)
	}
}

func TestSumByLabels_ReturnsZeroWhenNothingMatches(t *testing.T) {
	samples, err := ParsePrometheusMetric(samplePrometheusText, "check_service_decisions_total")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := SumByLabels(samples, map[string]string{"result": "denied", "reason": "nope"})
	if got != 0 {
		t.Fatalf("expected 0, got %v", got)
	}
}

func TestSumByLabels_ReturnsZeroForEmptySampleSet(t *testing.T) {
	got := SumByLabels(nil, map[string]string{"result": "allowed"})
	if got != 0 {
		t.Fatalf("expected 0, got %v", got)
	}
}
