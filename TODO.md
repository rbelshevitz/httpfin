# httpfin roadmap

The goal is a readable evidence report for HTTP/TLS request paths. Fingerprints
and anti-bot signals are supporting evidence, never the sole blocking input.

## Done

- [x] Production Nginx configs, TLS termination, `stream ssl_preread`, module
  compatibility, and Docker build guidance.
- [x] Bounded in-memory observation store.
- [x] Desktop, mobile, and WebView browser profiles.
- [x] Operator report with clients, fingerprint changes, bot signal, evidence
  state, observation point, and secret redaction.
- [x] Observe-only operations, emergency bypass, and Fail2ban examples.
- [x] Separate Docker smoke mode and host/systemd operations mode.
- [x] Nginx events pass through profiling, baseline, policy, and the security log.
- [x] Integration, fuzz, restart, and race-detector coverage.
- [x] Operations and non-technical QA documentation.

## Remaining for the current runtime

- [ ] Complete the optional ban path: add a separately reviewed `deny` rule and
  an end-to-end Fail2ban test. Keep observe-only as the default.
- [ ] Define loss accounting and collector health once the metrics format is
  agreed.
- [ ] Cover the remaining reliability cases: permissions, overflow, and slow
  consumers.

## Deferred

- Optional SQLite history for search across restarts; never enable by default.
- A separate wire observer for raw ClientHello, real JA3/JA4, and HTTP/2
  connection evidence.
- Automatic profile updates and distributed storage.

## Constraints

- The Nginx HTTP module cannot see passthrough or parse ClientHello.
- Unix datagram handoff is best-effort; lossless audit requires access logs or a
  future durable store.
- Dynamic modules must match the Nginx version and libc used at runtime.
