# httpfin roadmap

The current product is a diagnostic observer. Its purpose is a readable report
about HTTP/TLS paths and client fingerprint changes. It does not enforce access
rules and does not block clients.

## Done

- [x] Production Nginx configurations, TLS termination, stream `ssl_preread`,
  module compatibility, and Docker build guidance.
- [x] Bounded in-memory observation store and browser profiles for desktop,
  mobile, and WebView clients.
- [x] Readable operator report with clients, fingerprint changes, evidence
  state, observation point, baseline comparison, and secret redaction.
- [x] Separate Docker smoke mode and host/systemd operation mode.
- [x] Integration, fuzz, restart, and race-detector coverage.
- [x] Operations and non-technical QA documentation.

## Near-term operations work

- [ ] Loss accounting: agree on `received -> decoded -> stored -> dropped`,
  expose it in diagnostic output, and document reconciliation with access logs.
- [ ] Reliability checks for socket permission failures, datagram overflow, and
  a slow Go consumer. Nginx must continue serving in each case.
- [ ] Host deployment smoke test covering permissions, restart, health checks,
  report download, and graceful shutdown.

## Long-term items

- [ ] Optional SQLite history for searching observations across restarts. Do not
  enable it by default.
- [ ] A separate wire observer or TLS-aware proxy for raw ClientHello, JA3/JA4,
  and connection-level HTTP/2 data. This is a new deployment component, not an
  extension of the current HTTP module.
- [ ] Automatic profile updates and distributed storage.

## Constraints

- The Nginx HTTP module cannot see passthrough traffic or parse ClientHello.
- Unix datagram handoff is best effort; complete audit requires access logs or a
  future durable store.
- Dynamic modules must match the Nginx version and libc used at runtime.
