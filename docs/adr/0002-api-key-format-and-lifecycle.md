# 0002. API key format and lifecycle

Date: 2026-09-28

Status: Accepted

## Context

Keys need a format that supports fast lookup, secure storage, and per-key usage tracking, without building out scoping or rotation the PoC doesn't need yet.

## Decision

- Keys are generated as `api_<8-char Key ID>_<32-char secret>`.
- The **Key ID** is plaintext: used for lookup, shown in the Web UI, and used as the Prometheus metrics label. It replaces the need for any separately stored "last N characters" column.
- The **secret** portion is hashed before storage and never stored or logged in plaintext.
- Keys are global/mesh-wide for the PoC — no per-service/per-route scoping (tracked as a future addition in README.md).
- Keys support an optional, nullable `expires_at`. The check service denies requests made with an expired key.

## Consequences

- No masked/partial-key column is needed for UI identification — the Key ID already serves that purpose.
- Scoping and full auto-rotation strategies remain future work, not designed for here.
