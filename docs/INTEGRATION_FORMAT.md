# httpfin and Nginx integration format

## Component responsibilities

```text
Client → Nginx → application
           ├─ TLS termination, SNI/ALPN, proxying, access log
           └─ native module → Unix datagram → Go httpfin → optional security log
```

Nginx handles data-plane work: TLS, virtual-host selection, ALPN, proxying and
access logging. The module creates a request ID and sends a bounded,
non-blocking event. Go normalizes, profiles, applies baseline/policy, stores and
reports it, and may write the host security log for an external Fail2ban setup.

The event is best-effort telemetry. The module must not wait for Go, perform
DNS/HTTP/database work, or calculate expensive fingerprints inside an Nginx
worker.

## Why the module exists and whether Go is required

For an API developer, the short answer is: the usual deployment runs two
processes, Nginx and `httpfin`. Nginx receives the request; `httpfin` exposes
the result through the UI, JSON API and TXT report. The module connects them.

The module is useful when Nginx is already the TLS terminator and proxy, but
the team needs a structured handoff that is richer and more immediate than
parsing access logs. It adds request ID, selected HTTP fields, TLS variables
and delivery status without moving storage or analysis into an Nginx worker.

For the native-module integration, the Go executable is required:

```text
Nginx worker
  └─ ngx_http_httpfin_module.so
       └─ bounded Unix datagram
            └─ httpfin Go executable
                 ├─ in-memory store
                 ├─ profile/risk/baseline/policy processing
                 ├─ /api/connections and /api/report
                 ├─ inspector UI
                 └─ optional host security log → Fail2ban
```

The `.so` alone has no report storage, UI, API or durable queue. If the Go
socket consumer is absent, the module can only mark the event unavailable or
drop it; Nginx request processing continues. If only access logs are needed,
do not load the module. If direct HTTPS observation is sufficient, run the Go
executable without Nginx. The module + Go pair is the production integration
path for Nginx termination plus structured analysis.

### Who connects to whom

The handoff is local and one-way:

1. Go creates and listens on the Unix datagram socket
   `/run/httpfin/events.sock` at startup.
2. Each Nginx worker creates a `SOCK_DGRAM` socket and connects it to that path.
3. After each processed HTTP request, the module sends one JSON event with
   `send()`.
4. Go reads the datagram, decodes it into a `Connection`, and adds it to the
   store.

There is no HTTP request, TCP port, or bidirectional RPC between Nginx and Go.
Go does not connect to the module: Go listens on the socket, while the module
sends data to it. If Go has not started or the socket is unavailable, Nginx
continues serving requests but the telemetry event is lost.

You do not call the `.so` directly: Nginx loads it with `load_module`. You do
not write a Unix-socket client: `httpfin` creates and listens on the socket. In
Docker, one entrypoint starts both processes.

## Event fields

The module sends version `1` JSON containing request ID, method, URI, host,
User-Agent, source address, status, TLS version, SNI and negotiated ALPN. Empty
TLS fields are expected for plain HTTP, passthrough, or unavailable variables.

The event feeds `/api/connections`. Exact JA3/JA4 and HTTP/2 fingerprints need
raw ClientHello and connection-frame observation; one post-termination request
does not contain enough information.

## Reports

The report is a diagnostic artifact built from collector observations. It
combines an access-style row with available protocol and fingerprint signals;
it is not a second packet capture and it never fills missing wire data with an
assumption.

Get JSON for automation:

```bash
curl -fsS http://127.0.0.1:8080/api/connections > connections.json
```

Get the Squid-style text report for an incident, rollout or API-test artifact:

```bash
curl -fsS http://127.0.0.1:8080/api/report > httpfin-report.txt
```

The text report contains an access summary, a client-level summary with the
observe-only bot signal, and per-observation fingerprint details. Each row
shows the observation point and evidence state, plus changes from the previous
observation for the same client IP; missing evidence is rendered as `unknown`,
not as a mismatch. Sensitive query parameters are redacted at the report
boundary. Use JSON in CI and dashboards; attach TXT to a ticket or test result.
Empty
JA3/JA4/HTTP/2 fields mean that the required wire observation was unavailable.

Reports are generated synchronously from the current in-memory store when
`GET /api/report` is handled. No report file is created by the service. A file
appears only when the caller saves the HTTP response, for example with
`> httpfin-report.txt`. Restarting the process clears runtime observations;
durable retention belongs to access logs or an external storage component.

## TLS and certificates

- **Nginx termination:** certificate and key belong to Nginx. Go does not need
  the private key.
- **Direct Go HTTPS:** configure `httpfin_tls_cert` and `httpfin_tls_key`.
- **TLS passthrough:** Nginx uses `ssl_preread` and the certificate belongs to
  the downstream TLS service. Nginx can see SNI/ALPN but not HTTP headers.

Never bake production private keys into the image. Mount them as read-only
secrets.

## Building outside Docker

Build the module against the exact Nginx version, operating system/libc and
relevant configure flags that will load it. Use `--with-compat`, add the module
with `--add-dynamic-module`, run `make modules`, install the `.so`, and finish
with `nginx -t`.

Do not move an Alpine/musl module to a glibc host or the reverse. A dynamic
module is not a portable standalone binary.

```bash
cd /path/to/nginx-1.28.3
./configure --with-compat \
  --with-http_ssl_module --with-http_v2_module \
  --with-stream --with-stream_ssl_preread_module \
  --add-dynamic-module=/path/to/httpfin/nginx-module
make modules
install -D -m 0644 objs/ngx_http_httpfin_module.so \
  /usr/lib/nginx/modules/ngx_http_httpfin_module.so
nginx -t
```

See [Nginx configure](https://nginx.org/en/docs/configure.html),
[dynamic modules](https://docs.nginx.com/nginx/admin-guide/dynamic-modules/),
[SSL preread](https://nginx.org/en/docs/stream/ngx_stream_ssl_preread_module.html),
and the complete [observation model](ARCHITECTURE.md). Protocol sources are
[RFC 9846](https://www.rfc-editor.org/rfc/rfc9846) and historical
[RFC 8446](https://www.rfc-editor.org/rfc/rfc8446) for TLS 1.3,
[RFC 6066](https://www.rfc-editor.org/rfc/rfc6066) for SNI,
[RFC 7301](https://www.rfc-editor.org/rfc/rfc7301) for ALPN,
[RFC 9113](https://www.rfc-editor.org/rfc/rfc9113) for HTTP/2,
[RFC 9110](https://www.rfc-editor.org/rfc/rfc9110) for HTTP semantics,
[RFC 7239](https://www.rfc-editor.org/rfc/rfc7239) for `Forwarded`, and
[RFC 9849](https://www.rfc-editor.org/rfc/rfc9849) for Encrypted ClientHello.

## Validation

```bash
go test ./...
go vet ./...
./tests/nginx-module-static.sh
./tests/container-smoke.sh
```

The container smoke test checks runtime libraries, module loading, TLS 1.3,
SNI, ALPN and delivery of an event to the Go API.
