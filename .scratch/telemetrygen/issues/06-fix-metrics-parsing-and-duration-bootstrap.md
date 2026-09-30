# 06: Fix Prometheus label parsing, duplicate parser, and --duration/bootstrap interaction

**What to build:** Address three issues surfaced by code review of tickets 01–04: a label-parsing bug in the hand-rolled Prometheus exposition parser, its duplication of an already-vendored parser, and `--duration`'s timeout starting before bootstrap completes.

**Blocked by:** 04

**Status:** ready-for-agent

- [ ] `ParsePrometheusMetric`/`splitPromLabels` (`promexpo.go`) is replaced with `github.com/prometheus/common/expfmt`'s `TextParser.TextToMetricFamilies` (already a transitive dependency via `client_golang` and the OTel Prometheus exporter), removing the hand-rolled parser.
- [ ] The replacement correctly handles a label value containing a backslash-escaped quote followed by a comma (e.g. `reason="a\" complicated, value"`) without mis-splitting the label set — add a unit test covering this case, since it's what exposed the original bug.
- [ ] `SumByLabels`/`SumCheckOutcomes`/`SumManagementOutcomes` and their callers are updated to consume whatever sample type the new parser produces, preserving existing behavior for all current unit tests in `promexpo_test.go` and `summary_test.go`.
- [ ] In `main.go`, the `--duration` timeout is applied so it bounds only the traffic-generation window (the check/management streams' run time), not `bootstrap()` or the pre-run `scrapeMetricsBaseline()` call — i.e. a run with a large `--pool-size` and a short `--duration` still generates traffic for the full requested duration after bootstrap completes.
- [ ] If bootstrap's own duration should still be bounded (e.g. against a hung service), that's a separate, clearly-distinct timeout from `--duration`'s traffic-generation window — not the same context deadline.
- [ ] Unit tests cover the corrected duration/bootstrap sequencing behavior where feasible without a live service (e.g. by asserting the traffic-window context isn't already expired when the streams start).
