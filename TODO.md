# httpfin roadmap

The next direction is an anti-bot gatekeeper in front of an API. Keep the
current inspector useful as a read-only diagnostic component while adding
policy in separate, explicit stages.

## Immediate priorities

1. **Wire observer:** capture raw ClientHello and HTTP/2 connection evidence;
   calculate real JA3/JA4 where the observation point permits it, or preserve a
   precise raw summary when it does not.
2. **Durable storage and loss metrics:** retain observations across restarts
   and expose received, dropped, backpressure, and ingest-health counters.
3. **Explainable reports:** show which fields disagree, where each field was
   observed, and why a comparison is `matched`, `mismatch`, or `unknown`.

Anti-bot policy remains downstream of these diagnostic capabilities. It must not
become the primary purpose of the project.

## Delivery order and dependencies

The first release target is diagnostic transparency, not automated blocking:

1. **Observation quality:** wire observer, versioned observation schema and
   endpoint-specific baselines.
2. **Reliability:** bounded decoding, fuzzing, load tests, loss counters and
   optional persistence.
3. **Operator experience:** explainable reports, Prometheus metrics, health
   signals and replay/diff tooling.
4. **Policy integrations:** shadow mode first; challenge, throttling and
   Fail2ban remain external, explicit and fail-open by default.

Do not implement report explanations, browser mismatch decisions or policy
actions before every signal has an observation source and a state of
`declared`, `negotiated`, `inferred`, `unknown` or `unavailable`.

## Before expanding detection

- [x] Build the Nginx module against the pinned Nginx source and run the full
  container smoke test.
- [ ] Add fuzz tests for the C JSON builder and Go event decoder.
- [ ] Add load tests for TLS handshakes, HTTP/2 requests and Unix datagrams.
- [ ] Add fuzz tests for malformed, truncated and oversized ClientHello,
  HTTP/2 frames, JSON events and report fields; assert bounded memory and no
  panic in the Go decoder or C module.
- [ ] Add loss-injection tests for full Unix datagram buffers, slow consumers,
  Go restarts and socket permission failures.

## Anti-bot observation layer

- [ ] Define a versioned observation schema with separate namespaces for
  `declared`, `negotiated`, `inferred`, `unknown` and `unavailable` fields.
  Include source, observation point, parser version, capture time and
  connection/request IDs.
- [x] Add a first baseline-file loader for Go-terminated TLS; keep unknown
  signatures explicit and never convert a baseline mismatch directly into a
  ban.
- [ ] Add a wire-observer integration point for raw ClientHello, JA3/JA4 and
  HTTP/2 connection signals; keep unavailable fields explicit.
- [ ] Track source, observation point, proxy hop and baseline version for every
  signal.
- [ ] Add privacy controls: field allowlist, redaction, retention period and
  access audit.
- [ ] Add endpoint-specific baselines instead of one global browser profile;
  define matching precedence for method, normalized route, host, API version
  and client/partner identity.
- [ ] Make baseline comparison explainable: show the expected field, observed
  field, comparison rule, baseline version and whether the result is drift,
  missing evidence or a real mismatch.

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

## Baseline updater safety

### Minimal optional updater MVP

- [ ] Add one optional capture command that writes only `candidate.json`.
- [ ] Use an external `curl-impersonate` profile as the first traffic source;
  do not add browsers, Playwright, Node.js, or browser downloads to the core
  Go image.
- [ ] Add separate `validate` and `diff` operations before considering a
  candidate for activation.
- [ ] Keep activation as an explicit operator step; the updater must never
  replace `active.json` or emit Fail2ban `decision=deny` automatically.
- [ ] Start with one Chrome-like fixture and add other browser sources only
  when a real team scenario needs them.
- [ ] Keep the updater outside the `httpfin` runtime and make its failure
  harmless to collection, reports, and API traffic.

- [ ] Keep `candidate` and `active` baselines in separate files; the updater
  must never overwrite the active file directly.
- [ ] Require schema validation, a minimum number of repeated captures, and a
  human-reviewed diff before activation.
- [ ] Store browser version, OS, runner image digest, capture time, source, and
  tool version with every baseline entry.
- [ ] Reject sudden large changes in TLS/HTTP2 signatures unless an operator
  explicitly approves the change.
- [ ] Keep the previous active baseline for immediate rollback and write an
  audit record for every activation.
- [ ] Treat unknown or stale baselines as `unknown`/`observe`, never as `deny`.
- [ ] Run browser capture in an isolated, pinned CI environment without
  production credentials, cookies, or private network access.
- [ ] Verify that the updater cannot inject arbitrary IPs, reason codes, or
  Fail2ban `decision=deny` events.

## Operator experience

- [ ] Add a report section that explains each discrepancy in plain language,
  for example: “User-Agent declares Chrome, but the observed transport
  summary is generic; this is a signal, not proof of automation.”
- [ ] Show the observation point and evidence state beside every important
  value; never render an absent value as a negative match.
- [ ] Add report exports in JSON, Squid-style text, Markdown and HTML.
- [ ] Add report comparison by request/connection ID and a `differences-only`
  mode for client, Nginx, OpenSSL and SDK regression checks.
- [ ] Add OpenAPI endpoints for observations, baselines, policy previews and
  decision traces.
- [ ] Add Prometheus metrics for received, decoded, persisted, dropped,
  backpressure, parse failures, incomplete observations, observer availability,
  report latency and policy action counts.
- [ ] Add health checks for ingest lag and loss rate, with documented alert
  thresholds and a separate readiness state for optional persistence.
- [ ] Add a canary rollout guide and an emergency bypass procedure.

## Storage and retention

- [ ] Keep the bounded in-memory ring buffer as the default low-dependency mode.
- [ ] Add optional durable export with a small first backend (SQLite or JSONL);
  preserve the same versioned event schema and make write failures visible.
- [ ] Define retention, redaction, rotation, disk-full behavior and recovery
  procedures before enabling durable storage by default.
- [ ] Keep ClickHouse, remote queues and distributed storage as adapters after
  the local persistence path and loss accounting are proven.
