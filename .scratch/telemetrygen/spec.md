# Spec: Telemetry load-generator tool (`./tools/telemetrygen`)

Status: ready-for-agent

## Problem Statement

There's currently no way to generate realistic, sustained traffic against this service's telemetry-emitting surfaces — the Check Service's ext_authz endpoint and the management API — without manually curling endpoints or standing up a full Istio mesh. A developer who wants to verify that the `check_service.decisions` and `management_api.requests` Prometheus counters (ADR-0004) behave correctly, or wants to watch dashboards populate with realistic-looking data over time, has no tooling to do that.

## Solution

A standalone Go CLI tool under `./tools/telemetrygen`, in the same module as the service, that runs two independent, rate-controlled traffic streams directly against a running instance of the service (no Istio/mesh required):

- A **check-endpoint stream** hitting `/authz/check` with a configurable weighted mix of valid, unknown/malformed, expired, and missing-header requests, drawing "valid" keys from a shared, self-managed pool of throwaway API Keys.
- A **management-API stream** independently calling `list`/`create`/`revoke`, mutating that same shared pool over time (so revoked/newly-created keys realistically feed back into what the check stream sees).

The tool runs as a soak generator (indefinitely, or for an optional `--duration`), reports periodic progress, and on exit scrapes the service's own `/metrics` endpoint to print what Prometheus actually recorded alongside what the tool believes it sent — closing the loop on "did the telemetry actually land."

## User Stories

1. As a developer testing this service's telemetry, I want a standalone tool that generates realistic traffic against the check endpoint and management API, so that I can verify Prometheus metrics behave correctly without manually curling endpoints.
2. As a developer, I want the tool to run as a sustained load/soak generator rather than a one-shot smoke test, so that I can observe telemetry accumulate realistically over time on a dashboard.
3. As a developer, I want the tool to target both `check_service.decisions` and `management_api.requests`, so that every telemetry-emitting surface in the service gets exercised, not just one.
4. As a developer, I want the tool to hit the service directly over HTTP rather than requiring a real Istio mesh, so that I can test telemetry generation without standing up a full mesh.
5. As a developer, I want to point the tool at a configurable base URL, so that I can run it against `docker-compose` locally or any other reachable instance of the service.
6. As a developer, I want the tool to create and manage its own throwaway API Keys automatically, so that I don't have to manually provision keys before running it.
7. As a developer, I want to optionally supply an existing API Key via a flag, so that I can include a key I care about in the generated traffic.
8. As a developer, I want the check-endpoint traffic split across valid, missing-header, unknown/malformed, and expired-key scenarios in a configurable weighted mix, so that every check-decision reason gets exercised in realistic proportions.
9. As a developer, I want a sensible default mix (70% valid / 15% unknown / 10% expired / 5% missing) out of the box, so that I get useful telemetry without having to tune flags first.
10. As a developer, I want the management-API traffic (`list`/`create`/`revoke`) to run as its own independently-rated stream, so that `management_api.requests` gets steady telemetry regardless of what the check-endpoint stream is doing.
11. As a developer, I want the management stream's `create`/`revoke` calls to operate on the same shared key pool the check stream draws from, so that revoked/newly-created keys realistically affect what the check stream sees over the course of a run.
12. As a developer, I want a single dedicated pre-expired key created once at startup, so that the "key has expired" check-decision reason is exercised deterministically without waiting for real time to pass.
13. As a developer, I want both traffic streams rate-controlled independently of server response latency (open-loop), so that the volume of telemetry generated is a known, reproducible quantity I can cross-check against Prometheus.
14. As a developer, I want to configure the check-endpoint rate and the management-API rate via separate flags, so that I can tune the volume of each stream independently.
15. As a developer, I want the tool to run indefinitely by default until I stop it with Ctrl+C/SIGTERM, so that I can leave it running for an extended soak test.
16. As a developer, I want an optional `--duration` flag to auto-stop the run after a fixed time, so that I can script a bounded test run.
17. As a developer, I want the tool to leave every key it created (including revoked/expired ones) in place when it exits, so that I can inspect the resulting state afterward without it being cleaned up from under me.
18. As a developer, I want the tool to fail fast with a clear error if it can't bootstrap its initial key pool at startup, so that a misconfigured `--base-url` or unreachable service is caught immediately rather than silently producing zero telemetry.
19. As a developer, I want per-request failures during the steady-state run to be counted and logged rather than aborting the tool, so that transient errors don't kill a long-running soak test.
20. As a developer, I want periodic progress output while the tool runs, showing requests sent broken down by outcome and any failures, so that I can see the tool is working without waiting for it to exit.
21. As a developer, I want a final summary when the tool stops that scrapes `/metrics` and parses out `check_service.decisions` and `management_api.requests` by label, so that I can see what Prometheus actually recorded.
22. As a developer, I want that final summary to show the tool's own client-side counts alongside the scraped Prometheus counts, so that I can visually spot a mismatch between what was sent and what was recorded.
23. As a developer, I want the tool's core decision logic (pool state transitions, weighted scenario selection, metrics parsing/diffing) covered by unit tests, so that subtle bugs in that logic are caught without needing a running service.
24. As a developer, I want the tool's network/HTTP/CLI glue to not require its own automated test suite, so that verification effort is proportionate to a dev utility rather than treated like production service code.
25. As a developer, I want the tool documented in its own README under `./tools/telemetrygen/`, so that I can learn its flags and behavior without reading the source.
26. As a developer, I want the tool excluded from the service's shipped container image, so that a dev-only utility never ends up bundled into the production Docker image.
27. As a developer, I want the tool to live in the same Go module as the rest of the service, so that it can be run with a simple `go run ./tools/telemetrygen` without a separate build/dependency setup.
28. As a developer, I want the tool to require no changes to `docker-compose.yml`, so that running it doesn't introduce a second, redundant way to start the service stack.

## Implementation Decisions

**Location and shape.** `./tools/telemetrygen`, `package main`, in the existing Go module — run via `go run ./tools/telemetrygen [flags]`. Not referenced by `Dockerfile` (which builds `./cmd/server` explicitly) or `docker-compose.yml`.

**Targets.** The Check Service's `/authz/check` endpoint and the management API's create/list/revoke endpoints. `POST /api/keys/validate` is deliberately not targeted — it emits no telemetry today (see Further Notes).

**Connectivity.** Talks to a configurable base URL (default `http://localhost:8080`, matching `docker-compose`) directly over HTTP. No Istio/mesh involved — the check endpoint's behavior doesn't depend on how it's invoked.

**Shared key pool.** An in-process, mutex-guarded pool of API Keys tracking which are currently active. At startup the tool bootstraps the pool with an initial set of valid keys via the management API, plus creates one dedicated pre-expired key (an `expires_at` in the past) held outside the pool, used exclusively for the "expired" scenario. An optional flag lets the caller inject an existing key into the pool alongside the self-created ones.

**Two independent, open-loop traffic streams** (rate-controlled via a ticker against a target rate, not tied to response latency):
- *Check stream*: target requests/sec. Picks a scenario per the weighted mix (default 70% valid / 15% unknown-or-malformed / 10% expired / 5% missing-header; weights configurable via flags). "Valid" draws a key from the shared pool; "unknown" constructs a malformed or well-formed-but-nonexistent key; "expired" always uses the dedicated pre-expired key; "missing" omits the `X-API-Key` header entirely.
- *Management stream*: target calls/min, independent of the check stream's rate. Cycles through `list`, `create` (adds a new key into the shared pool), and `revoke` (removes a random active key from the shared pool — which also stops the check stream from drawing it as "valid" from that point on).

**Lifecycle.** Runs until SIGINT/SIGTERM or an optional `--duration` elapses, using a cancellable context in the same shutdown style as `cmd/server`. No cleanup of created or revoked keys on exit — the resulting Postgres state is left as-is for inspection.

**Error handling.** Bootstrap failures (initial pool/dedicated-key creation) are fatal — the tool aborts immediately with a clear error, since that usually means a bad `--base-url` or an unreachable service. Steady-state per-request failures during the run are counted per outcome bucket and logged, not fatal.

**Progress and reporting.** Periodic progress lines while running, showing requests sent broken down by outcome/operation and any failure counts. On exit, a final summary scrapes `/metrics`, parses the Prometheus exposition text for `check_service.decisions` and `management_api.requests`, breaks the values out by label, and prints them alongside the tool's own client-side tallies for direct comparison.

**Configuration surface.** CLI flags only (no environment variables) — e.g. base URL, check-stream rate, management-stream rate, duration, an existing-key override, and overrides for the check-mix weights. Exact flag names and numeric defaults are left to the implementing agent's judgment.

## Testing Decisions

**What makes a good test here.** Only the tool's pure, deterministic decision/state logic is unit tested — no real network calls, no goroutines, no real clock. The HTTP/CLI/ticker glue that wires that logic to the outside world is not covered by an automated suite; it's verified manually by running the tool against `docker-compose up`.

**Modules to test:**
- The key-pool state machine: creating a key adds it as active; revoking removes it; looking up an active key vs. the dedicated (always-expired) key behaves correctly.
- The weighted-mix selector: given a set of weights and a seeded/deterministic random source, it selects scenarios in the expected long-run proportions and resolves each scenario to the right kind of key (or no key, for "missing").
- The metrics parser/diff: given sample Prometheus exposition text, it correctly extracts `check_service.decisions` and `management_api.requests` values broken out by label, and correctly diffs them against expected counts.

**Not tested automatically:** the real HTTP client calls, the rate-limiting tickers, signal handling, and CLI flag parsing.

**Prior art.** This repo's existing unit tests for pure/deterministic logic (`internal/keys/validate_test.go`, `internal/config/config_test.go`, `internal/metricsquery/metricsquery_test.go`) are the closest precedent for testing this tool's pool/mixer/metrics-diff logic directly, in isolation, the same way. `internal/app/app_test.go`'s HTTP-boundary style (used for the service itself) is deliberately *not* mirrored here — this tool doesn't own an HTTP surface; its differentiating logic is the pure decision layer, not a server.

## Out of Scope

- Exercising `POST /api/keys/validate` — it emits no telemetry today, so there's nothing for this tool to verify there.
- Running against a real Istio mesh, or testing `ext_authz`/`AuthorizationPolicy` wiring itself — the tool talks to the service directly over HTTP.
- Any cleanup or deletion of keys created during a run.
- Changes to `docker-compose.yml` or `Dockerfile` — this is a host-only tool, explicitly excluded from the shipped image.
- Automated end-to-end/HTTP-level tests of the tool itself.
- A root `README.md` section or any documentation beyond the tool's own `./tools/telemetrygen/README.md`.
- Any new server-side telemetry, metrics, or endpoints — this tool only exercises what already exists in the service.

## Further Notes

- **Revoked vs. unknown keys are telemetrically identical at the check endpoint.** `Store.Get` (`internal/keys/store.go`) returns `ErrKeyNotFound` for a revoked key exactly as it does for a nonexistent one, so `keys.Validate` reports the same generic `"invalid key"` reason with no `key_id` label either way (`internal/keys/validate.go`). The management stream's `revoke` calls are therefore valuable for exercising `management_api.requests{operation="revoke_key"}`, not for producing a distinct check-decision reason — the check-mix's "unknown" bucket already covers that outcome.
- Exact default rates, the progress-reporting interval, and specific flag names are intentionally left as implementation judgment calls; this spec fixes behavior and architecture, not precise defaults.
