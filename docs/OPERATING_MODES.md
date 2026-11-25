# httpfin operating modes: use cases, TLS, and certificates

Choose the observation point before deploying `httpfin`.

| Mode | TLS terminator | Certificate for httpfin | Visible data | Best for |
|---|---|---:|---|---|
| Plain HTTP API debugging | none | no | HTTP request/response | SDK and API debugging |
| Direct HTTPS listener | Go/httpfin | yes | TLS, SNI, ALPN, HTTP | ClientHello experiments |
| Nginx TLS termination | Nginx | no | Nginx TLS fields + HTTP handoff | production ingress |
| Nginx TLS passthrough | downstream service | no for Nginx | SNI, offered ALPN, advertised TLS version | routing diagnostics |

## Direct HTTPS

The Go listener needs a certificate and private key because it terminates TLS:

```bash
httpfin_tls_cert=/etc/httpfin/tls/server.crt \
httpfin_tls_key=/etc/httpfin/tls/server.key \
httpfin_tls_addr=:8443 \
go run ./cmd/httpfin
```

Use a self-signed certificate for local tests and a certificate for the real
hostname in shared or production environments. Keep the private key readable
only by the service account.

## Nginx termination

This is the recommended production topology:

```text
client --TLS--> Nginx --HTTP/proxy--> application
                   ├─ certificate/key
                   ├─ SNI/ALPN/TLS access log
                   └─ httpfin module -> Unix datagram -> Go collector
```

The certificate belongs to Nginx; the Go collector does not need one. Mount
the certificate and key into the Nginx container as a secret or read-only
volume. The Docker example generates a short-lived self-signed certificate
only for its `inspector.example.test` smoke test.

After termination, the original ClientHello is not forwarded to the upstream.
Use Nginx TLS variables for operational diagnostics. Exact JA3/JA4 requires
capturing ClientHello before termination.

## TLS passthrough

With `stream { ssl_preread on; }`, Nginx does not need a certificate. The
downstream TLS service owns the certificate. This exposes SNI, the ALPN list,
and the TLS version advertised in ClientHello, but not the negotiated TLS
version or HTTP method, URL, headers, status, or User-Agent.

`ssl_preread` observes ClientHello before TLS termination. Its protocol value
is therefore a client offer, not proof of the version eventually negotiated by
the downstream service.

## Practical team workflows

- DevOps/SRE: compare direct and ingress requests after changes to TLS,
  HTTP/2, real IP, proxy protocol, or upstream routing.
- API QA: keep baselines for curl, curl-impersonate, SDKs, and browsers; compare
  transport behavior as well as response bodies.
- Security: use fingerprints as triage signals and combine them with rate,
  authentication, IP reputation, and application logs.
- Incident response: preserve `/api/report`, access logs, Nginx version, and
  OpenSSL version together so a transport change can be reproduced.

The complete Russian runbook is in [`OPERATING_MODES.ru.md`](OPERATING_MODES.ru.md).
The architecture diagrams are in [`ARCHITECTURE.md`](ARCHITECTURE.md).

## Protocol sources

- [RFC 9846 — TLS 1.3](https://www.rfc-editor.org/rfc/rfc9846) and historical
  [RFC 8446](https://www.rfc-editor.org/rfc/rfc8446);
- [RFC 6066 — SNI](https://www.rfc-editor.org/rfc/rfc6066);
- [RFC 7301 — ALPN](https://www.rfc-editor.org/rfc/rfc7301);
- [RFC 9113 — HTTP/2](https://www.rfc-editor.org/rfc/rfc9113);
- [RFC 9110 — HTTP semantics](https://www.rfc-editor.org/rfc/rfc9110);
- [RFC 7239 — Forwarded](https://www.rfc-editor.org/rfc/rfc7239);
- [RFC 9849 — Encrypted ClientHello](https://www.rfc-editor.org/rfc/rfc9849).
