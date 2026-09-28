# 01: Service skeleton boots and applies schema migrations

**What to build:** The single Go binary starts up, reads its Postgres connection details from a `DATABASE_URL` environment variable, and automatically applies its embedded schema migrations before serving traffic. A health endpoint confirms the service is up and the database connection is live. This is the foundation every later ticket builds on — no key domain logic, API, or UI yet.

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] The binary reads `DATABASE_URL` from its environment and connects to Postgres on startup.
- [ ] Schema migrations are embedded in the binary (`go:embed`) and applied automatically on startup via the `golang-migrate` library — no separate CLI step or migration job.
- [ ] The initial migration creates the `keys` metadata table (columns matching ADR-0002/ADR-0004: Key ID, secret hash, Owner, created_at, expires_at) even though nothing reads/writes it yet.
- [ ] A health endpoint returns healthy only when the database connection is actually live (not just "process is running").
- [ ] If the database is unreachable or migrations fail, the binary fails to start with a clear error rather than serving traffic against a broken schema.
- [ ] An HTTP-level test starts the wired binary against a real Postgres (e.g. via testcontainers) and asserts the health endpoint succeeds and the `keys` table exists afterward.
