# telemetrygen

`telemetrygen` is a dev-only load-generation tool that drives sustained, realistic traffic against a running instance of this service, to exercise its telemetry-emitting surfaces — the Check Service's `/authz/check` endpoint and the management API — without standing up a real Istio mesh.

It's useful for:

- Sanity-checking that `check_service.decisions` and `management_api.requests` metrics are recorded correctly, across every decision outcome, under sustained load.
- Generating realistic-looking Web UI usage data (see the Web UI's usage graphs) without manually clicking through key creation/revocation.
- Watching a dashboard or `/metrics` scrape settle into a steady state.

It is **not** part of the shippable product: it's not referenced by `Dockerfile` or `docker-compose.yml`, and it isn't built into the service's image.

## What it does

On startup, the tool:

1. Bootstraps a shared pool of valid API Keys via the management API (`--pool-size` of them), plus one dedicated key created with an `expires_at` already in the past, held outside the pool.
2. Takes a baseline scrape of `/metrics`, so its final summary can report only what changed during this run.

It then runs three concurrent loops until stopped:

- **Check stream** — sends requests to `/authz/check` at a fixed target rate (`--check-rate`). Each request is one of four scenarios, chosen per configurable weights:
  - `valid` — draws a key from the shared pool.
  - `unknown` — sends either a structurally malformed key or a well-formed-but-never-issued key.
  - `expired` — always uses the dedicated pre-expired key.
  - `missing` — omits the `X-API-Key` header entirely.
- **Management stream** — cycles through `list` → `create` → `revoke` calls against the management API at its own target rate (`--management-rate`), independent of the check stream. `create` adds the new key into the shared pool; `revoke` picks a random active key from the pool, revokes it, and removes it from the pool — so the check stream naturally starts seeing it as invalid.
- **Progress reporter** — logs a snapshot of client-side sent/failed counts, per check-mix scenario and per management operation, every 10 seconds.

Both traffic streams are open-loop (ticker-driven against their target rate, not tied to response latency), so a slow or hanging response never throttles the send rate. Per-request failures during the run are logged and counted, not fatal.

On exit — whether from an interrupt (Ctrl+C/SIGTERM), `--duration` elapsing, or otherwise — the tool scrapes `/metrics` one more time and prints a final summary comparing what it believes it sent against what Prometheus actually recorded for `check_service.decisions` and `management_api.requests`, flagging any mismatch. Keys created during the run, including the dedicated expired key, are never deleted or modified on exit — they're left in place so the Web UI and `/metrics` continue to reflect the run afterwards.

Keys the tool creates are labeled with an owner so they're identifiable elsewhere (e.g. in the Web UI) as telemetrygen's own throwaway keys: pool keys as `telemetrygen`, the dedicated expired key as `telemetrygen-expired`, and keys created by the management stream as `telemetrygen-managed`.

## Usage

```sh
go run ./tools/telemetrygen [flags]
```

### Flags

| Flag | Default | Description |
| --- | --- | --- |
| `--base-url` | `http://localhost:8080` | Base URL of the running service. |
| `--check-rate` | `10` | Target rate, in requests/sec, for the check-endpoint stream. |
| `--management-rate` | `6` | Target rate, in calls/min, for the management-API stream (list/create/revoke). |
| `--pool-size` | `20` | Number of valid API keys to bootstrap into the shared pool. |
| `--duration` | unset (runs until interrupted) | Optional total run duration (e.g. `5m`). |
| `--weight-valid` | `70` | Relative weight of the check stream's valid-key scenario. |
| `--weight-unknown` | `15` | Relative weight of the check stream's unknown-or-malformed-key scenario. |
| `--weight-expired` | `10` | Relative weight of the check stream's expired-key scenario. |
| `--weight-missing` | `5` | Relative weight of the check stream's missing-header scenario. |

Weight flags are proportional, not required to sum to 100 — only their ratios matter.

### Example: running against a local `docker-compose` instance

```sh
docker compose up --build -d

go run ./tools/telemetrygen --duration 2m

curl -s http://localhost:8080/metrics | grep -E 'check_service_decisions_total|management_api_requests_total'
```

This bootstraps a 20-key pool, runs the default check-mix and management traffic for two minutes, prints progress every 10 seconds, and ends with a summary comparing its own tallies against the service's `/metrics` output.
