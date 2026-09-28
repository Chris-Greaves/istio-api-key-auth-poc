# 04: Check service enforces key validity

**What to build:** The Envoy `ext_authz` HTTP Check Service endpoint (ADR-0001) validates the `X-API-Key` header on incoming requests against the keys created/revoked in tickets 02–03. A request with a valid, non-expired, non-revoked key is allowed through with the key's Owner injected as a response header; every other case is denied.

**Blocked by:** 02, 03

**Status:** ready-for-agent

- [ ] A request with a valid, active key returns a success response with the key's Owner injected as a response header.
- [ ] A request with a missing `X-API-Key` header is denied.
- [ ] A request with an unknown/malformed key is denied.
- [ ] A request with an expired key (`expires_at` in the past) is denied.
- [ ] A request with a revoked key (from ticket 03) is denied.
- [ ] The check endpoint emits an OpenTelemetry trace/metric for each decision (allow/deny), tagged with enough context to debug a denial without reading application logs.
- [ ] An HTTP-level test against the wired binary + real Postgres covers the full valid/missing/unknown/expired/revoked matrix, asserting both status code and the Owner header on success.
