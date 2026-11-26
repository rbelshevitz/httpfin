# httpfin roadmap after 0.2

The main 0.2 path is complete: observation collection, JSON API, text report,
HTML status page, TLS/HTTP modes, Docker smoke path, and documentation.

## Open

- Add report-contract tests for `go-http`, `go-tls`, `nginx-http-module`, and
  absent fields.

## Deferred

Only return to these items when there is a concrete operational requirement:

- stress testing Unix-datagram overflow and a slow or stopped collector;
- a separate wire observer for raw ClientHello, standard JA3/JA4, and
  connection-level HTTP/2 evidence;
- `HTTP2Fingerprints` matching in baseline configuration;
- SQLite or another durable history store;
- automatic profile updates and distributed storage.

## Current limits

- The Nginx HTTP module runs after TLS termination and cannot see the original
  ClientHello or HTTP inside TLS passthrough.
- Unix datagrams are best-effort; an event may be lost while the client request
  still succeeds.
- User-Agent, profiles, and fingerprints are spoofable signals, not identity.
- `Risk` is a source label, not a calculated threat score.
- `Baseline` is a limited `ClientHelloSig` comparison, not a client verdict.
- Dynamic Nginx modules must match the runtime Nginx version and libc.
