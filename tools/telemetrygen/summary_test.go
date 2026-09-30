package main

import "testing"

func comparisonFor(t *testing.T, rows []MetricComparison, label string) MetricComparison {
	t.Helper()
	for _, row := range rows {
		if row.Label == label {
			return row
		}
	}
	t.Fatalf("no comparison row for label %q in %+v", label, rows)
	return MetricComparison{}
}

func TestBuildCheckComparison_MatchesEachScenarioToItsPrometheusLabels(t *testing.T) {
	snapshot := StatsSnapshot{
		CheckSent: map[Scenario]int64{
			ScenarioValid:   50,
			ScenarioUnknown: 15,
			ScenarioExpired: 10,
			ScenarioMissing: 5,
		},
	}
	samples := []PromSample{
		{Labels: map[string]string{"key_id": "a", "result": "allowed"}, Value: 30},
		{Labels: map[string]string{"key_id": "b", "result": "allowed"}, Value: 20},
		{Labels: map[string]string{"result": "denied", "reason": "invalid key"}, Value: 15},
		{Labels: map[string]string{"key_id": "c", "result": "denied", "reason": "key has expired"}, Value: 10},
		{Labels: map[string]string{"result": "denied", "reason": "missing key"}, Value: 5},
	}

	rows := BuildCheckComparison(snapshot, samples, nil)

	valid := comparisonFor(t, rows, ScenarioValid.String())
	if valid.ClientSent != 50 || valid.Prometheus != 50 {
		t.Errorf("valid: got %+v, want sent=50 prometheus=50", valid)
	}

	unknown := comparisonFor(t, rows, ScenarioUnknown.String())
	if unknown.ClientSent != 15 || unknown.Prometheus != 15 {
		t.Errorf("unknown: got %+v, want sent=15 prometheus=15", unknown)
	}

	expired := comparisonFor(t, rows, ScenarioExpired.String())
	if expired.ClientSent != 10 || expired.Prometheus != 10 {
		t.Errorf("expired: got %+v, want sent=10 prometheus=10", expired)
	}

	missing := comparisonFor(t, rows, ScenarioMissing.String())
	if missing.ClientSent != 5 || missing.Prometheus != 5 {
		t.Errorf("missing: got %+v, want sent=5 prometheus=5", missing)
	}
}

func TestBuildCheckComparison_ReportsZeroPrometheusValueWhenNoSamplesMatch(t *testing.T) {
	snapshot := StatsSnapshot{CheckSent: map[Scenario]int64{ScenarioValid: 7}}

	rows := BuildCheckComparison(snapshot, nil, nil)

	valid := comparisonFor(t, rows, ScenarioValid.String())
	if valid.ClientSent != 7 || valid.Prometheus != 0 {
		t.Errorf("expected a mismatch (sent=7, prometheus=0), got %+v", valid)
	}
}

func TestBuildCheckComparison_SubtractsBaselineFromCurrentCounterValue(t *testing.T) {
	snapshot := StatsSnapshot{CheckSent: map[Scenario]int64{ScenarioValid: 30}}
	samples := []PromSample{
		{Labels: map[string]string{"key_id": "a", "result": "allowed"}, Value: 130}, // 100 pre-existing + 30 from this run
	}
	baseline := map[Scenario]float64{ScenarioValid: 100}

	rows := BuildCheckComparison(snapshot, samples, baseline)

	valid := comparisonFor(t, rows, ScenarioValid.String())
	if valid.ClientSent != 30 || valid.Prometheus != 30 {
		t.Errorf("expected the baseline's pre-existing traffic to be excluded, got %+v", valid)
	}
}

func TestBuildManagementComparison_MatchesEachOpToItsPrometheusLabels(t *testing.T) {
	snapshot := StatsSnapshot{
		MgmtSent: map[string]int64{
			"list":   12,
			"create": 6,
			"revoke": 2,
		},
	}
	samples := []PromSample{
		{Labels: map[string]string{"operation": "list_keys", "result": "success"}, Value: 12},
		{Labels: map[string]string{"key_id": "d", "operation": "create_key", "result": "success"}, Value: 6},
		{Labels: map[string]string{"key_id": "e", "operation": "revoke_key", "result": "success"}, Value: 2},
		{Labels: map[string]string{"operation": "create_key", "result": "invalid_request"}, Value: 3},
	}

	rows := BuildManagementComparison(snapshot, samples, nil)

	list := comparisonFor(t, rows, "list")
	if list.ClientSent != 12 || list.Prometheus != 12 {
		t.Errorf("list: got %+v, want sent=12 prometheus=12", list)
	}

	create := comparisonFor(t, rows, "create")
	if create.ClientSent != 6 || create.Prometheus != 6 {
		t.Errorf("create: got %+v, want sent=6 prometheus=6 (invalid_request samples excluded)", create)
	}

	revoke := comparisonFor(t, rows, "revoke")
	if revoke.ClientSent != 2 || revoke.Prometheus != 2 {
		t.Errorf("revoke: got %+v, want sent=2 prometheus=2", revoke)
	}
}

func TestBuildManagementComparison_SubtractsBaselineFromCurrentCounterValue(t *testing.T) {
	snapshot := StatsSnapshot{MgmtSent: map[string]int64{"create": 4}}
	samples := []PromSample{
		{Labels: map[string]string{"key_id": "d", "operation": "create_key", "result": "success"}, Value: 68}, // 64 pre-existing + 4 from this run
	}
	baseline := map[string]float64{"create": 64}

	rows := BuildManagementComparison(snapshot, samples, baseline)

	create := comparisonFor(t, rows, "create")
	if create.ClientSent != 4 || create.Prometheus != 4 {
		t.Errorf("expected the baseline's pre-existing traffic to be excluded, got %+v", create)
	}
}

func TestSumCheckOutcomes_SumsEachScenarioAcrossKeyIDs(t *testing.T) {
	samples := []PromSample{
		{Labels: map[string]string{"key_id": "a", "result": "allowed"}, Value: 10},
		{Labels: map[string]string{"key_id": "b", "result": "allowed"}, Value: 5},
		{Labels: map[string]string{"result": "denied", "reason": "missing key"}, Value: 3},
	}

	sums := SumCheckOutcomes(samples)

	if sums[ScenarioValid] != 15 {
		t.Errorf("expected 15, got %v", sums[ScenarioValid])
	}
	if sums[ScenarioMissing] != 3 {
		t.Errorf("expected 3, got %v", sums[ScenarioMissing])
	}
	if sums[ScenarioExpired] != 0 {
		t.Errorf("expected 0, got %v", sums[ScenarioExpired])
	}
}

func TestSumManagementOutcomes_SumsEachOpAcrossKeyIDs(t *testing.T) {
	samples := []PromSample{
		{Labels: map[string]string{"operation": "list_keys", "result": "success"}, Value: 7},
		{Labels: map[string]string{"key_id": "d", "operation": "create_key", "result": "success"}, Value: 2},
	}

	sums := SumManagementOutcomes(samples)

	if sums["list"] != 7 {
		t.Errorf("expected 7, got %v", sums["list"])
	}
	if sums["create"] != 2 {
		t.Errorf("expected 2, got %v", sums["create"])
	}
	if sums["revoke"] != 0 {
		t.Errorf("expected 0, got %v", sums["revoke"])
	}
}
