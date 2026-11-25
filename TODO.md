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

## Near-term operations work

- [ ] Test the host/systemd Fail2ban path with a controlled security event:
  verify that no jail means no ban, and that an explicitly enabled jail can ban
  and unban. This does not mean fingerprint-based blocking is implemented.
- [ ] Define loss accounting and collector health once the metrics format is
  agreed.
- [ ] Cover the remaining reliability cases: permissions, overflow, and slow
  consumers.

## Long-term architecture

- Optional SQLite history for search across restarts; never enable by default.
- Fingerprint-based blocking requires a separate TLS-aware edge proxy, wire
  observer, or direct Go TLS proxy. The current Nginx HTTP module is not enough.
- Automatic profile updates and distributed storage.

## Constraints

- The Nginx HTTP module cannot see passthrough or parse ClientHello.
- Unix datagram handoff is best-effort; lossless audit requires access logs or a
  future durable store.
- Dynamic modules must match the Nginx version and libc used at runtime.
