package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// progressInterval is how often the tool logs a snapshot of its client-side
// tallies while running.
const progressInterval = 10 * time.Second

// runProgressReporter logs a progress snapshot every progressInterval until
// ctx is cancelled.
func runProgressReporter(ctx context.Context, logger *slog.Logger, stats *Stats) {
	ticker := time.NewTicker(progressInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			logProgress(logger, stats.Snapshot())
		}
	}
}

// logProgress logs snap's per-scenario and per-operation sent/failed counts
// as a single progress line.
func logProgress(logger *slog.Logger, snap StatsSnapshot) {
	args := []any{}
	for _, s := range []Scenario{ScenarioValid, ScenarioUnknown, ScenarioExpired, ScenarioMissing} {
		args = append(args,
			"check_"+s.String()+"_sent", snap.CheckSent[s],
			"check_"+s.String()+"_failed", snap.CheckFailed[s],
		)
	}
	for _, op := range managementOps {
		args = append(args,
			"management_"+op+"_sent", snap.MgmtSent[op],
			"management_"+op+"_failed", snap.MgmtFailed[op],
		)
	}
	logger.Info("progress", args...)
}

// scrapeMetricsBaseline scrapes /metrics once, before the run's traffic
// streams start, and sums each check-mix scenario's and management
// operation's existing counter value. The final summary subtracts this from
// its own end-of-run scrape, so it reports only what changed during this
// run rather than the server's entire history — which, since these are
// process-lifetime counters, may include traffic from long before this run
// started. A scrape or parse failure here is non-fatal: it's logged, and a
// nil baseline (BuildCheckComparison/BuildManagementComparison treat a
// missing entry as zero) falls back to comparing against the server's full
// cumulative totals instead of a delta.
func scrapeMetricsBaseline(ctx context.Context, logger *slog.Logger, client *apiClient) (checkBaseline map[Scenario]float64, managementBaseline map[string]float64) {
	body, err := client.scrapeMetrics(ctx)
	if err != nil {
		logger.Warn("failed to scrape /metrics for a baseline; final summary will compare against cumulative totals", "error", err)
		return nil, nil
	}

	checkSamples, err := ParsePrometheusMetric(body, checkDecisionsMetric)
	if err != nil {
		logger.Warn("failed to parse "+checkDecisionsMetric+" for baseline", "error", err)
	}
	managementSamples, err := ParsePrometheusMetric(body, managementRequestsMetric)
	if err != nil {
		logger.Warn("failed to parse "+managementRequestsMetric+" for baseline", "error", err)
	}

	return SumCheckOutcomes(checkSamples), SumManagementOutcomes(managementSamples)
}

// runFinalSummary scrapes the service's /metrics endpoint and prints, for
// each check-mix scenario and management operation, the tool's own
// client-side tally alongside what Prometheus recorded during this run
// (current counter value minus checkBaseline/managementBaseline) — so a
// mismatch between sent and recorded is visible at a glance. It's best
// effort: a scrape or parse failure is logged and the comparison is printed
// with zero Prometheus counts rather than aborting, since the summary is
// diagnostic, not a precondition for the run having done its job.
func runFinalSummary(ctx context.Context, logger *slog.Logger, client *apiClient, stats *Stats, checkBaseline map[Scenario]float64, managementBaseline map[string]float64) {
	snapshot := stats.Snapshot()

	var checkSamples, managementSamples []PromSample
	body, err := client.scrapeMetrics(ctx)
	if err != nil {
		logger.Warn("failed to scrape /metrics for final summary", "error", err)
	} else {
		checkSamples, err = ParsePrometheusMetric(body, checkDecisionsMetric)
		if err != nil {
			logger.Warn("failed to parse "+checkDecisionsMetric, "error", err)
		}
		managementSamples, err = ParsePrometheusMetric(body, managementRequestsMetric)
		if err != nil {
			logger.Warn("failed to parse "+managementRequestsMetric, "error", err)
		}
	}

	fmt.Fprintln(os.Stdout, "\n=== Final summary: client-sent vs. Prometheus-recorded (this run) ===")
	printComparisons(os.Stdout, "check_service.decisions (by scenario)", BuildCheckComparison(snapshot, checkSamples, checkBaseline))
	printComparisons(os.Stdout, "management_api.requests (by operation)", BuildManagementComparison(snapshot, managementSamples, managementBaseline))
}
