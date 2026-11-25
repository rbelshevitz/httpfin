# httpfin: Client Fingerprint Inspector

[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![Nginx](https://img.shields.io/badge/Nginx-1.28.3-009639?logo=nginx&logoColor=white)](Dockerfile)
[![Alpine](https://img.shields.io/badge/Alpine-3.23-0D597F?logo=alpinelinux&logoColor=white)](Dockerfile)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)](docker-compose.yml)
[![CI](https://github.com/rbelshevitz/httpfin/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/rbelshevitz/httpfin/actions/workflows/ci.yml)

> **Observe the client. Explain the path.**

`httpfin` is a Go diagnostic service for incoming HTTP clients. It collects
request, TLS/SNI/ALPN and User-Agent data available at the observation point,
then produces a JSON API, UI and Squid-style report.

It is an evidence tool, not an authentication system and it does not block
requests.
`curl-impersonate` is used only as a reference traffic generator; its profiles
can become outdated. The `curl-impersonate` project is maintained under the GitHub account
[`lwthiker`](https://github.com/lwthiker).

## Quick start

```bash
go run ./cmd/httpfin
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/api/connections
curl -fsS http://127.0.0.1:8080/api/report > httpfin-report.txt
```

Open <http://127.0.0.1:8080/> for the inspector UI.

In production the collector binds to `127.0.0.1:8080` by default and starts
with an empty store. Synthetic demo observations are opt-in with
`httpfin_demo=1`.

Available endpoints:

- `/api/connections` — recent observations as JSON;
- `/api/report` — on-demand Squid-style text report;
- `/healthz` — health check;
- `/` — local UI.

## Where httpfin is placed

The main deployment is an HTTP observer behind the TLS termination point:

```text
client -- TLS --> Nginx -- HTTP --> application
                    |
                    └─ httpfin module → Go report/API
```

Nginx owns the certificate and decrypts the connection. The module then sees
the HTTP request, the negotiated TLS version, SNI and ALPN. This is the mode
supported by the current product and Docker example.

In TLS passthrough mode, Nginx only forwards encrypted bytes to another TLS
endpoint. `httpfin` does not see the HTTP request there. Nginx `ssl_preread`
can expose SNI and offered ALPN for routing, but it does not provide a full
ClientHello fingerprint to this HTTP module.

`ClientHello` is the first TLS handshake message sent by the client. It contains
the offered TLS versions, cipher suites and extensions. JA3/JA4 are derived
from such handshake data. Capturing them requires a TLS-aware listener at the
termination point. The current Nginx HTTP module is not such a listener; a
future direct-Go mode may add this with a suitable library.

ClientHello is not an HTTP request and does not contain the server response or
the user's data. It is sent once for each new TLS connection. Its contents vary
between TLS 1.2 and TLS 1.3 and between browsers, mobile browsers, SDKs and
command-line clients. The visible fields can include SNI, an ALPN list such as
`h2` and `http/1.1`, cipher suites, extensions, supported groups and signature
algorithms. SNI or ALPN may be absent, TLS resumption may add PSK/session data,
and Encrypted ClientHello (ECH) may hide the real SNI and inner handshake.
Therefore a ClientHello fingerprint is a diagnostic comparison value, not a
stable identity.

Reports are generated on request and are not written to a server-side report
directory. The redirected file is created by the shell running `curl`.

## Docker: Nginx + TLS + module

```bash
docker compose up --build
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  --http2 https://inspector.example.test:8443/healthz
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/api/report > httpfin-report.txt
```

If the Compose plugin is unavailable, build and run the image with Docker
directly:

```bash
docker build --pull=false -t httpfin:local .
docker run -d --rm --name httpfin -p 8443:8443 httpfin:local
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/healthz
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/api/report > httpfin-report.txt
docker rm -f httpfin
```

If Alpine package installation fails with a DNS error, retry with the host
network: `docker build --network=host --pull=false -t httpfin:local .`.

The example entrypoint creates a short-lived self-signed certificate. In
production, mount the real certificate and key into Nginx as read-only secrets.
The Go collector does not need the private key when Nginx terminates TLS. This
Compose mode is for observation and smoke tests. It does not change application
traffic or host firewall state.

## How the Nginx integration works

```text
client → Nginx / ingress → application
             │
             └─ native module → Unix datagram → Go httpfin → report/API
```

The native module runs in the Nginx HTTP request lifecycle. It sends one
bounded, non-blocking event after the final status is known. Go receives,
normalizes, stores and reports it.

Go does not connect back to the module. If Go is unavailable, Nginx continues
serving requests; the event is best-effort telemetry and may be lost.

The module does not parse raw ClientHello, calculate JA3/JA4 or observe
connection-level HTTP/2 SETTINGS. Those signals require a separate TLS entry
program or a future direct-Go mode with that capability.

See [`nginx-module/README.md`](nginx-module/README.md) and
[`docs/INTEGRATION_FORMAT.md`](docs/INTEGRATION_FORMAT.md).

## What httpfin can tell you

- what User-Agent the client declared;
- observed or negotiated TLS version, SNI and ALPN;
- HTTP method, target, status and source address;
- heuristic profiles for desktop and mobile clients such as `Chrome-like`,
  `Chrome Android-like`, `Edge-like`, `Firefox-like`, `Samsung Internet-like`,
  `Android WebView-like`, or `Generic HTTP`;

The current Nginx HTTP module does not produce JA3/JA4 or connection-level
HTTP/2 fingerprints. To collect them, a separate program must accept the TLS
connection before Nginx, or replace Nginx as the TLS entry point, read
ClientHello and HTTP/2 connection frames, and pass the resulting values to
`httpfin`.

User-Agent and fingerprints are client-controlled or imitable signals. An
empty fingerprint means that the observation point did not have the required
data; it is not a browser mismatch.

Timing and response size come from the direct Go mode or the Nginx access log.
The current Nginx HTTP module does not include them in its `httpfin` event.

## Which httpfin evidence is enough

| Goal | What httpfin provides now | What needs additional observation |
|---|---|---|
| API QA and regression testing | method, URI, status, User-Agent, protocol, plus TLS, SNI and ALPN in TLS mode | access log for timing and bytes; a separate observer for the exact TLS handshake or HTTP/2 |
| Nginx, proxy and ingress troubleshooting | source address, SNI, ALPN, TLS version and status; some fields must be checked against the access log | compare the original handshake before TLS termination |
| Comparing browsers, SDKs and mobile clients | User-Agent profile, available transport fields, and baseline comparison | a separate observer when the difference cannot be explained by the client version or network path |
| Investigating ClientHello differences | separate ClientHello fields and a local signature in direct Go TLS mode | standard JA3/JA4 or the complete raw handshake fields |
| Investigating HTTP/2 connection behavior | negotiated HTTP version and request result | SETTINGS, frame order, and other connection-level signals |
| Counting every request | a bounded in-memory report for a short diagnostic run | access logs or durable storage for complete accounting |

The deeper levels are not automatically better. They add deployment complexity,
certificate or packet-capture concerns, and more data to retain. They are only
justified when the current report cannot answer a specific operational question.
None of these fields alone is enough to identify a person or authorize or block
a client.

## Typical uses

- compare curl, SDKs, browsers and `curl-impersonate`;
- debug API clients, proxies, ingress and CDN paths;
- verify TLS termination, SNI, ALPN and source-IP handling;
- detect network regressions after Nginx, OpenSSL, browser or SDK updates;
- build an incident timeline for DevOps, API QA and security teams;
- provide a repeatable evidence artifact for an incident, rollout or QA run.

Detailed runbooks and examples:

- [`docs/USE_CASES_TEAM.md`](docs/USE_CASES_TEAM.md)
- [`docs/DEVOPS_RUNBOOK.md`](docs/DEVOPS_RUNBOOK.md)
- [`docs/OPERATING_MODES.md`](docs/OPERATING_MODES.md)
- [`docs/WIRE_OBSERVER.md`](docs/WIRE_OBSERVER.md)
- [`docs/openapi.yaml`](docs/openapi.yaml)
- [`TODO.md`](TODO.md)

## Development checks

```bash
go test ./...
go vet ./...
./tests/nginx-module-static.sh
./tests/container-smoke.sh
```

The module source check does not require Docker. The container smoke test
requires Docker and the Compose plugin; without it, use the direct Docker
build and run commands above.

The full Docker/Nginx module build is available manually from GitHub Actions as
`Manual Docker build`. It is intentionally separate from the fast CI check.

Russian documentation: [`README.ru.md`](README.ru.md).
