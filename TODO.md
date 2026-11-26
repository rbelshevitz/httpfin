# httpfin roadmap after 0.2

The main 0.2 path is complete: observation collection, JSON API, text report,
HTML status page, TLS/HTTP modes, Docker smoke path, and documentation.

## Next priorities

### High

- [ ] Load-test the Nginx module and Unix-datagram handoff: throughput,
  datagram overflow, stopped or slow collector, recovery, and request latency.
  The container smoke already covers bounded load, a paused collector, and
  storage recovery; full stress and overflow characterization remain.
- [ ] Add semantic comparison for two saved reports. Compare normalized
  observations, not rendered HTML/TXT: clients, User-Agent, profile, TLS,
  ALPN, SNI, status, timing, bytes, and observation point. Cover `go-http`,
  `go-tls`, `nginx-http-module`, and absent fields with report-contract tests.

### Medium

- [ ] Expose collector metrics for Prometheus: `received`, `decoded`,
  `stored`, `dropped`, handoff errors, store size, uptime, and ingest health.

## Deliberately deferred

SQLite or another durable history store, automatic profile updates, and
distributed storage stay out of the near-term plan. Reopen them only when a
concrete operational requirement appears.

## Current limits

- The Nginx HTTP module runs after TLS termination and cannot see the original
  ClientHello or HTTP inside TLS passthrough.
- Unix datagrams are best-effort; an event may be lost while the client request
  still succeeds.
- User-Agent, profiles, and fingerprints are spoofable signals, not identity.
- `Risk` is a source label, not a calculated threat score.
- `Baseline` is a limited `ClientHelloSig` comparison, not a client verdict.
- Dynamic Nginx modules must match the runtime Nginx version and libc.
