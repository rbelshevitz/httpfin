# httpfin — Client Fingerprint Inspector

[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![License](https://img.shields.io/badge/license-MIT-2ea44f?logo=opensourceinitiative&logoColor=white)](LICENSE)
[![Nginx](https://img.shields.io/badge/Nginx-1.28.3-009639?logo=nginx&logoColor=white)](Dockerfile)
[![Alpine](https://img.shields.io/badge/Alpine-3.23-0D597F?logo=alpinelinux&logoColor=white)](Dockerfile)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)](docker-compose.yml)
[![Focus](https://img.shields.io/badge/focus-HTTP%20%2F%20TLS%20diagnostics-6f42c1?logo=wireshark&logoColor=white)](docs/ARCHITECTURE.md)

> **Observe the client. Explain the path.**

`httpfin` is a Go diagnostic service for observing incoming HTTP clients. It
records request metadata, TLS/SNI/ALPN facts available at the observation
point, a declared User-Agent, and passive fingerprint signals. It produces a
compact Squid-style text report and a JSON API.

`curl-impersonate` is used only as a reference traffic generator. The upstream
project is created and maintained under the GitHub account [`lwthiker`](https://github.com/lwthiker);
the public profile does not provide a verified legal name. Its browser-like
TLS and HTTP/2 behavior is useful for regression fixtures, but it can become
outdated as browsers and TLS stacks change.

## Quick start

```bash
go run ./cmd/httpfin
```

Open `http://127.0.0.1:8080/` for the local inspector UI. In another terminal:

```bash
curl -v http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/api/connections
curl -fsS http://127.0.0.1:8080/api/report > httpfin-report.txt
```

This direct HTTP mode needs no certificate and is the simplest way to debug
API methods, headers, status and timing.

Endpoints:

- `GET /api/connections` — recent observations as JSON;
- `GET /api/report` — readable Squid-style text report;
- `GET /healthz` — health check.

For the tested Nginx + TLS + native module image:

```bash
docker compose up --build
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/healthz
```

Test the complete TLS/SNI/HTTP2 path and save its output:

```bash
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  --http2 https://inspector.example.test:8443/healthz
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/api/connections
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/api/report > httpfin-report.txt
```

`--resolve` makes the TLS SNI name match the test certificate and routes the
request to localhost. `-k` is needed only because the example entrypoint
creates a short-lived self-signed certificate.

If `curl-impersonate` is installed, compare available wrapper profiles:

```bash
curl_chrome116 --http2 https://inspector.example.test:8443/healthz
curl_ff117 --http2 https://inspector.example.test:8443/healthz
```

Wrapper names and supported browser versions depend on the upstream release.
Treat them as test fixtures, not permanent browser truth.

Run the integration checks:

```bash
go test ./...
go vet ./...
./tests/nginx-module-static.sh
./tests/container-smoke.sh
```

The last check requires a working Docker daemon.

The image is a diagnostic example. Its entrypoint creates a short-lived
self-signed certificate only when no certificate is mounted. Production
deployments should mount the real certificate and private key into Nginx as a
read-only secret; the Go collector does not need the key when Nginx terminates
TLS.

## What can be identified

`httpfin` can reliably record what was observed at its position in the path:

- the complete client-supplied User-Agent string;
- negotiated TLS version, SNI and ALPN when the observing TLS endpoint has
  access to them;
- the offered ALPN list and a stable ClientHello summary when `httpfin` itself
  terminates TLS;
- HTTP method, URI, status, timing, bytes and source address;
- a heuristic profile such as `Chrome-like`, `Firefox-like`, `Safari-like` or
  `Generic HTTP`.

The profile is an inference, not proof of a real browser. User-Agent is
client-controlled, and a tool such as `curl-impersonate` is specifically able
to make a request look browser-like. Exact JA3/JA4 and HTTP/2 fingerprints
require raw ClientHello and HTTP/2 connection-frame observation; they cannot be
reconstructed honestly from one post-termination HTTP request.

`JA3` is a normalized TLS ClientHello fingerprint; `JA3S` describes the server
response. `JA4` is a newer FoxIO TLS fingerprint, while `JA4H` describes HTTP
client behavior and `JA4+` names the wider family. These are grouping signals,
not RFC protocol fields or client authentication. The current `httpfin` module
does not calculate them; it can display values supplied by an external wire
observer.

## Nginx integration

The repository contains a native dynamic Nginx module. It is intentionally
thin: it creates a request ID, collects bounded request/TLS fields and sends a
non-blocking Unix datagram to Go. The Go process owns normalization, storage,
classification, baselines, risk hints and reports.

The module is engaged after Nginx has handled the TLS handshake: it initializes
request context in `NGX_HTTP_POST_READ_PHASE` and emits one event in
`NGX_HTTP_LOG_PHASE`, after the final request status is available. It is not a
raw ClientHello parser and does not participate in `stream ssl_preread`.

In plain terms: Nginx receives the request, the module copies a small summary,
and the Go program receives that summary and builds the API, UI and report. If
you run Go directly, you do not need the Nginx module. If you run Nginx with
the module, you also run the Go executable as its local event consumer.

The handoff is one-way: Go listens on `/run/httpfin/events.sock`, and the Nginx
module sends one JSON datagram after each request. Go does not connect to the
module, and no TCP port is needed between them.

```text
client → Nginx / ingress → application
             │
             ├─ TLS termination, SNI/ALPN, proxying, access log
             └─ ngx_http_httpfin_module.so → Unix datagram → Go httpfin
```

Read [`nginx-module/README.md`](nginx-module/README.md),
[`docs/OPERATING_MODES.md`](docs/OPERATING_MODES.md), and
[`docs/INTEGRATION_FORMAT.md`](docs/INTEGRATION_FORMAT.md) for module
build compatibility, TLS certificate placement, passthrough, and operational
failure modes. The OpenAPI description is in [`docs/openapi.yaml`](docs/openapi.yaml).
The observation boundaries and data flows are shown in
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

## Why the report is useful

The report is useful for API debugging, ingress changes, client-regression
checks, incident triage, and comparing a claimed browser identity with the
observed TLS/HTTP behavior. Its value comes from correlation: timestamp,
request, source, protocol, TLS/SNI/ALPN, User-Agent, profile, and consistency
are shown together.

It is a diagnostic evidence summary, not an authentication or blocking verdict.
Use it with rate limits, authentication logs, IP reputation, session behavior,
and application logs. Empty fingerprint fields mean that the current
observation point did not have the required wire data; they are not a failed
request.

## Report purpose and creation

`httpfin` creates a report from observations already held by the Go collector.
It does not re-scan traffic or invent missing TLS data. Each row combines an
operational summary with the signals available at the observation point:

- time, request ID, source address, method and URL;
- status, response size and elapsed time;
- HTTP protocol, TLS version, SNI and ALPN;
- User-Agent, inferred profile and consistency result;
- JA3, JA4 and HTTP/2 fields when a suitable wire observer supplied them.

Create a machine-readable snapshot with:

```bash
curl -fsS http://127.0.0.1:8080/api/connections > connections.json
```

Create a human-readable Squid-style artifact with:

```bash
curl -fsS http://127.0.0.1:8080/api/report > httpfin-report.txt
```

Use the JSON for CI, dashboards and further processing. Attach the TXT report
to an incident, rollout or API test result. A blank fingerprint field means
that the observation point did not receive the required wire data; it must not
be silently interpreted as a mismatch.

### Where the report is found

Reports are generated on demand; the service does not write them to a report
directory and does not run a background exporter. Use one of these entry
points:

- open `/` and click **Export report**;
- request `GET /api/report` in a browser or API client;
- save the response explicitly with shell redirection.

```bash
curl -fsS http://127.0.0.1:8080/api/report > httpfin-report.txt
```

The file is created in the current directory of the shell running `curl`, not
inside `httpfin`. In Docker, the file remains on the host only if the command
is run on the host or a volume is mounted. The current store is in memory; a
process restart clears runtime observations and restores only the built-in demo
records. Use access logs or an external database for durable history.

## Project goals

`httpfin` answers a practical question: “what client actually reached this
endpoint, and what changed on the way?” It is designed to help teams:

- debug API clients by comparing curl, SDKs, Go/Java/Python clients,
  browsers, and `curl-impersonate`;
- verify that an ingress or reverse proxy preserves source IP, SNI, ALPN,
  HTTP version, User-Agent, and timing;
- inspect TLS termination and confirm the negotiated protocol and server name;
- find inconsistencies such as a declared Chrome User-Agent with generic
  client-like network signals;
- detect regressions after upgrading Nginx, OpenSSL, browsers, SDKs, or
  proxies;
- compare direct traffic with traffic sent through a proxy, CDN, ingress, or
  service mesh;
- build a readable incident timeline from client, endpoint, status, and
  available fingerprint signals;
- provide evidence for security and anti-bot analysis without making an
  automatic blocking decision inside `httpfin`.

`httpfin` provides an observable technical picture and a useful report for
DevOps/SRE, API QA, backend, and security teams. It does not replace a WAF,
SIEM, browser telemetry, or an authentication system.

## Use cases

- compare ordinary curl, `curl-impersonate`, browsers, Go clients and API SDKs;
- debug an API client or reverse proxy that changes HTTP version, headers, SNI
  or ALPN;
- check whether an ingress preserves source identity and request timing;
- detect drift after upgrading Nginx, OpenSSL, a browser, or a client library;
- retain a readable incident timeline for DevOps, API QA and security teams.

See [`docs/USE_CASES_TEAM.md`](docs/USE_CASES_TEAM.md) for team-oriented
runbooks, [`docs/USE_CASES.md`](docs/USE_CASES.md) for practical examples, and
[`docs/DEVOPS_RUNBOOK.md`](docs/DEVOPS_RUNBOOK.md) for rollout diagnostics.
The anti-bot gatekeeper roadmap is in [`TODO.md`](TODO.md).

## References

- [curl-impersonate](https://github.com/lwthiker/curl-impersonate) and its
  [usage documentation](https://github.com/lwthiker/curl-impersonate/tree/main/docs);
- [RFC 6066 — SNI](https://www.rfc-editor.org/rfc/rfc6066);
- [RFC 7301 — ALPN](https://www.rfc-editor.org/rfc/rfc7301);
- [RFC 9113 — HTTP/2](https://www.rfc-editor.org/rfc/rfc9113);
- [RFC 9110 — HTTP semantics](https://www.rfc-editor.org/rfc/rfc9110);
- [RFC 7239 — Forwarded](https://www.rfc-editor.org/rfc/rfc7239);
- [RFC 9849 — Encrypted ClientHello](https://www.rfc-editor.org/rfc/rfc9849);
- [RFC 9846 — current TLS 1.3](https://www.rfc-editor.org/rfc/rfc9846);
- [RFC 8446 — original TLS 1.3](https://www.rfc-editor.org/rfc/rfc8446);
- [Nginx dynamic modules](https://docs.nginx.com/nginx/admin-guide/dynamic-modules/);
- [Nginx SSL preread](https://nginx.org/en/docs/stream/ngx_stream_ssl_preread_module.html).

Russian documentation: [`README.ru.md`](README.ru.md).
