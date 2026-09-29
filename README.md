# Istio API Key Authenticator - PoC

This repo contains a prototype API Key service, designed to work with Istio in a kubernetes cluster.

## Features

- A Web UI for managing API Keys (create, delete, view metrics).
- Metrics, to enable usage tracking of API keys. Exportable to prometheus.
- API that will integrate with Istio to provide Authentication checks on incoming traffic.
- Keys are persisted in a PostgreSQL Database (external by default; the Helm chart can optionally deploy a bundled instance).

## Running Locally with Docker

Build the image and bring up the service alongside a Postgres instance with:

```sh
docker compose up --build
```

This starts a `postgres:16-alpine` container and the service container, wiring `DATABASE_URL` between them. The service waits for Postgres to report healthy before starting, then applies its embedded schema migrations on boot.

Once the stack is up, confirm the service is healthy (this also confirms migrations ran against the Compose Postgres instance):

```sh
curl -i http://localhost:8080/healthz
```

A `200 OK` response means the service is connected to Postgres with migrations applied.

To build the image on its own (e.g. for pushing to a registry):

```sh
docker build -t istio-api-key-auth-poc .
```

## Additions to Be Added on Official Release

- Deployed as a Kubernetes operator with CRDs to aid with the creating of API Keys.
- Ability to store tokens in Kubernetes Secrets, AWS Secrets Manager or Azure Key Vault
- Auto-rolling strategies
- Key scoping (per-service / per-route API keys, rather than mesh-wide)

## Technical Requirements

- Written in Golang, single binary, any web files should be embedded.
- Containerised and ready to ship into a Kubernetes cluster using a Helm Chart
- OpenTelemetry Instrumented (perhaps using the new [otelc](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation) tool)