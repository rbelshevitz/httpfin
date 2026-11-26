# httpfin operating modes: use cases, TLS, and certificates

Choose the observation point before deploying `httpfin`.

| Mode | TLS terminator | Certificate for httpfin | Visible data | Best for |
|---|---|---:|---|---|
| Plain HTTP API debugging | none | no | HTTP request/response | SDK and API debugging |
| Direct HTTPS listener | Go/httpfin | yes | TLS, SNI, ALPN, HTTP | direct TLS diagnostics |
| Nginx TLS termination | Nginx | no | Nginx TLS fields + HTTP handoff | production ingress |
| Nginx TLS passthrough | downstream service | no for Nginx | SNI, offered ALPN, advertised TLS version | routing diagnostics |

## Architecture and boundaries

```text
Client → Nginx → application
           └─ httpfin module → Unix datagram → Go collector → API/UI/report
```

Nginx continues serving requests if the collector is unavailable. The module
sends a bounded, non-blocking event after request processing; it does not wait
for Go or perform HTTP, DNS, database, or fingerprint work in the worker. Go
normalizes the event, stores a bounded in-memory history, and serves the UI and
reports. Runtime observations are lost on restart.

The Nginx event contains request ID, method, URI, Host, User-Agent, source
address, status, request time, response size, protocol, TLS version, SNI, and
ALPN when available. It does not contain the original ClientHello after TLS
termination, so standard JA3/JA4 and connection-level HTTP/2 fingerprints are
not calculated by this path.

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
only by the service account. The current listener already receives selected
ClientHello fields through Go's TLS callback and records a local signature, but
it does not calculate standard JA3/JA4. That is a separate TODO item based on a
TLS-aware listener.

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

The bundled Docker Nginx configuration reads these paths:

```yaml
services:
  httpfin:
    volumes:
      - /etc/letsencrypt/live/api.example.test/fullchain.pem:/etc/nginx/certs/server.crt:ro
      - /etc/letsencrypt/live/api.example.test/privkey.pem:/etc/nginx/certs/server.key:ro
```

For a host-installed Nginx, use the same idea with paths such as
`/etc/nginx/tls/api.example.test.fullchain.pem` and
`/etc/nginx/tls/api.example.test.key`, and reference them from
`ssl_certificate` and `ssl_certificate_key`.

For a local smoke test, create a short-lived self-signed certificate and map
the example hostname to localhost:

```bash
sudo install -d -m 0750 /etc/nginx/tls
sudo openssl req -x509 -nodes -newkey rsa:2048 -days 7 \
  -keyout /etc/nginx/tls/api.example.test.key \
  -out /etc/nginx/tls/api.example.test.fullchain.pem \
  -subj '/CN=api.example.test' \
  -addext 'subjectAltName=DNS:api.example.test'
echo '127.0.0.1 api.example.test' | sudo tee -a /etc/hosts
sudo nginx -t && sudo systemctl reload nginx
```

Use `curl -k` only for this local certificate. For a real public hostname,
obtain a trusted certificate, for example with Certbot:

```bash
sudo certbot --nginx -d api.example.com
```

Certbot normally keeps the certificate and private key under
`/etc/letsencrypt/live/api.example.com/`. Never use a self-signed certificate
in production, and keep the private key readable only by Nginx.

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

## Observation storage

The default runtime keeps a bounded number of recent observations in memory and
relies on the access log for detailed request history. This keeps the collector
simple and bounded without making reports depend on a database or disk state.

Runtime observations are lost on restart. Save `/api/report` and retain access
logs before restarting when an incident needs to be investigated.

SQLite is not required or enabled by default. It is reserved for a future
durable-history mode that must define retention, redaction, cleanup, disk-full
behavior, and recovery before it is considered production-ready.

## Practical team workflows

- DevOps/SRE: compare direct and ingress requests after changes to TLS,
  HTTP/2, real IP, proxy protocol, or upstream routing.
- API QA: keep exported reports for curl, curl-impersonate, SDKs, and browsers;
  compare transport behavior as well as response bodies.
- Security: use available request and transport fields as triage signals and
  combine them with rate, authentication, IP reputation, and application logs.
- Incident response: preserve `/api/report`, `/api/ingest`, access logs, Nginx version, and
  OpenSSL version together so a transport change can be reproduced.

The Russian version is in [`OPERATING_MODES.ru.md`](OPERATING_MODES.ru.md).
For operational checks, use [`DEVOPS_RUNBOOK.md`](DEVOPS_RUNBOOK.md).

## Protocol sources

- [RFC 9846 — TLS 1.3](https://www.rfc-editor.org/rfc/rfc9846) and historical
  [RFC 8446](https://www.rfc-editor.org/rfc/rfc8446);
- [RFC 6066 — SNI](https://www.rfc-editor.org/rfc/rfc6066);
- [RFC 7301 — ALPN](https://www.rfc-editor.org/rfc/rfc7301);
- [RFC 9113 — HTTP/2](https://www.rfc-editor.org/rfc/rfc9113);
- [RFC 9110 — HTTP semantics](https://www.rfc-editor.org/rfc/rfc9110);
- [RFC 7239 — Forwarded](https://www.rfc-editor.org/rfc/rfc7239);
- [RFC 9849 — Encrypted ClientHello](https://www.rfc-editor.org/rfc/rfc9849).
