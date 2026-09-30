package main

import (
	"fmt"
	"io"
)

// checkDecisionsMetric and managementRequestsMetric are the Prometheus
// counter names the OpenTelemetry Prometheus exporter derives from
// check_service.decisions and management_api.requests (dots become
// underscores, plus a "_total" counter suffix) — see
// internal/httpapi/telemetry.go.
const (
	checkDecisionsMetric     = "check_service_decisions_total"
	managementRequestsMetric = "management_api_requests_total"
)

// checkOutcome pairs a check-mix scenario with the check_service.decisions
// label set a correctly-behaving service records for it. See
// internal/httpapi/checkauthz.go and internal/keys/validate.go for the
// result/reason strings.
type checkOutcome struct {
	scenario Scenario
	labels   map[string]string
}

var checkOutcomes = []checkOutcome{
	{ScenarioValid, map[string]string{"result": "allowed"}},
	{ScenarioUnknown, map[string]string{"result": "denied", "reason": "invalid key"}},
	{ScenarioExpired, map[string]string{"result": "denied", "reason": "key has expired"}},
	{ScenarioMissing, map[string]string{"result": "denied", "reason": "missing key"}},
}

// managementOutcome pairs a management-stream op (as used in managementOps)
// with the management_api.requests label set a correctly-behaving service
// records for a successful call. See internal/httpapi/keys.go.
type managementOutcome struct {
	op     string
	labels map[string]string
}

var managementOutcomes = []managementOutcome{
	{"list", map[string]string{"operation": "list_keys", "result": "success"}},
	{"create", map[string]string{"operation": "create_key", "result": "success"}},
	{"revoke", map[string]string{"operation": "revoke_key", "result": "success"}},
}

// MetricComparison is one row of the final summary: an outcome's label, the
// tool's own client-side count of requests it sent for it, and what the
// scraped Prometheus counter actually recorded for the matching labels.
type MetricComparison struct {
	Label      string
	ClientSent int64
	Prometheus float64
}

// SumCheckOutcomes sums checkDecisionSamples once per check-mix scenario,
// keyed by scenario, using each scenario's Prometheus label set (see
// checkOutcomes). It underlies both BuildCheckComparison and baselining a
// scrape taken before a run starts.
func SumCheckOutcomes(checkDecisionSamples []PromSample) map[Scenario]float64 {
	sums := make(map[Scenario]float64, len(checkOutcomes))
	for _, outcome := range checkOutcomes {
		sums[outcome.scenario] = SumByLabels(checkDecisionSamples, outcome.labels)
	}
	return sums
}

// SumManagementOutcomes sums managementRequestSamples once per management
// operation, keyed by op, using each op's Prometheus label set (see
// managementOutcomes). It underlies both BuildManagementComparison and
// baselining a scrape taken before a run starts.
func SumManagementOutcomes(managementRequestSamples []PromSample) map[string]float64 {
	sums := make(map[string]float64, len(managementOutcomes))
	for _, outcome := range managementOutcomes {
		sums[outcome.op] = SumByLabels(managementRequestSamples, outcome.labels)
	}
	return sums
}

// BuildCheckComparison compares the check stream's client-side tallies
// against the scraped check_service_decisions_total samples, one row per
// check-mix scenario. Prometheus's side of each row is the scenario's
// current counter sum minus its value in baseline (from SumCheckOutcomes on
// a scrape taken before the run started) — a nil or zero-valued baseline
// falls back to the counter's full cumulative value, which includes any
// traffic from before this run and so is a much noisier comparison. A
// scenario's client count only reflects requests that got a response
// (Stats.RecordCheckSent) — not ones that never reached the service — since
// only those could have been recorded by Prometheus at all. A mismatch can
// still legitimately occur (e.g. a "valid" request denied because the
// management stream revoked that exact key first), not just from a bug.
func BuildCheckComparison(snapshot StatsSnapshot, checkDecisionSamples []PromSample, baseline map[Scenario]float64) []MetricComparison {
	current := SumCheckOutcomes(checkDecisionSamples)

	rows := make([]MetricComparison, 0, len(checkOutcomes))
	for _, outcome := range checkOutcomes {
		rows = append(rows, MetricComparison{
			Label:      outcome.scenario.String(),
			ClientSent: snapshot.CheckSent[outcome.scenario],
			Prometheus: current[outcome.scenario] - baseline[outcome.scenario],
		})
	}
	return rows
}

// BuildManagementComparison compares the management stream's client-side
// tallies against the scraped management_api_requests_total samples, one row
// per management operation, baselined the same way as BuildCheckComparison.
func BuildManagementComparison(snapshot StatsSnapshot, managementRequestSamples []PromSample, baseline map[string]float64) []MetricComparison {
	current := SumManagementOutcomes(managementRequestSamples)

	rows := make([]MetricComparison, 0, len(managementOutcomes))
	for _, outcome := range managementOutcomes {
		rows = append(rows, MetricComparison{
			Label:      outcome.op,
			ClientSent: snapshot.MgmtSent[outcome.op],
			Prometheus: current[outcome.op] - baseline[outcome.op],
		})
	}
	return rows
}

// printComparisons writes title followed by one line per row, flagging any
// row where the client's and Prometheus's counts disagree so a mismatch is
// visible at a glance.
func printComparisons(w io.Writer, title string, rows []MetricComparison) {
	fmt.Fprintf(w, "\n%s\n", title)
	for _, row := range rows {
		flag := ""
		if float64(row.ClientSent) != row.Prometheus {
			flag = "  <-- mismatch"
		}
		fmt.Fprintf(w, "  %-10s sent=%-8d prometheus=%-8.0f%s\n", row.Label, row.ClientSent, row.Prometheus, flag)
	}
}
