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

## Long-term tasks

- [ ] Optional SQLite history for searching observations across restarts. Do not
  enable it by default.
- [ ] A separate wire observer or TLS-aware proxy for raw ClientHello, JA3/JA4,
  and connection-level HTTP/2 data. This is a new deployment component, not an
  extension of the current HTTP module.
- [ ] TLS fingerprinting PoC under MIT: evaluate
  [`psanford/tlsfingerprint`](https://github.com/psanford/tlsfingerprint). It can
  wrap a Go `net.Listener`, read ClientHello before `tls.Server`, and provide
  JA3/JA4 to the same request context. First use it in the direct Go HTTPS mode;
  do not put it in front of the Nginx production path until certificate,
  latency, HTTP/2, SNI, timeout and restart tests pass.
- [ ] Proxy alternative under MIT: evaluate
  [`mitmproxy`](https://github.com/mitmproxy/mitmproxy) only in a controlled
  test network. Its transparent or reverse modes terminate TLS and generate
  interception certificates, so clients must trust its CA. It is useful for a
  lab or QA path, not a drop-in production edge for arbitrary public clients.
- [ ] Do not select `elazarl/goproxy` as the production edge without a separate
  maintenance and protocol review. It is an MIT HTTP proxy, but its age and
  scope make it a fallback prototype rather than the default choice.
- [ ] Automatic profile updates and distributed storage.

## Current constraints

- The HTTP module works only after TLS termination. It does not see passthrough
  traffic and cannot parse the original ClientHello.
- The Unix datagram is a fast, best-effort handoff. If the collector is stopped
  or overloaded, an event can be lost while the client request still succeeds.
- Access logs are needed when every request must be accounted for. They provide
  request records, while a future durable store would retain httpfin details.
- Dynamic modules must match the Nginx version and libc used at runtime.
