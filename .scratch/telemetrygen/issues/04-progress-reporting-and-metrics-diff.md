# 04: Progress reporting and final metrics diff

**What to build:** Periodic progress output while the tool runs, plus a final summary on exit that scrapes the service's `/metrics` endpoint and shows what Prometheus actually recorded alongside what the tool believes it sent.

**Blocked by:** 02, 03

**Status:** ready-for-agent

- [ ] While running, the tool periodically prints progress: counts of requests sent broken down by outcome (per check-mix scenario) and by management operation, plus any failure counts.
- [ ] Per-request failures during the steady-state run are counted and logged, not fatal — the tool keeps running.
- [ ] On exit (whether via interrupt, `--duration` elapsing, or otherwise), the tool scrapes the service's `/metrics` endpoint.
- [ ] A pure, unit-tested parser extracts `check_service.decisions` and `management_api.requests` values from the scraped Prometheus exposition text, broken out by their labels.
- [ ] The final summary prints the parsed Prometheus values alongside the tool's own client-side tallies for the same metrics, so a mismatch between sent and recorded is visible at a glance.
- [ ] Unit tests cover the parser/diff logic against sample exposition text, independent of any running service.
