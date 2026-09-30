# Istio API Key Authenticator - PoC

This repo contains a prototype API Key service, designed to work with Istio in a kubernetes cluster.

## Features

- A Web UI for managing API Keys (create, delete, view metrics).
- Metrics, to enable usage tracking of API keys. Exportable to prometheus.
- API that will integrate with Istio to provide Authentication checks on incoming traffic.
- Keys are persisted in a PostgreSQL Database (external by default; the Helm chart can optionally deploy a bundled instance).

## Running Locally with Docker

Build the image and bring up the service alongside Postgres and Prometheus with:

```sh
docker compose up --build
```

This starts a `postgres:16-alpine` container, a `prom/prometheus` container (scraping the service's `/metrics` every 5s, per `prometheus.yml`), and the service container, wiring `DATABASE_URL` and `PROMETHEUS_URL` between them. The service waits for Postgres to report healthy before starting, then applies its embedded schema migrations on boot. Prometheus backs the Web UI's per-key usage graphs (ADR-0004); without it, the Usage view reports "unavailable" (see `tools/telemetrygen` for generating traffic to see it populated).

Once the stack is up, confirm the service is healthy (this also confirms migrations ran against the Compose Postgres instance):

```sh
curl -i http://localhost:8080/healthz
```

A `200 OK` response means the service is connected to Postgres with migrations applied.

To build the image on its own (e.g. for pushing to a registry):

```sh
docker build -t istio-api-key-auth-poc .
```

## Deploying with Helm

The chart at `charts/istio-api-key-auth-poc` deploys the container image built above into a Kubernetes cluster. By default it targets an external Postgres instance: create a `Secret` named `<release>-istio-api-key-auth-poc-db` with a `DATABASE_URL` key before installing, then run:

```sh
helm dependency build charts/istio-api-key-auth-poc
helm install my-release charts/istio-api-key-auth-poc
```

For a demo or local cluster with no external database, deploy a bundled Postgres instance instead:

```sh
helm install my-release charts/istio-api-key-auth-poc --set postgresql.enabled=true
```

The management Web UI's Service defaults to `ClusterIP` (see ADR-0003); set `service.type` to `NodePort`/`LoadBalancer` to opt in to external exposure.

## Using with Istio

See `docs/istio-integration.md` for a full walkthrough of registering the check service as an Istio `ext_authz` provider and enforcing it on a workload with an `AuthorizationPolicy`.

## Additions to Be Added on Official Release

- Deployed as a Kubernetes operator with CRDs to aid with the creating of API Keys.
- Ability to store tokens in Kubernetes Secrets, AWS Secrets Manager or Azure Key Vault
- Auto-rolling strategies
- Key scoping (per-service / per-route API keys, rather than mesh-wide)

## Technical Requirements

- Written in Golang, single binary, any web files should be embedded.
- Containerised and ready to ship into a Kubernetes cluster using a Helm Chart
- OpenTelemetry Instrumented (perhaps using the new [otelc](https://github.com/open-telemetry/opentelemetry-go-compile-instrumentation) tool)