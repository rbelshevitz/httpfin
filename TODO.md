# httpfin roadmap after 0.2

The main 0.2 path is complete: observation collection, JSON API, text report,
HTML status page, TLS/HTTP modes, Docker smoke path, and documentation.

## Next priorities

### High

- [ ] Load-test the Nginx module and Unix-datagram handoff: throughput,
  datagram overflow, stopped or slow collector, recovery, and request latency.
  The container smoke already covers bounded load, a paused collector, and
  storage recovery; full stress and overflow characterization remain.
- [ ] Add upstream context to Nginx events: upstream address and status,
  connect/header/response timing, request size, and retry information when the
  corresponding Nginx variables are available.
- [ ] Add semantic comparison for two saved reports. Compare normalized
  observations, not rendered HTML/TXT: client, User-Agent, profile, TLS, ALPN,
  SNI, route, status, timing, bytes, upstream context, and observation point.
  Cover `go-http`, `go-tls`, `nginx-http-module`, and absent fields with
  report-contract tests.
- [ ] Add a small command or API operation that compares two exported JSON
  snapshots and returns a non-zero result when selected fields differ.

### Medium

- [ ] Expose collector metrics for Prometheus: `received`, `decoded`,
  `stored`, `dropped`, handoff errors, store size, uptime, and ingest health.
- [ ] Capture `traceparent` and `tracestate` as bounded correlation fields;
  never treat them as secrets or use them as metric labels.
- [ ] Add an explicit allowlist and redaction policy for selected request and
  response headers. Keep body capture disabled by default and enforce value
  size limits before storage or export.
- [ ] Add report filters for time range, route, status class, client source,
  observation point, and correlation ID.

## Deliberately deferred

SQLite or another durable history store, automatic profile updates, body
capture, distributed storage, WAF decisions, anti-bot enforcement, raw TLS
capture, and connection-level HTTP/2 fingerprinting stay out of the near-term
plan. Reopen them only when a concrete operational requirement appears.

## Current limits

- The Nginx HTTP module runs after TLS termination and cannot see the original
  ClientHello or HTTP inside TLS passthrough.
- Unix datagrams are best-effort; an event may be lost while the client request
  still succeeds.
- User-Agent and profiles are spoofable signals, not identity. JA3/JA4 and
  HTTP/2 fields, when supplied by an external source, have the same limitation.
- Dynamic Nginx modules must match the runtime Nginx version and libc.
