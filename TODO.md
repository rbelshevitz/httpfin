# httpfin roadmap

The next direction is an anti-bot gatekeeper in front of an API. Keep the
current inspector useful as a read-only diagnostic component while adding
policy in separate, explicit stages.

## Before expanding detection

- [ ] Build the Nginx module against the pinned Nginx source and run the full
  container smoke test.
- [ ] Run the pinned Docker build and full container smoke test on a host with a
  Docker daemon.
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
