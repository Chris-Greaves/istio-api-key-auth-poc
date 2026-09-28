# 05: Prometheus metrics export

**What to build:** The service exposes a `/metrics` endpoint in Prometheus exposition format, with request counters labeled per Key ID (ADR-0004), populated by real traffic through the check service built in ticket 04.

**Blocked by:** 04

**Status:** ready-for-agent

- [ ] `/metrics` returns Prometheus exposition format.
- [ ] Check-service requests increment a counter labeled by Key ID (and result, e.g. allowed/denied) so per-key usage is queryable in Prometheus.
- [ ] The management API's create/list/delete endpoints (tickets 02–03) are also instrumented with OpenTelemetry metrics/traces, consistent with the check endpoint's instrumentation from ticket 04.
- [ ] An HTTP-level test against the wired binary drives check-endpoint traffic for a known key, then scrapes `/metrics` and asserts the expected counter/label is present with the expected value.
