# 02: Weighted check-mix across every check-decision scenario

**What to build:** The check stream sends a configurable weighted mix of valid, unknown/malformed, expired, and missing-header requests, so every `check_service.decisions` reason gets exercised in realistic proportions instead of only the valid case.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] A pure, unit-tested weighted-mix selector chooses a scenario (valid / unknown-or-malformed / expired / missing-header) per request, given a set of weights and a deterministic random source.
- [ ] Default weights are 70% valid / 15% unknown-or-malformed / 10% expired / 5% missing-header.
- [ ] CLI flags allow overriding each scenario's weight.
- [ ] "Valid" draws a key from the shared pool; "unknown" sends a malformed or well-formed-but-nonexistent key; "expired" always uses the dedicated pre-expired key from ticket 01; "missing" omits the `X-API-Key` header entirely.
- [ ] Unit tests confirm the selector produces the expected long-run distribution across a large sample, and resolves each scenario to the right key-handling behavior.
- [ ] Running the tool against a live service produces all of the check endpoint's outcomes (allowed, `missing key`, `invalid key`, `key has expired`) in roughly the configured proportions.
