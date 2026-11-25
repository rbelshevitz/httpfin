# httpfin roadmap

The next direction is an anti-bot gatekeeper in front of an API. Keep the
current inspector useful as a read-only diagnostic component while adding
policy in separate, explicit stages.

## Before expanding detection

- [x] Build the Nginx module against the pinned Nginx source and run the full
  container smoke test.
- [ ] Add fuzz tests for the C JSON builder and Go event decoder.
- [ ] Add load tests for TLS handshakes, HTTP/2 requests and Unix datagrams.

## Anti-bot observation layer

- [ ] Define a versioned observation schema for declared, negotiated and
  inferred fields.
- [ ] Add a wire-observer integration point for raw ClientHello, JA3/JA4 and
  HTTP/2 connection signals; keep unavailable fields explicit.
- [ ] Track source, observation point, proxy hop and baseline version for every
  signal.
- [ ] Add privacy controls: field allowlist, redaction, retention period and
  access audit.
- [ ] Add endpoint-specific baselines instead of one global browser profile.

## Policy layer: API gatekeeper

- [ ] Add a separate policy service/module that consumes observations and
  application context; do not put a rule engine in the Nginx worker.
- [ ] Start in **observe-only** mode: produce reasons and a decision trace,
  never change the request.
- [x] Add the first two-signal anti-bot evaluator with `decision=observe`; keep
  `decision=deny` unavailable until wire-level baselines and replay tests exist.
- [ ] Add **shadow mode**: calculate challenge/throttle/allow decisions and
  compare them with operator outcomes.
- [ ] Define explainable actions: `allow`, `review`, `challenge`, `throttle`,
  and `deny`, with reason codes and expiry.
- [ ] Support fail-open and fail-closed per endpoint and per action, with a
  safe default for collector outages.
- [ ] Add a signed, authenticated handoff from the edge to the API so an
  application never trusts an arbitrary client-supplied fingerprint header.
- [ ] Integrate with a WAF, API gateway or service mesh only after shadow-mode
  results are reviewed.

## Fail2ban integration

Design note: Fail2ban should consume a dedicated security log or journal
stream, not scrape `/api/report` or the observation JSON. `httpfin` should
produce evidence; Fail2ban should perform thresholding and firewall actions on
the host. See [`docs/FAIL2BAN.md`](docs/FAIL2BAN.md).

- [x] Define a stable, line-oriented security event format for Fail2ban
  ingestion: timestamp, source IP, endpoint, reason code, confidence, and
  correlation ID; never put raw URLs, tokens, cookies, or full User-Agent data
  into the ban log.
- [x] Add an opt-in security log or exporter separate from the human report and
  JSON observation API.
- [x] Provide a sample `jail.local` and filter for repeated, explainable
  failures such as rate-limit violations, authentication abuse, malformed
  requests, or a policy decision from the future gatekeeper.
- [ ] Support `dry-run`/observe-only mode and an explicit ban threshold before
  any firewall action is enabled.
- [ ] Document trusted proxy configuration so Fail2ban bans the real client IP
  only when the proxy chain is authenticated and configured correctly.
- [ ] Test IPv4, IPv6, NAT, shared corporate egress, CDN addresses, container
  networks, and firewall backends such as nftables and iptables.
- [ ] Make ban duration, recovery, allowlists, ignore rules, and emergency
  unban procedures explicit; keep the default action fail-safe for API
  availability.
- [ ] Do not ban solely on User-Agent, JA3/JA4, or a heuristic browser profile;
  combine repeated behavior with rate, authentication, session, and endpoint
  context.
- [ ] Add integration tests that verify a malicious client can trigger a
  temporary ban without allowing an attacker to inject an arbitrary source IP
  or reason into the Fail2ban event stream.

References: [Fail2ban jail configuration](https://github.com/fail2ban/fail2ban/blob/master/config/jail.conf),
[filter development and test guidance](https://github.com/fail2ban/fail2ban/blob/master/FILTERS),
[reverse-proxy client IP guidance](https://fail2ban.dev/troubleshooting/reverse-proxy-ip/),
and [Nginx-specific Fail2ban examples](https://fail2ban.dev/guides/nginx/).

## Detection quality

- [ ] Measure false-positive and false-negative rates by endpoint and client
  version.
- [ ] Test browser updates, SDK updates, proxies, NAT, mobile networks, ECH,
  TLS resumption and HTTP/2 connection reuse.
- [ ] Treat fingerprints as correlation signals; combine them with rate,
  authentication, session and application behavior.
- [ ] Create a replay fixture set from ordinary browsers, SDKs, automation and
  known-good partner clients.

## Operator experience

- [ ] Add a report section that explains each decision reason in plain language.
- [ ] Add OpenAPI endpoints for observations, baselines, policy previews and
  decision traces.
- [ ] Add metrics for event loss, observer availability, policy latency and
  action counts.
- [ ] Add a canary rollout guide and an emergency bypass procedure.
