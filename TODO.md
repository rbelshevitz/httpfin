# httpfin roadmap after 0.2

Version 0.2 covers the main working path: observation collection, JSON API,
text report, HTML status page, TLS/HTTP modes, and documentation. The remaining
work is limited to three focused accuracy and reliability improvements.

## Product boundary

`httpfin` is an observation tool in front of an API. It reports only fields
actually collected at the selected observation point. It does not authenticate
clients, identify people, block traffic, or act as a MITM proxy.

Supported observation points:

- `go-http`: HTTP middleware; no ClientHello;
- `go-tls`: direct Go TLS listener with TLS version, SNI, ALPN, and custom
  `ClientHelloSig`;
- `nginx-http-module`: limited post-termination HTTP event over Unix datagram.

Standard JA3/JA4 and HTTP/2 fingerprints are not computed by the built-in
collection paths. Empty means not observed at that point.

## Done in 0.2

- [x] Bounded in-memory storage of recent observations.
- [x] JSON API, text report, and strict HTML status page from the same
  `model.Connection` records.
- [x] Live mode using local jQuery only for state refresh.
- [x] Access-oriented table with time, IP, method, path, HTTP status, latency,
  profile, TLS/ALPN, SNI, UA, and detection point.
- [x] Per-record details for Client, TLS, available fingerprints, and baseline.
- [x] Baseline comparison only when `ClientHelloSig` is available.
- [x] Non-scoring `Risk` hidden from the status page and retained only in the
  raw model for external-source compatibility.
- [x] Missing fields are not replaced with synthetic evidence.
- [x] Host timezone display through `time.Local`; stored timestamps remain UTC.
- [x] Demo/smoke records explicitly marked with `observation_point=demo`.

## Next focused improvements

- [x] Extend the Nginx event contract with host, request time, protocol, and
  response size; Go normalizes them into `elapsed_ms`, `bytes`, and `protocol`.
- [x] Add `received -> decoded -> stored -> dropped` accounting and expose it
  through `/api/ingest`.
- [ ] Add report-contract tests for all observation points and absent fields.

## Deferred

Unix-datagram stress testing, a dedicated wire observer for standard JA3/JA4,
`HTTP2Fingerprints` baseline matching, SQLite history, profile updates, and
distributed storage are deliberately outside the current release. Reopen them
only when there is a concrete operational requirement.

## Constraints

- The Nginx HTTP module runs after TLS termination and cannot see the original
  ClientHello or HTTP inside TLS passthrough.
- Unix datagrams are best-effort; an event may be lost while the client request
  still succeeds.
- User-Agent, profiles, and fingerprints are spoofable signals, not identity.
- `Risk` is a source label, not a calculated threat score.
- `Baseline` is a limited `ClientHelloSig` comparison, not a client verdict.
- Dynamic Nginx modules must match the runtime Nginx version and libc.
