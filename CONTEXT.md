# Istio API Key Authenticator — Context

## Glossary

- **API Key**: A credential of the form `api_<Key ID>_<secret>` issued to a consumer, used to authenticate mesh traffic via the check service. Only the hashed secret portion is stored.
- **Key ID**: The 8-character plaintext segment of an API Key. Used for lookup, shown in the Web UI, and used as the Prometheus metrics label. Not secret.
- **Owner**: The name/label attached to a key at creation time, identifying who or what it was issued to. Injected as a response header by the check service on successful auth.
- **Check Service**: This service's Envoy `ext_authz` HTTP endpoint that Istio calls (via `AuthorizationPolicy` / `extensionProviders.envoyExtAuthzHttp`) to authenticate incoming mesh traffic.
- **PoC**: The current scope of this repo. Items explicitly deferred to a future release are tracked in README.md's "Additions to Be Added on Official Release" section and are not designed for yet.

## See also

- `docs/adr/` for architectural decisions and their rationale.
