# 03: Management-API stream and shared pool mutation

**What to build:** An independent, rate-controlled stream of management-API traffic (`list`/`create`/`revoke`) that runs alongside the check stream and mutates the same shared key pool over time, so revoked and newly created keys realistically feed back into what the check stream sees.

**Blocked by:** 01

**Status:** ready-for-agent

- [ ] An independent management-API traffic stream runs on its own target rate (calls/min), configurable via its own CLI flag, decoupled from the check stream's rate.
- [ ] The stream cycles through `list`, `create`, and `revoke` calls against the management API.
- [ ] `create` calls add a newly created key into the shared pool so the check stream can subsequently draw it as valid.
- [ ] `revoke` calls pick a random active key from the shared pool, revoke it via the management API, and remove it from the pool so the check stream stops drawing it as valid from that point on.
- [ ] The pool's create/revoke state transitions are implemented as pure, unit-tested logic, with no network calls in those unit tests.
- [ ] Running the tool shows the pool's active key set changing over time as `create`/`revoke` calls land, independent of the check stream's activity.
