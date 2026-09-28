# 0004. Metrics and observability approach

Date: 2026-09-28

Status: Accepted

## Context

The service needs both an operator-facing Prometheus surface and an end-user "view metrics" feature in the Web UI.

## Decision

- Prometheus metrics are labeled per individual API Key ID. Acceptable cardinality at PoC scale (not expected to handle large numbers of keys).
- The Web UI queries Prometheus directly for per-key usage graphs, rather than duplicating usage counters in Postgres. Postgres stays metadata-only (id, hash, owner, timestamps, `expires_at`).
- The service is instrumented with OpenTelemetry for both metrics and traces. Logs remain plain structured logging for the PoC.

## Consequences

- The Web UI backend depends on Prometheus being reachable to serve metrics views.
- If key volume grows significantly beyond PoC scale, the per-key label cardinality decision will need revisiting.
