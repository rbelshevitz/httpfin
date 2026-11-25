# httpfin: HTTP client observation

[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![Nginx](https://img.shields.io/badge/Nginx-1.28.3-009639?logo=nginx&logoColor=white)](Dockerfile)
[![Alpine](https://img.shields.io/badge/Alpine-3.23-0D597F?logo=alpinelinux&logoColor=white)](Dockerfile)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)](docker-compose.yml)
[![CI](https://github.com/rbelshevitz/httpfin/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/rbelshevitz/httpfin/actions/workflows/ci.yml)

> **Observe the client. Explain the path.**

`httpfin` is a diagnostic service placed in front of an API. It collects the
HTTP-client signals available at the observation point and produces a JSON API,
UI, and readable text report.

`httpfin` is not an authentication system, does not identify people, and does
not block requests. `curl-impersonate` is used only to generate reference
traffic; that project is maintained under the
[`lwthiker`](https://github.com/lwthiker) account.

## Quick start

```bash
go run ./cmd/httpfin
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/api/connections
curl -fsS http://127.0.0.1:8080/api/report > httpfin-report.txt
```

Open <http://127.0.0.1:8080/> for the UI. By default the collector listens only
on `127.0.0.1:8080` and keeps a bounded number of recent observations in memory.
Synthetic demo records are enabled only with `httpfin_demo=1`.

## Main deployment

```text
client -- TLS --> Nginx -- HTTP --> API
                    │
                    └─ httpfin module → Go collector → report/API
```

In the recommended mode, Nginx terminates TLS and exposes the HTTP request,
negotiated TLS version, SNI, and ALPN to the module. The module sends one
bounded, non-blocking event to Go over a Unix datagram socket. If the collector
is unavailable, Nginx continues serving requests, but the event may be lost.

Reports are generated when `/api/report` is requested. The service does not
write a report file; save the response by redirecting command output.

## What the current httpfin provides

- HTTP method, target, status, and source address;
- User-Agent and a heuristic client profile;
- TLS version, SNI, and ALPN when available at the observation point;
- changes between sequential observations;
- a JSON API, UI, and text report.

Timing and response size come from the direct Go mode or the Nginx access log.
The current Nginx HTTP module does not include them in its event.

## Important limits

`ClientHello` is the first TLS message sent by a client. It contains offered
TLS versions, cipher suites, and extensions. In the ordinary Nginx HTTP phase
it is no longer available, so the module does not calculate standard JA3/JA4
or observe connection-level HTTP/2 settings.

With TLS passthrough, Nginx only forwards encrypted data. `httpfin` cannot see
the URL, headers, User-Agent, or response status there. `ssl_preread` can expose
SNI and offered ALPN for routing, but not a complete ClientHello fingerprint.

User-Agent and profiles can be spoofed. A missing signal means that it was not
available at the selected observation point; it is not a mismatch. The report
is for diagnosis, not for blocking or identity verification.

## Docker

```bash
docker compose up --build
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/healthz
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/api/report > httpfin-report.txt
```

Without the Compose plugin:

```bash
docker build --pull=false -t httpfin:local .
docker run -d --rm --name httpfin -p 8443:8443 httpfin:local
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/healthz
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/api/report > httpfin-report.txt
docker rm -f httpfin
```

The image creates a temporary self-signed certificate for testing only. In
production, provide the real certificate and key to Nginx as read-only secrets.
If Alpine cannot download packages because of DNS, retry the build with
`--network=host`.

## Documentation

- [`docs/USE_CASES_TEAM.md`](docs/USE_CASES_TEAM.md) — workflows for DevOps, QA, and developers;
- [`docs/OPERATING_MODES.md`](docs/OPERATING_MODES.md) — TLS, certificates, and Nginx modes;
- [`docs/INTEGRATION_FORMAT.md`](docs/INTEGRATION_FORMAT.md) — module events and report format;
- [`docs/WIRE_OBSERVER.md`](docs/WIRE_OBSERVER.md) — ClientHello and HTTP/2 boundaries;
- [`docs/DEVOPS_RUNBOOK.md`](docs/DEVOPS_RUNBOOK.md) — operational troubleshooting;
- [`TODO.md`](TODO.md) — current work plan.

## Development checks

```bash
go test ./...
go vet ./...
./tests/nginx-module-static.sh
./tests/container-smoke.sh
```

The full Docker/Nginx module build is available manually through the GitHub
Actions workflow `Manual Docker build`. Fast CI checks the Go code and module
source.

Russian documentation: [`README.ru.md`](README.ru.md).
