# 08: Helm chart deployment

**What to build:** A platform engineer can `helm install` this service into a cluster. By default it targets an external Postgres instance; setting `postgresql.enabled=true` deploys a bundled instance instead. The management/Web UI surface defaults to not being reachable outside the cluster.

**Blocked by:** 01, 07

**Status:** ready-for-agent

- [ ] The chart's Deployment runs the service's container image (built in ticket 07), sourcing `DATABASE_URL` from a Kubernetes `Secret` (never inlined into values.yaml or a ConfigMap).
- [ ] The management/Web UI Service defaults to `ClusterIP` (ADR-0003); external exposure is only possible via an explicit values.yaml flag.
- [ ] PostgreSQL is declared as an optional chart dependency (e.g. the Bitnami `postgresql` subchart) gated by `postgresql.enabled`, defaulting to `false` — default `helm install` expects an external database and does not deploy Postgres.
- [ ] Setting `postgresql.enabled=true` deploys a bundled Postgres instance and wires its connection details into the same `DATABASE_URL` Secret path used for the external-DB case.
- [ ] `helm lint` passes, and `helm template` is asserted (in CI) against both the default values and `postgresql.enabled=true`, checking the Service type default, Secret reference, and conditional subchart inclusion.
