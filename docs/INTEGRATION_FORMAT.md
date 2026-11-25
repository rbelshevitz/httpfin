# httpfin and Nginx integration format

## Component responsibilities

```text
Client → Nginx → application
           ├─ TLS termination, SNI/ALPN, proxying, access log
           └─ native module → Unix datagram → Go httpfin
```

Nginx handles data-plane work: TLS, virtual-host selection, ALPN, proxying and
access logging. The module creates a request ID and sends a bounded,
non-blocking event. Go normalizes, stores, classifies and reports it.

The event is best-effort telemetry. The module must not wait for Go, perform
DNS/HTTP/database work, or calculate expensive fingerprints inside an Nginx
worker.

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

The text report contains an access summary followed by fingerprint details.
Use JSON in CI and dashboards; attach TXT to a ticket or test result. Empty
JA3/JA4/HTTP/2 fields mean that the required wire observation was unavailable.

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
and [RFC 6066](https://www.rfc-editor.org/rfc/rfc6066),
[RFC 7301](https://www.rfc-editor.org/rfc/rfc7301), and
[RFC 9113](https://www.rfc-editor.org/rfc/rfc9113).

## Validation

```bash
go test ./...
go vet ./...
./tests/nginx-module-static.sh
./tests/container-smoke.sh
```

The container smoke test checks runtime libraries, module loading, TLS 1.3,
SNI, ALPN and delivery of an event to the Go API.
