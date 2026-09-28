# 0005. Database migrations and deployment

Date: 2026-09-28

Status: Accepted

## Context

The service must ship as a single Go binary with no separate migration-tooling step, and needs a PostgreSQL database that should be easy to stand up for a PoC without requiring a pre-existing external instance.

## Decision

- Schema migrations are SQL files embedded via `go:embed` and run automatically on startup using the `golang-migrate` library (not its CLI).
- Database credentials are supplied via a `DATABASE_URL` environment variable, sourced from a Kubernetes `Secret`.
- The Helm chart declares PostgreSQL as an optional dependency (e.g. the Bitnami `postgresql` subchart), gated by `postgresql.enabled`, **disabled by default**. An external database remains the default expectation; setting `postgresql.enabled: true` deploys a bundled instance for demos/local use.

## Consequences

- `helm install` works out of the box against an external DB, matching README.md's stated default.
- A one-flag opt-in (`postgresql.enabled: true`) covers demo/local convenience without making a bundled, non-persistent-by-default database the norm.
