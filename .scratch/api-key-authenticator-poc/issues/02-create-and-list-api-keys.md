# 02: Create and list API Keys

**What to build:** An operator can create a new API Key from the Web UI, providing a required Owner label and an optional expiry date, and is shown the full key (`api_<Key ID>_<secret>`) exactly once. The Web UI also lists all existing keys (Key ID, Owner, created_at, expires_at) without ever displaying the secret or its hash.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] Key generation produces `api_<8-char Key ID>_<32-char secret>` (ADR-0002); the Key ID is plaintext and unique, the secret is hashed before it touches storage or a log line.
- [ ] `POST` create-key requires an Owner; a request without one is rejected. Expiry (`expires_at`) is optional.
- [ ] The create-key response includes the full plaintext key exactly once; there is no endpoint that can ever reproduce it afterward.
- [ ] `GET` list-keys returns Key ID, Owner, created_at, and expires_at for every key, and never returns the secret or its hash.
- [ ] The Web UI has a page to create a key (form: Owner, optional expiry) that displays the one-time full key clearly, and a page/table listing existing keys.
- [ ] An HTTP-level test against the wired binary + real Postgres covers: creating a key returns the full key once, the created key then appears in the list without its secret, and creating without an Owner is rejected.
