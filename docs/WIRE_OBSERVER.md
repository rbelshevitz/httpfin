# TLS/HTTP2 connection observer: design notes

> **Status: design document.** The TLS/HTTP2 connection observer is not
> implemented in the current release. There is no separate observer process,
> raw-packet capture path, standard JA3/JA4 calculation, or HTTP/2 connection
> observer.

This document defines the future TLS/HTTP2 connection observer. It separates
connection-level evidence from the request-level data collected by the current
HTTP module.

## Recommended implementation

Write the first version in Go as a separate package and command in this
repository. Use `crypto/tls` to own the direct TLS listener and capture the
available `ClientHelloInfo` fields; add a connection-aware HTTP/2 layer only
where the observer owns the decrypted connection. Keep the observer out of the
Nginx worker and join its records with httpfin observations through an explicit
connection or request ID.

## Prototype plan and Go library shortlist

The first prototype should be a local, opt-in Go listener. It should accept a
TLS connection, assign a connection ID, retain a bounded prefix of the bytes
read by `tls.Server`, and emit one versioned observation record. It must not
change the current Nginx module or require packet capture. The prototype is
useful even when parsing fails: capture limits, parser errors, and handshake
outcomes are evidence about the observation path.

| Candidate | Use in the prototype | Decision |
|---|---|---|
| Go `crypto/tls` | Direct TLS termination, `ClientHelloInfo`, negotiated TLS/ALPN | Required base; standard library |
| Go `x/net/http2` | HTTP/2 frame and SETTINGS observation where the observer owns the connection | Use only at a connection-aware boundary; do not depend on `net/http` internals |
| `github.com/psanford/tlsfingerprint` | Candidate parser/calculator for JA3 and JA4 from an incoming ClientHello | Spike behind an adapter; pin and verify against fixtures before adopting |
| `github.com/dreadl0ck/ja3` | Candidate JA3 implementation and comparison oracle | Test-only comparison initially; its packet-decoding dependency is unnecessary for the first capture path |
| `github.com/google/gopacket` | PCAP/L4 capture and packet decoding | Defer; it does not solve the terminated HTTP/2 observation problem |
| `github.com/refraction-networking/utls` | ClientHello construction and client impersonation | Do not use for the observer; useful for generating test clients |

The ready-made fingerprint packages are third-party code, not protocol
authority. Keep the observer's input model independent from them so the
project can replace a parser, compare two implementations, and retain the raw
normalized fields.

### Prototype stages

1. **Capture:** bounded `net.Conn` tee before `tls.Server`; no second reader and
   no read from `ClientHelloInfo.Conn` in the callback.
2. **Parse:** decode one complete ClientHello or report `incomplete`,
   `malformed`, or `limit_exceeded`; retain normalized cipher, extension,
   group, signature, SNI, and ALPN inputs.
3. **Fingerprint:** calculate JA3 and JA4 only when required inputs are
   complete; store algorithm/parser versions, GREASE policy, normalized input,
   and digest separately.
4. **HTTP/2:** only after ALPN selects `h2` and the observer owns the
   decrypted connection; record preface, initial SETTINGS, ACKs, and a
   stream/frame summary without header values or bodies.
5. **Join:** publish connection evidence independently, then link requests by
   trusted `connection_id` and HTTP/2 `stream_id`. Never infer the link from IP
   and time alone.

### Fixtures and acceptance criteria

The prototype is ready for review when it has offline fixtures for TLS 1.2 and
1.3, SNI and no-SNI, `h2` and `http/1.1`, GREASE, resumption,
HelloRetryRequest, truncated and oversized ClientHello messages. Generate test
traffic with the standard Go client, `curl`, and browser-like clients, but do
not use their claimed User-Agent as ground truth.

Each fixture must assert parsed fields, canonical JA3/JA4 input, the digest,
capture status, and failure state. A direct-TLS smoke test must prove that a
truncated capture does not break the handshake and that the Nginx request path
remains unchanged. Memory must be bounded per connection; raw payloads are not
stored in the normal report by default.

The first milestone stops after ClientHello capture and fingerprints. HTTP/2 is
a second milestone because standard `net/http` normally owns the connection;
frame observation needs an explicit ownership or tee boundary.

## What each observation point can know

| Observation point | TLS ClientHello | Exact JA3/JA4 | HTTP/2 connection evidence | HTTP request fields |
|---|---:|---:|---:|---:|
| Go TLS listener before `tls.Server` | raw bytes available | yes, with a parser | only if the HTTP/2 connection is instrumented | yes |
| Go `tls.ClientHelloInfo` callback | parsed fields only | not guaranteed | no | later |
| Nginx HTTP module after TLS termination | no | no | no | yes |
| Nginx `stream ssl_preread` | SNI/ALPN/version summary | no | no | no |
| L4 sidecar before TLS termination | raw bytes available | yes, with a parser | only if it also observes decrypted HTTP/2 | no plaintext headers |

The current Nginx module stays in the fourth column. It must not claim JA3/JA4
or HTTP/2 fingerprints from that request-level hook.

## Observation record schema

Every signal should carry its provenance instead of being stored as an
unqualified string. The first version of the schema should be explicit and
forward-compatible:

```json
{
  "schema_version": "observation.v1",
  "connection_id": "tls-01H...",
  "request_id": "req-01H...",
  "signal": {
    "name": "tls.ja4",
    "value": "t13d1516h2_...",
    "state": "inferred",
    "source": "go-tls-http2-observer",
    "observation_point": "before_tls_server",
    "parser_version": "clienthello/1",
    "algorithm_version": "ja4-tls/1",
    "captured_at": "2026-11-25T18:00:00Z"
  }
}
```

Use these signal states consistently:

- `declared`: supplied by the client, such as User-Agent or offered ALPN;
- `negotiated`: selected by the protocol, such as TLS version or ALPN;
- `inferred`: derived by httpfin from one or more observations;
- `unknown`: the observation was expected but insufficient or ambiguous;
- `unavailable`: this deployment point cannot observe the field.

`source` identifies the producer, `observation_point` identifies where it saw
the bytes, and parser/algorithm versions make later comparisons reproducible.
An empty value must not be interpreted as a mismatch. A parser failure should
be represented as `unknown` with a bounded error code, not as a fabricated
fingerprint.

The schema must preserve connection and request identity separately. One TLS
connection can carry many HTTP/2 requests; a request can also arrive without a
TLS/HTTP2 observer record. Joining by source IP and timestamp alone is forbidden.

## TLS details

TLS 1.3 allows ClientHello extensions in any order except where the protocol
specification imposes a specific rule (for example, `pre_shared_key` is last).
A fingerprint implementation therefore needs the original cipher and extension
lists, GREASE handling, supported groups, point formats, signature algorithms,
ALPN, SNI state, and the offered TLS versions. A parsed summary is useful
evidence but is not automatically the original wire message.

Go's `tls.ClientHelloInfo` exposes useful parsed fields such as cipher suites,
curves, signature schemes, ALPN, supported versions, and extension IDs. Its
`Conn` must not be read from or written to in the callback. Exact wire capture
therefore belongs in a wrapper around the connection before TLS parsing, or in
a separate observer.

TLS 1.3 resumption, HelloRetryRequest, ECH, and missing SNI must produce
explicit `unknown` or separate observation states. They must not be forced into
a browser mismatch. GREASE values must be recognized and handled according to
the selected fingerprint algorithm; they are not evidence of a non-browser
client by themselves.

## JA3 and JA4

JA3 and JA4 are fingerprinting conventions, not TLS protocol fields. Store the
raw normalized inputs alongside the digest so a future algorithm revision can
be reproduced and audited. Record the method version, parser version, GREASE
policy, and capture point.

JA4 has additional implementation and licensing details for JA4+ methods. The
project should begin with TLS JA4 only, record the upstream specification
version, and avoid silently presenting JA4H/JA4T or other methods as JA4. The
JA4 repository distinguishes the TLS JA4 license from the licenses for other
JA4+ methods; review the upstream license before distributing derived code.

## HTTP/2 details

HTTP/2 SETTINGS are connection-level data. A request access-log hook cannot
attribute the initial SETTINGS frame reliably to each request, especially when
many streams share one connection. The observer must key the evidence by a
connection ID and then link request IDs to that connection.

The first HTTP/2 evidence record should preserve:

- client connection preface observed or not;
- ordered initial SETTINGS pairs;
- SETTINGS ACK behavior;
- initial WINDOW_UPDATE and PRIORITY/PRIORITY_UPDATE where observed;
- HEADERS/CONTINUATION framing summary, without storing header values;
- connection reuse and stream count.

Malformed or incomplete connections must be retained as incomplete evidence,
not silently classified as a browser mismatch.

## Bounded tee and join semantics

The direct-TLS capture path is a passive bounded tee, not a second TLS reader:

1. accept the raw connection and assign `connection_id` before TLS parsing;
2. copy each byte read by `tls.Server` into a fixed-size capture buffer until
   the limit is reached;
3. always return the complete read result to the TLS parser, including bytes
   that are no longer retained after the limit;
4. record `captured_bytes`, `capture_limit`, `truncated`, `read_error` and
   `handshake_result` separately;
5. if capture or parsing fails, continue the normal TLS path when possible and
   mark the evidence incomplete;
6. never read `ClientHelloInfo.Conn` from the callback and never start an
   unbounded goroutine or queue for capture.

The limit must be configurable but hard-bounded. The default should be large
enough for ordinary ClientHello messages while a maliciously large handshake
can only consume the configured buffer. A capture limit is an evidence limit,
not a TLS message limit: TLS must still receive the original bytes.

Join rules:

- `connection_id` is created at the earliest observer and is stable for the
  lifetime of that TCP/TLS connection;
- HTTP/2 connection evidence joins to requests through `connection_id` and
  `stream_id`;
- Nginx request events use their `request_id`; a raw TLS record may be joined
  only when the deployment supplies a trusted connection or handoff ID;
- proxy hops receive distinct observation records and are never merged merely
  because the source address matches;
- missing IDs produce separate records with `join_state=unavailable`, not a
  guessed association.

## Recommended implementation order

1. Build a Go direct-TLS capture path that tees only the bounded initial bytes
   before `tls.Server` consumes them.
2. Parse and store a versioned raw ClientHello summary.
3. Calculate JA3/JA4 in a separate package with golden fixtures and parser
   version metadata.
4. Add connection IDs and an HTTP/2 observer at a connection-aware layer.
5. Keep the Nginx HTTP module request-level and join observations by an explicit
   handoff/request identifier where topology permits.

Do not add raw packet capture, eBPF, or a second Nginx module until the direct
TLS path has fixtures and measurable loss counters.

## Sources

- [RFC 8446: TLS 1.3](https://www.rfc-editor.org/rfc/rfc8446.html)
- [RFC 6066: TLS extensions, including Server Name Indication](https://www.rfc-editor.org/rfc/rfc6066.html)
- [RFC 8701: GREASE](https://www.rfc-editor.org/rfc/rfc8701.html)
- [RFC 9849: TLS Encrypted Client Hello](https://www.rfc-editor.org/rfc/rfc9849.html)
- [Go `crypto/tls.ClientHelloInfo`](https://pkg.go.dev/crypto/tls#ClientHelloInfo)
- [Go `x/net/http2` frame API](https://pkg.go.dev/golang.org/x/net/http2)
- [RFC 9113: HTTP/2](https://www.rfc-editor.org/rfc/rfc9113.html)
- [Nginx stream SSL preread module](https://nginx.org/en/docs/stream/ngx_stream_ssl_preread_module.html)
- [`psanford/tlsfingerprint`: Go JA3/JA4 candidate](https://github.com/psanford/tlsfingerprint)
- [`dreadl0ck/ja3`: Go JA3 candidate](https://github.com/dreadl0ck/ja3)
- [`google/gopacket`: packet and PCAP decoding](https://github.com/google/gopacket)
- [`refraction-networking/utls`: client-side TLS fingerprint construction](https://github.com/refraction-networking/utls)
- [JA4 technical details](https://github.com/FoxIO-LLC/ja4/blob/main/technical_details/JA4.md)
