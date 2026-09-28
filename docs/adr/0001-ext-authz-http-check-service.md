# 0001. Use Envoy ext_authz HTTP check service for Istio integration

Date: 2026-09-28

Status: Accepted

## Context

The service needs to authenticate incoming mesh traffic against API keys, on Istio's behalf. Istio supports a few mechanisms for this: an Envoy `ext_authz` HTTP check service, an `ext_authz` gRPC check service, or a WASM plugin injected into sidecars.

## Decision

Use an `ext_authz` HTTP check service, registered via Istio's `extensionProviders.envoyExtAuthzHttp` and enforced with an `AuthorizationPolicy`.

- Clients present the key via the `X-API-Key` header.
- On a valid key, the checker injects the key's `Owner` as a response header so the upstream service gets attribution for free.
- If the checker is unreachable or errors, Istio is configured to fail **closed** (deny traffic) rather than fail open.

## Consequences

- Simpler to build and operate as a single Go binary than a gRPC service or WASM plugin would require.
- Failing closed means checker downtime blocks all traffic protected by the policy. Acceptable for the PoC; worth revisiting if the checker needs a high-availability story in production.
