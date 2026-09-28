# 0003. No authentication on the Web UI for the PoC

Date: 2026-09-28

Status: Accepted

## Context

The Web UI manages creation and deletion of API keys, which are themselves the mesh's auth mechanism. Building real UI authentication (RBAC/SSO) is significant scope for a PoC.

## Decision

The PoC ships with no authentication on the management UI. This is an explicit, documented gap rather than an oversight. The Helm chart defaults the UI's Service to `ClusterIP` so it isn't reachable outside the cluster unless explicitly opted into.

## Consequences

- The chart must not default to exposing the UI externally; external exposure is an opt-in values.yaml flag.
- Full UI authentication is deferred to the official release, alongside the other secrets-management work already listed in README.md.
