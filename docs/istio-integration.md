# Using this service with Istio

This walks through wiring the check service (ADR-0001) into an Istio mesh as an
`ext_authz` HTTP authorization provider, and enforcing it on a workload with an
`AuthorizationPolicy`. It assumes the Helm chart is already installed (see
README.md) and Istio is present in the cluster.

## How it fits together

- The chart's Service (`<release>-istio-api-key-auth-poc`) exposes the check
  endpoint at `/authz/check`, alongside the management API and Web UI.
- Istio's `meshConfig.extensionProviders` registers that Service as an
  `envoyExtAuthzHttp` provider — a mesh-wide, named authorizer any
  `AuthorizationPolicy` can reference.
- An `AuthorizationPolicy` with `action: CUSTOM` tells Istio to call that
  provider for matching requests before they reach the workload. A 2xx
  response allows the request through; anything else denies it.
- On success, the checker injects an `X-Api-Key-Owner` response header, which
  Istio forwards to the upstream workload — the app gets key attribution for
  free without doing any auth itself.

Registering the extension provider is the one piece that isn't a CRD — it's
global mesh config, set on istiod itself. Everything downstream of that
(which workloads enforce it, how traffic is routed to them) is ordinary
Istio CRDs.

## 1. Register the extension provider

Add this to istiod's Helm values (or merge into an existing `meshConfig`) and
install/upgrade istiod with it:

```yaml
# istiod-values.yaml
meshConfig:
  extensionProviders:
  - name: api-key-checker
    envoyExtAuthzHttp:
      service: my-release-istio-api-key-auth-poc.default.svc.cluster.local
      port: 8080
      timeout: 2s
      failOpen: false            # ADR-0001: fail closed, not open
      includeRequestHeadersInCheck:
      - X-API-Key
      headersToUpstreamOnAllow:
      - X-API-Key-Owner
      pathPrefix: "/authz/check"
```

```sh
helm upgrade istiod istio/istiod -n istio-system -f istiod-values.yaml
```

Adjust `service` to match your release name and namespace
(`<release>-istio-api-key-auth-poc.<namespace>.svc.cluster.local`).

`failOpen: false` matters: if the checker is unreachable or errors, Istio
denies traffic rather than letting it through unchecked (ADR-0001). Expect
mesh-wide outages of the checker to block everything it protects — that's
the intended, accepted tradeoff for this PoC, not a bug.

### The `pathPrefix` gotcha

Envoy's HTTP `ext_authz` filter always sends the check request to
`pathPrefix + <the original request's path>` — never to the bare prefix
alone. A request for `/get` becomes a check request for `/authz/check/get`,
not `/authz/check`. The check endpoint's route must be a subtree match (a
trailing-slash pattern in Go's `net/http` `ServeMux`) to catch this, which is
why `internal/httpapi/router.go` registers the handler at both `/authz/check`
and `/authz/check/`. If you ever see every request denied regardless of key
validity, check `check_service_decisions_total` in `/metrics` first — if it's
not incrementing at all, the check request isn't reaching the handler, which
usually means this path shape has regressed.

## 2. Enable sidecar injection and deploy a workload

```sh
kubectl create namespace demo
kubectl label namespace demo istio-injection=enabled
kubectl apply -n demo -f https://raw.githubusercontent.com/istio/istio/release-1.24/samples/httpbin/httpbin.yaml
```

Any sidecar-injected workload works here — this uses Istio's sample
`httpbin` service as a stand-in for "the thing you want to protect".

## 3. Enforce the check with an AuthorizationPolicy

```yaml
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: httpbin-require-api-key
  namespace: demo
spec:
  selector:
    matchLabels:
      app: httpbin
  action: CUSTOM
  provider:
    name: api-key-checker
  rules:
  - {}          # applies to all requests to the selected workload
```

```sh
kubectl apply -f authz-policy.yaml
```

Narrow `rules` (by path, method, source, etc.) the same way you would for any
other `AuthorizationPolicy` — the `CUSTOM` action only decides *whether* the
provider gets called for a given rule match, same as `ALLOW`/`DENY`.

## 4. (Optional) Expose it through the ingress gateway

To test from outside the mesh rather than pod-to-pod, route it through a
`Gateway` + `VirtualService`:

```yaml
apiVersion: networking.istio.io/v1
kind: Gateway
metadata:
  name: demo-gateway
  namespace: demo
spec:
  selector:
    istio: ingressgateway
  servers:
  - port:
      number: 80
      name: http
      protocol: HTTP
    hosts:
    - "httpbin.local"
---
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: httpbin
  namespace: demo
spec:
  hosts:
  - "httpbin.local"
  gateways:
  - demo-gateway
  http:
  - route:
    - destination:
        host: httpbin.demo.svc.cluster.local
        port:
          number: 8000
```

The `AuthorizationPolicy` is enforced identically regardless of whether
traffic arrives via the ingress gateway or from another in-mesh pod — it's
attached to the workload, not the route.

## 5. Try it

Create a key via the management API (see README.md), then send requests with
and without it:

```sh
# Create a key
curl -X POST http://<checker-service>:8080/api/keys \
  -H "Content-Type: application/json" \
  -d '{"owner":"demo-test"}'
# => {"key":"api_xxxxxxxx_...", "key_id":"xxxxxxxx", ...}

# No key: denied
curl -o /dev/null -w "%{http_code}\n" http://httpbin:8000/get
# => 401

# Bogus/expired/revoked key: denied
curl -o /dev/null -w "%{http_code}\n" -H "X-API-Key: bogus" http://httpbin:8000/get
# => 401

# Valid key: allowed, Owner header forwarded to the upstream
curl -H "X-API-Key: api_xxxxxxxx_..." http://httpbin:8000/get
# => 200, upstream sees X-Api-Key-Owner: demo-test
```

Revoking a key (`DELETE /api/keys/{keyID}`) or letting it pass its
`expires_at` produces the same 401 as an unknown key — the checker doesn't
distinguish these to the caller, only in its own telemetry (`reason` label on
`check_service_decisions_total`).

## Troubleshooting

- **Everything denied, including valid keys** — check `check_service_decisions_total`
  on the checker's `/metrics`. If it's not incrementing at all, the check
  request isn't reaching the handler (see the `pathPrefix` gotcha above).
  If it *is* incrementing with `result="denied"`, the key itself is the
  problem, not the wiring.
- **Checker downtime blocks all protected traffic** — expected per
  `failOpen: false` (ADR-0001), not a bug.
- **`AuthorizationPolicy` seems to have no effect** — confirm the workload's
  pod actually has the Istio sidecar injected (`kubectl get pod -o
  jsonpath='{.spec.containers[*].name}'` should list `istio-proxy`), and that
  `spec.selector` matches the workload's labels.

## See also

- ADR-0001 (`docs/adr/0001-ext-authz-http-check-service.md`) — why an HTTP
  `ext_authz` service, fail-closed rationale.
- ADR-0002 (`docs/adr/0002-api-key-format-and-lifecycle.md`) — key format,
  expiry, revocation.
- `CONTEXT.md` — glossary (Check Service, Key ID, Owner).
