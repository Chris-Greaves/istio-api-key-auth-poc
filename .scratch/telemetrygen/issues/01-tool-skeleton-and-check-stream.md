# 01: Tool skeleton, key pool bootstrap, and single-scenario check stream

**What to build:** A runnable `./tools/telemetrygen` command that stands up a shared pool of throwaway API Keys against a running instance of the service, then sends a steady, rate-controlled stream of valid-key requests to the Check Service's `/authz/check` endpoint until stopped.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] `./tools/telemetrygen` runs via `go run ./tools/telemetrygen [flags]` as its own `package main` in the existing Go module, and is not referenced by `Dockerfile` or `docker-compose.yml`.
- [ ] CLI flags configure at least: the service's base URL (default `http://localhost:8080`), the check-stream's target rate (requests/sec), and an optional run duration.
- [ ] At startup, the tool bootstraps a shared pool of valid API Keys via the management API, plus creates one dedicated key with an `expires_at` already in the past, held outside the pool.
- [ ] A failure during bootstrap (e.g. an unreachable base URL) aborts the tool immediately with a clear error message.
- [ ] Once running, the tool sends requests to `/authz/check` at the configured target rate, each drawing a valid key from the pool and presenting it via `X-API-Key`.
- [ ] The check stream's rate is open-loop (ticker-driven against a target rate), not tied to response latency.
- [ ] The tool runs until interrupted (Ctrl+C/SIGTERM) or, if `--duration` is set, until that duration elapses, then shuts down gracefully.
- [ ] No keys created during the run (including the dedicated expired key) are deleted or modified on exit.
