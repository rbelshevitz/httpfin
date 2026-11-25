# Wire observer implementation notes

This document fixes the boundary for the first diagnostic priority: raw TLS
and HTTP/2 evidence. It prevents the HTTP module from pretending that a
request-level hook can reconstruct connection-level fingerprints.

## What each observation point can know

| Observation point | TLS ClientHello | Exact JA3/JA4 | HTTP/2 connection evidence | HTTP request fields |
|---|---:|---:|---:|---:|
| Go TLS listener before `tls.Server` | raw bytes available | yes, with a parser | only if the HTTP/2 connection is instrumented | yes |
| Go `tls.ClientHelloInfo` callback | parsed fields only | not guaranteed | no | later |
| Nginx HTTP module after TLS termination | no | no | no | yes |
| Nginx `stream ssl_preread` | SNI/ALPN/version summary | no | no | no |
| L4 sidecar before TLS termination | raw bytes available | yes, with a parser | only if it also observes decrypted HTTP/2 | no plaintext headers |

The current Nginx module stays in the fourth column. It must not claim JA3/JA4
or HTTP/2 fingerprints from that hook.

## TLS details

TLS 1.3 allows ClientHello extensions in any order except where the protocol
specification imposes a specific rule. A fingerprint implementation therefore
needs the original cipher and extension lists, GREASE handling, supported
groups, point formats, signature algorithms, ALPN, SNI state, and the offered
TLS versions. A parsed summary is useful evidence but is not automatically the
original wire message.

Go's `tls.ClientHelloInfo` exposes useful parsed fields such as cipher suites,
curves, signature schemes, ALPN, supported versions, and extension IDs. Its
`Conn` must not be read from or written to in the callback. Exact wire capture
therefore belongs in a wrapper around the connection before TLS parsing, or in
a separate observer.

TLS 1.3 resumption, HelloRetryRequest, ECH, and missing SNI must produce
explicit `unknown` or separate observation states. They must not be forced into
a browser mismatch.

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
- [Go `crypto/tls.ClientHelloInfo`](https://pkg.go.dev/crypto/tls#ClientHelloInfo)
- [RFC 9113: HTTP/2](https://www.rfc-editor.org/rfc/rfc9113.html)
- [Nginx stream SSL preread module](https://nginx.org/en/docs/stream/ngx_stream_ssl_preread_module.html)
- [JA4 technical details](https://github.com/FoxIO-LLC/ja4/blob/main/technical_details/JA4.md)
