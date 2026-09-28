# Spec: API Key Authenticator PoC

Status: ready-for-agent

## Problem Statement

Teams running services inside an Istio mesh have no lightweight way to gate mesh traffic behind a simple, revocable credential. Building bespoke authentication into every service is wasteful, and standing up a full IAM/SSO system is disproportionate for internal or partner-to-service traffic that just needs a shared-secret style check. Operators also need a way to issue and revoke these credentials, see who a credential belongs to, and see how much it's being used — without hand-editing a database or reading raw Prometheus queries.

## Solution

A single Go binary, deployed into the mesh via Helm, that:

- Issues and revokes **API Keys** through a Web UI, storing key metadata in PostgreSQL.
- Acts as an Envoy `ext_authz` HTTP **Check Service** that Istio calls on every request, validating the `X-API-Key` header and injecting the key's **Owner** as a response header on success.
- Exposes a Prometheus `/metrics` endpoint labeled per **Key ID**, and surfaces per-key usage graphs in the Web UI by querying Prometheus directly.
- Runs its own schema migrations on startup from files embedded in the binary — no separate migration step or CLI.
- Ships with an optional bundled PostgreSQL subchart for demos, but defaults to expecting an external database.

This is the PoC scope described in README.md and CONTEXT.md; items under README.md's "Additions to Be Added on Official Release" are explicitly out of scope (see below).

## User Stories

1. As a mesh operator, I want to create a new API Key with an Owner label, so that I can issue a credential to a consumer and know who it belongs to later.
2. As a mesh operator, I want to optionally set an expiry date when creating a key, so that short-lived credentials expire without manual cleanup.
3. As a mesh operator, I want the full API Key (`api_<Key ID>_<secret>`) shown to me exactly once at creation time, so that I can hand it to the consumer, knowing it can never be retrieved again.
4. As a mesh operator, I want the secret portion of a key hashed before storage, so that a database compromise doesn't leak usable credentials.
5. As a mesh operator, I want to see a list of all existing keys with their Key ID, Owner, creation date, and expiry (if any), so that I can audit what's currently issued.
6. As a mesh operator, I want the key list to never display the secret or its hash, so that the UI itself can't leak credential material.
7. As a mesh operator, I want to delete/revoke a key from the Web UI, so that a compromised or decommissioned credential immediately stops working.
8. As a consumer service, I want to authenticate my requests by presenting `X-API-Key`, so that I can access mesh services protected by the check service.
9. As a mesh operator, I want the check service to reject requests with a missing `X-API-Key` header, so that unauthenticated traffic is denied by default.
10. As a mesh operator, I want the check service to reject requests with an unknown or malformed API Key, so that guessed or fabricated keys don't get through.
11. As a mesh operator, I want the check service to reject requests made with an expired key, so that expiry is actually enforced, not just recorded.
12. As a mesh operator, I want the check service to reject requests made with a revoked/deleted key, so that revocation from the UI takes effect immediately.
13. As an upstream service developer, I want the authenticated request to carry the key's Owner as a response header, so that I get caller attribution for free without doing my own lookup.
14. As a mesh operator, I want Istio to fail closed if the check service is unreachable or erroring, so that a checker outage can't silently turn into an open mesh.
15. As a mesh operator, I want per-key request metrics exported in Prometheus format labeled by Key ID, so that I can build dashboards and alerts outside this service.
16. As a mesh operator, I want the Web UI to show a usage graph for an individual key, so that I can see its traffic pattern without learning PromQL.
17. As a mesh operator, I want the Web UI's usage graph to be sourced live from Prometheus rather than a duplicated counter in Postgres, so that there's a single source of truth for usage data.
18. As a platform engineer, I want to `helm install` this service against an external Postgres instance with no extra flags, so that the chart matches the documented default (BYO database).
19. As a platform engineer, I want to set `postgresql.enabled=true` to get a bundled Postgres instance, so that I can demo or run the service locally without provisioning an external database.
20. As a platform engineer, I want database credentials supplied via a `DATABASE_URL` sourced from a Kubernetes Secret, so that credentials never live in values.yaml or a ConfigMap.
21. As a platform engineer, I want schema migrations to run automatically on startup, so that there's no separate migration job or manual `migrate` CLI step in the deployment pipeline.
22. As a security-conscious platform engineer, I want the Web UI's Service to default to `ClusterIP`, so that the unauthenticated management surface isn't reachable outside the cluster unless I explicitly opt in.
23. As a platform engineer, I want to explicitly opt in to exposing the Web UI externally via a values.yaml flag, so that external exposure is a deliberate decision, not an accident of chart defaults.
24. As an operator running this service, I want it instrumented with OpenTelemetry traces and metrics, so that I can plug it into my existing observability stack.
25. As an operator debugging an incident, I want structured logs from the service, so that I can correlate check-service decisions with request context.
26. As a platform engineer, I want the whole service to ship as a single Go binary with web assets embedded, so that deployment is one container image with no separate static-asset hosting.
27. As a mesh operator, I want key creation to reject a request that omits the Owner label, so that every issued key is attributable.
28. As a mesh operator, I want the Key ID (not a separately stored masked value) to be what identifies a key in the UI, metrics, and audit trail, so that there's one identifier used consistently everywhere.

## Implementation Decisions

**Process shape.** One Go binary, one running process, serving three logical HTTP surfaces from the same server:

- **Management surface**: the embedded Web UI static assets + its backing JSON API (create/list/delete keys, fetch usage graphs).
- **Check surface**: the Envoy `ext_authz` HTTP check endpoint Istio calls on the data path (ADR-0001).
- **Metrics surface**: a Prometheus-format `/metrics` scrape endpoint (ADR-0004).

Keeping these on one server/one binary matches the "single Go binary" technical requirement and keeps the primary test seam to one HTTP boundary (see Testing Decisions).

**Key domain module.** Owns key generation, hashing, and validation:

- Generates keys as `api_<8-char Key ID>_<32-char secret>` (ADR-0002).
- Key ID is plaintext, used as the lookup key, the UI display value, and the Prometheus label value — no separate masked/display column.
- The secret is hashed before it ever reaches storage or a log line.
- Validation checks: key exists, secret hash matches, key not expired, key not revoked/deleted.

**Postgres store.** Metadata-only, per ADR-0004: key id, secret hash, owner, created_at, expires_at (nullable). Deleting a key from the UI removes/marks it in Postgres; historical Prometheus data for that Key ID is unaffected since usage isn't stored in Postgres at all (ADR-0004) — deletion is a metadata-only operation and has no bearing on historical metrics retention.

**Migrations.** SQL files embedded via `go:embed`, executed on startup through the `golang-migrate` library directly (not its CLI) (ADR-0005). No separate migration step in the deployment pipeline.

**Check endpoint contract.** Implements the shape Istio's `envoyExtAuthzHttp` extension provider expects:

- Reads the API Key from the `X-API-Key` request header.
- On success: `200`, with the key's Owner injected as a response header for the upstream service to read.
- On missing/malformed/unknown/expired/revoked key: a non-2xx response (deny). Istio-side `AuthorizationPolicy` configuration for fail-closed behavior on checker errors is a deployment/Helm concern, not application logic (ADR-0001).

**Management API contract.**

- `POST` create-key: accepts Owner (required) and an optional expiry; returns the full key (`api_<id>_<secret>`) exactly once. This response is never reproducible — the API has no "reveal secret" endpoint.
- `GET` list-keys: returns Key ID, Owner, created_at, expires_at, revocation state — never the hash or secret.
- `DELETE` key-by-id: revokes/removes the key; check-endpoint lookups for that Key ID subsequently deny.
- Usage-graph endpoint: given a Key ID and time range, the Web UI backend queries Prometheus (via the `MetricsQuerier` interface below) and returns a time series to the frontend. Postgres is not involved in this path.

**MetricsQuerier interface.** A small interface wrapping calls to Prometheus's query API (e.g., a `QueryRange(keyID, timeRange)` method), with a concrete implementation backed by an HTTP client against Prometheus. This interface exists specifically because Prometheus is an external system outside this service's control and isn't practical to include in the primary HTTP-seam test environment (see Testing Decisions) — it is not a general "add an interface per dependency" pattern, and no equivalent interface is introduced for Postgres.

**Observability.** OpenTelemetry instrumentation for traces and metrics across the HTTP handlers on all three surfaces (ADR-0004). Logs remain plain structured logging for the PoC — no log pipeline/aggregation is part of this scope.

**Helm chart.**

- Deployment + Service templates; the management/Web UI Service defaults to `ClusterIP` (ADR-0003), with external exposure behind an explicit values.yaml flag.
- `DATABASE_URL` sourced from a Kubernetes `Secret`, never inlined into a ConfigMap or values.yaml.
- PostgreSQL declared as an optional chart dependency (e.g. the Bitnami `postgresql` subchart), gated by `postgresql.enabled`, defaulting to `false` (ADR-0005). Default `helm install` targets an external database.

## Testing Decisions

**What makes a good test here.** Tests should exercise the service the way Istio, an operator, and the Web UI frontend actually do — over HTTP, asserting on status codes, response headers (notably the injected Owner header), and response bodies. Tests should not reach into internal packages to call handler logic directly, assert on internal function calls, or mock Postgres — the goal is confidence that the wired binary behaves correctly, not that any given internal function was called.

**Primary seam: the HTTP boundary of the fully wired binary.** Stand up the real server (all three surfaces) against a real Postgres instance (e.g. via `testcontainers-go`), migrations included, and drive it with real HTTP requests. This is the single seam covering the large majority of behavior:

- Key lifecycle: create (with/without expiry, with/without Owner), list, delete, and the resulting effect on the check endpoint (a deleted/expired key starts denying).
- Check endpoint: valid key → 200 + Owner header; missing header → deny; unknown key → deny; expired key → deny; revoked key → deny.
- `/metrics`: scrape returns Prometheus exposition format with the expected per-Key-ID label after some check-endpoint traffic has occurred.

**Secondary seam: `MetricsQuerier`.** Stubbed with an in-memory fake in tests that exercise the Web UI's usage-graph endpoint, since real Prometheus is a non-deterministic external system out of scope for this service's own test suite. This is the only seam besides the HTTP boundary — introduced solely because Prometheus can't be treated as a same-process test double the way Postgres can.

**Prior art.** None yet — there is no existing test suite in this repo. The tests written for this spec effectively set the precedent (harness setup, testcontainers usage, fixture conventions) that later vertical slices should follow rather than reinvent.

**Helm chart testing.** Validated via `helm lint` and `helm template` (asserting on rendered manifests — e.g. Service type defaults, Secret references, conditional subchart inclusion) rather than a full cluster/e2e test. Live Istio integration testing is out of scope (see below).

## Out of Scope

- Everything listed under README.md's "Additions to Be Added on Official Release": Kubernetes operator + CRDs, storing tokens in Kubernetes Secrets/AWS Secrets Manager/Azure Key Vault, auto-rolling/rotation strategies, and per-service/per-route key scoping.
- Web UI authentication (RBAC/SSO) — explicitly deferred per ADR-0003.
- A full cluster end-to-end test with a real Istio control plane and `AuthorizationPolicy` wiring; this spec's Istio-facing behavior is verified by testing the check endpoint's HTTP contract directly, not by running Istio itself.
- Deploying or operating the OpenTelemetry collector/backend or Prometheus itself — this spec only covers instrumenting the service and querying an assumed-reachable Prometheus.
- Any "reveal secret after creation" capability — by design, the secret is shown exactly once.
- Log aggregation/shipping — structured logs to stdout only.

## Further Notes

- **Likely vertical slice boundaries** (for the later ticket split): (1) key domain + Postgres store + migrations, (2) check-service endpoint, (3) management API + Web UI shell for create/list/delete, (4) Prometheus metrics export + Web UI usage graphs, (5) Helm chart. This is a suggestion for slicing, not a decision made here.
- **Open question, not covered by any ADR**: what should the Web UI's usage-graph view do if Prometheus is unreachable at query time (hard error vs. a degraded "usage unavailable" state)? Worth resolving when that vertical slice is ticketed, and possibly worth its own ADR if the answer is non-obvious.
- No conflicts with existing ADRs were found while writing this spec.
