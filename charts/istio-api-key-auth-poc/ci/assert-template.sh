#!/usr/bin/env bash
# Asserts on `helm template` output for both the default values (external DB)
# and postgresql.enabled=true (bundled DB) cases. Run from the chart directory
# after `helm dependency build`.
set -euo pipefail

CHART_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$CHART_DIR"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

echo "== helm lint =="
helm lint .

echo "== default values =="
default_out="$(helm template rel . )"

echo "$default_out" | grep -q "kind: Service" || fail "no Service rendered with default values"
echo "$default_out" | grep -A1 "^spec:" | grep -q "type: ClusterIP" \
  || fail "Service does not default to ClusterIP"

echo "$default_out" | grep -A2 "name: DATABASE_URL" | grep -q "secretKeyRef" \
  || fail "DATABASE_URL is not sourced from a Secret"
echo "$default_out" | grep -A2 "name: DATABASE_URL" | grep -qE "^\s*value:" \
  && fail "DATABASE_URL must not be set as a literal value"

echo "$default_out" | grep -q "kind: StatefulSet" \
  && fail "postgresql subchart resources rendered even though postgresql.enabled is false"

echo "default values: OK"

echo "== postgresql.enabled=true =="
pg_out="$(helm template rel . --set postgresql.enabled=true)"

echo "$pg_out" | grep -q "kind: StatefulSet" \
  || fail "postgresql subchart was not included when postgresql.enabled=true"

echo "$pg_out" | grep -q "name: rel-istio-api-key-auth-poc-db" \
  || fail "bundled-postgres DATABASE_URL Secret not rendered"
echo "$pg_out" | grep -A10 "name: rel-istio-api-key-auth-poc-db" | grep -q "DATABASE_URL:.*@rel-postgresql:5432" \
  || fail "bundled-postgres DATABASE_URL does not point at the subchart's Postgres service"

echo "postgresql.enabled=true: OK"

echo "All helm chart assertions passed."
