# httpfin roadmap

## Product boundary

`httpfin` is a diagnostic observer in front of an API. In the supported
production topology, Nginx terminates TLS and the module sends a bounded HTTP
event to Go. Go stores recent observations and produces a readable report.

The product does not authenticate clients, block traffic, or act as a MITM
proxy. It does not provide standard JA3/JA4 in the Nginx HTTP-module path.

## Done

- [x] Nginx termination and passthrough examples, dynamic-module compatibility,
  Docker build guidance, and host/systemd deployment files.
- [x] Bounded in-memory storage, browser profiles, baseline comparison, and
  secret redaction.
- [x] JSON API, UI, and readable report with clients, evidence, and fingerprint
  changes.
- [x] Docker smoke mode and host/systemd production mode.
- [x] Integration, fuzz, restart, and race-detector coverage.
- [x] Operations, QA, and non-technical documentation.

## Next release

- [ ] Add event-loss accounting: `received -> decoded -> stored -> dropped`.
  Show the result in diagnostic output and document reconciliation with the
  Nginx access log.
- [ ] Add repeatable reliability checks for socket permissions, datagram
  overflow, a stopped collector, and a slow collector. Nginx must continue
  serving requests in every case.
- [ ] Run and document a host deployment check: certificate paths, permissions,
  restart, health check, report download, and graceful shutdown.

## Separate future experiment

- [ ] Test standard JA3/JA4 in a separate direct-Go TLS listener using the
  MIT-licensed [`psanford/tlsfingerprint`](https://github.com/psanford/tlsfingerprint).
  The listener would terminate TLS, read ClientHello before the handshake, and
  pass the result into the report. This is a new input mode, not a change to
  the current Nginx HTTP module.
- [ ] Only if the direct listener is insufficient, evaluate a dedicated
  reverse proxy or a separate passive TLS program. A MITM proxy is not required
  by the supported product topology.

## Later, only if required

- [ ] Optional SQLite history across restarts, with retention, redaction,
  cleanup, disk-full handling, and recovery rules. Do not enable by default.
- [ ] Automatic profile updates and distributed storage.

## Current constraints

- The Nginx HTTP module runs after TLS termination and cannot parse the original
  ClientHello or see passthrough HTTP traffic.
- The Unix datagram is a fast, best-effort handoff. An event can be lost while
  the client request still succeeds; access logs remain the request record.
- Dynamic modules must match the Nginx version and libc used at runtime.
