# Observation field semantics

Use this reference to interpret each field in an `httpfin` observation and trace
it to its source. Transport and HTTP fields describe what the observation point
received; profile fields are heuristics based on available data.

## Observable evidence

- HTTP method, URL, status, elapsed time, and bytes are access observations;
- `User-Agent` is declared by the client and is easy to change;
- `detected_profile` is a heuristic based on available signals, usually UA;
- TLS version, negotiated/offered ALPN, SNI, and `ClientHelloSig` are transport
  signals available only at the relevant observation point;
- JA3, JA4, and HTTP/2 fingerprint fields are optional values supplied by an
  external source; built-in production paths do not calculate them;
- `observation_point` identifies the source that created the record.

Transport signals are harder to change through an ordinary HTTP header, but
they can still be produced by a client, proxy, or TLS impersonation layer.
They are evidence, not immutable identity.

Missing evidence means that the field was not available at the observation
point. It is not a mismatch and does not produce a security verdict.
