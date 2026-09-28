# 03: Revoke API Keys

**What to build:** An operator can revoke/delete an existing API Key from the Web UI. Once revoked, the key no longer appears in the key list. (Enforcement that a revoked key actually stops authenticating traffic is covered by ticket 04, which depends on this one.)

**Blocked by:** 02

**Status:** ready-for-agent

- [ ] `DELETE` key-by-Key-ID removes/marks the key as revoked in Postgres.
- [ ] Deleting a nonexistent Key ID returns a clear not-found response rather than succeeding silently.
- [ ] The Web UI's key list has a delete/revoke action per row, with a confirmation step before it takes effect.
- [ ] After revocation, the key no longer appears in the `GET` list-keys response or the Web UI list.
- [ ] An HTTP-level test against the wired binary + real Postgres covers: creating a key, revoking it, and confirming it's absent from the subsequent list-keys response.
