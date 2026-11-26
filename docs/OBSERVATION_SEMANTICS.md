# Observation field semantics

This document defines what each field in an `httpfin` observation means and
where it comes from. These fields are observation signals, not proof of client
identity.

## Risk

In the current version, `Risk` is not a calculated threat score and is not a
blocking decision.

Built-in sources write a technical source label:

| Source | Current value |
| --- | --- |
| `go-http` | `Info` |
| `go-tls` | `Low` |
| `nginx-http-module` | `Info` |

`Review` and `High` may be supplied by an external collector. httpfin does not
validate or explain an external risk label.

For that reason, `Risk` is hidden from the status page and is not used for
conclusions, filtering, or alerts. It remains in raw JSON for compatibility
with external sources.

## Baseline

`Baseline` is not a general client rating. It is the result of a limited
comparison between an observed `ClientHelloSig` and a configured baseline file.

The comparison is possible only when all of the following are true:

1. a baseline is loaded through `httpfin_baseline_file`;
2. the observation point provides `ClientHelloSig`;
3. the signature is compared with a profile's `client_hello_signatures`.

Results:

- `matched` — the signature was found and its profile agrees with the detected
  client profile;
- `mismatch` — the signature was found, but the profiles disagree;
- `unknown` — `ClientHelloSig` exists but is not present in the baseline;
- `—` — the baseline is disabled or ClientHello is unavailable.

Baseline currently compares only `ClientHelloSig`. The
`HTTP2Fingerprints` field in the baseline configuration is not used for
comparison yet.

Baseline does not prove that a client is safe or malicious. It is a reason to
open the record and compare the UA, profile, TLS signals, observation point,
and request.

## Observable evidence

- HTTP method, URL, status, elapsed time, and bytes are access observations;
- `User-Agent` is declared by the client and is easy to change;
- `detected_profile` is a heuristic based on available signals, usually UA;
- TLS version, negotiated/offered ALPN, SNI, and `ClientHelloSig` are transport
  signals available only at the relevant observation point;
- `JA3`, `JA4`, and `HTTP2Fingerprint` are model fields, but the built-in
  production paths do not currently compute them;
- `observation_point` identifies the source that created the record.

Transport signals are harder to change through an ordinary HTTP header, but
they can still be produced by a client, proxy, or TLS impersonation layer.
They are evidence, not immutable identity.
