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

- [ ] **Host Fail2ban:** on a dedicated test address verify four states: a log
  without a jail does not ban, the filter rejects `observe`, an enabled jail
  bans only `deny`, and `unban` removes the ban. Cover IPv4, IPv6, trusted proxy,
  and allowlist behavior.
- [ ] **Loss accounting:** agree on the minimum states `received → decoded →
  stored → dropped`, expose them through health/diagnostic output, and document
  reconciliation with the access log. Missing telemetry must not be treated as
  proof that no request occurred.
- [ ] **Reliability:** add reproducible cases for socket permission failures,
  datagram overflow, and a slow Go consumer. Nginx must continue serving in all
  three cases, with the loss reason visible to operators.

## Long-term items — do not pull into current work

- [ ] Optional SQLite history for search across restarts; never enable by default.
- [ ] Fingerprint-based blocking requires a separate TLS-aware edge proxy, wire
  observer, or direct Go TLS proxy. The current Nginx HTTP module is not enough.
- [ ] Automatic profile updates and distributed storage.

## Constraints

- The Nginx HTTP module cannot see passthrough or parse ClientHello.
- Unix datagram handoff is best-effort; lossless audit requires access logs or a
  future durable store.
- Dynamic modules must match the Nginx version and libc used at runtime.
