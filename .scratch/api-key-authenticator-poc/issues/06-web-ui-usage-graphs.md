# 06: Web UI usage graphs

**What to build:** An operator viewing a key in the Web UI can see a usage graph for that key, sourced live from Prometheus (ADR-0004) rather than from a duplicated counter in Postgres.

**Blocked by:** 05

**Status:** ready-for-agent

- [ ] A `MetricsQuerier` interface wraps querying Prometheus (e.g. a range query) for a given Key ID and time range; the concrete implementation calls Prometheus's HTTP query API using the counter introduced in ticket 05.
- [ ] The Web UI backend exposes an endpoint that, given a Key ID and time range, returns a time series sourced via `MetricsQuerier` — Postgres is not involved in this path.
- [ ] The Web UI's key detail view renders this time series as a usage graph.
- [ ] Tests for this endpoint stub `MetricsQuerier` with an in-memory fake rather than standing up a real Prometheus instance.
- [ ] Behavior when Prometheus is unreachable is decided and covered by a test (see the open question in the spec's Further Notes — resolve it here rather than leaving it implicit).
