# 07: Containerize the service

**What to build:** A Dockerfile that builds the single Go binary (with its embedded web assets and migrations) into a minimal, runnable container image, plus a Docker Compose file that runs the service alongside a Postgres instance so a developer can bring the whole stack up locally with one command, without a cluster. This image is what the Helm chart (ticket 08) deploys.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] A multi-stage Dockerfile compiles the Go binary (web assets and migrations embedded via `go:embed`, per ticket 01) and copies only the resulting binary into a minimal runtime base image — no Go toolchain or source in the final image.
- [ ] The container runs as a non-root user and exposes only the port(s) the service listens on.
- [ ] The image reads its configuration the same way the binary already does (`DATABASE_URL` from the environment, per ticket 01) — no container-specific config path is introduced.
- [ ] A `docker-compose.yml` at the repo root starts the service container and a Postgres container together, wiring `DATABASE_URL` between them, so `docker compose up` gives a working local stack with no manual setup.
- [ ] After `docker compose up`, the service's health endpoint (ticket 01) reports healthy, confirming migrations ran against the Compose Postgres instance.
- [ ] A short section in the README or docs explains how to build the image and run the stack via Compose for local testing.
