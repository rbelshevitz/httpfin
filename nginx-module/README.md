# httpfin native NGINX module

This directory contains the native NGINX integration for `httpfin`. It is an
HTTP dynamic module written in C. It does not replace the Go inspector and
does not calculate JA3/JA4 or HTTP/2 fingerprints inside an NGINX worker.

## What it does

When `httpfin on;` is enabled, the module:

1. allocates a request id in `NGX_HTTP_POST_READ_PHASE`;
2. emits one bounded JSON event in `NGX_HTTP_LOG_PHASE`;
3. sends the event with a connected, non-blocking Unix `SOCK_DGRAM` socket;
4. exposes `$httpfin_request_id` and `$httpfin_handoff_status` for access logs.

`$httpfin_handoff_status` is `sent`, `dropped_backpressure`,
`dropped_error`, `unavailable`, `oversize`, or `not_sent`. Dropping is
intentional: the NGINX request path never waits for the Go consumer.

## Build

### Runtime libraries and OpenSSL

The Go binary is built with `CGO_ENABLED=0`, so it does not need OpenSSL,
PCRE2, or zlib on the host. The C module also does not call OpenSSL and is not
linked directly to `libssl` or `libcrypto`; it uses NGINX's module ABI and POSIX
Unix sockets. NGINX itself still needs the runtime libraries selected by its
build: OpenSSL for `http_ssl_module` and stream TLS, PCRE2 for regular
expressions, and zlib for gzip support.

On Alpine, the runtime package names are `openssl`, `pcre2`, and `zlib`; the
module build additionally needs `openssl-dev`, `pcre2-dev`, `zlib-dev`,
`linux-headers`, a compiler, and the matching `nginx-mod-dev`/NGINX source.
On Debian/Ubuntu/RHEL-like systems, use the distribution's corresponding
runtime and development packages. Do not copy a module built on Alpine into a
glibc-based NGINX installation.

The container intentionally keeps the NGINX/OpenSSL dependency in the NGINX
runtime layer. Verify the actual host or image with:

```sh
nginx -V 2>&1
ldd "$(command -v nginx)"
nginx -t
```

The module must be compiled against the same NGINX version and compatible
configure flags as the binary that loads it. `--with-compat` helps with
configure-feature differences; it does not make an arbitrary NGINX version or
libc combination safe.

Outside Docker, separate the dependencies into two groups:

- the `httpfin` Go binary is built with `CGO_ENABLED=0` and does not require
  OpenSSL;
- NGINX requires the runtime libraries it was built with; the `httpfin` module
  does not call OpenSSL and must not link directly to `libssl`.

For Alpine 3.23, use `nginx-mod-dev` from the same NGINX package branch as the
installed `nginx`. The v3.23 package branch contains NGINX 1.28.3; verify
`nginx -V` against the source tree before building. On Debian/Ubuntu/RHEL,
use development packages and sources from the same package branch as the NGINX
binary.

Do not treat `.so` as a portable artifact: Alpine/musl and Debian/RHEL/glibc
are different ABI environments, and a dynamic module is tied to the NGINX
version and configure profile.

The Docker smoke-test pins Alpine 3.23's packaged stable family, NGINX
1.28.3, instead of following the newest mainline release. This keeps the
module and the runtime NGINX binary on one known version line.

The module must be built against the exact NGINX source/release family used by
the target binary. `--with-compat` improves compatibility but is not a promise
across incompatible NGINX builds.

```sh
make check
make build NGINX_SOURCE=/path/to/nginx-source \
  NGINX_CONFIGURE_ARGS='--with-compat --with-http_ssl_module --with-http_v2_module --with-stream --with-stream_ssl_preread_module'
sudo make install NGINX_SOURCE=/path/to/nginx-source PREFIX=/usr/local
```

`NGINX_CONFIGURE_ARGS` should match the flags reported by the target
`nginx -V`. The Makefile is a convenience for a source tree; it is not a
replacement for reproducing the distribution's complete configure command.

The resulting `.so` is loaded at top-level with:

```nginx
load_module modules/ngx_http_httpfin_module.so;
```

See `httpfin-module.conf.example` for an end-to-end HTTP/TLS configuration.

## TLS and SNI boundary

For TLS terminated by NGINX, the module copies `$ssl_server_name`,
`$ssl_protocol`, and `$ssl_alpn_protocol` into the handoff event and they can
also be written with `$ssl_cipher` to the access log. It does not parse the
raw ClientHello and cannot produce a trustworthy JA3/JA4 from an HTTP-phase
hook.

For TLS passthrough, use NGINX `stream` plus
`ssl_preread on;`. The built-in stream module exposes
`$ssl_preread_server_name`, `$ssl_preread_alpn_protocols`, and
`$ssl_preread_protocol`. The last value is the highest TLS version advertised
by the client in ClientHello, not the version negotiated by the downstream
TLS endpoint. Route or log those values separately, or add a future stream
companion module. An HTTP module cannot see a passthrough connection.

HTTP/2 frame/settings fingerprints require an HTTP/2-aware observer. They are
not reconstructed from the ordinary HTTP request phase in this module.

## Handoff format

Each datagram is newline-terminated JSON, capped at 4096 bytes:

```json
{"version":1,"request_id":"1234-7","method":"GET","uri":"/healthz","host":"example.test","user_agent":"curl/8","remote_addr":"127.0.0.1","status":200,"tls_version":"TLS 1.3","sni":"example.test","alpn":"h2"}
```

The consumer owns the socket lifecycle and must bind it before NGINX workers
start. A datagram socket has finite kernel receive capacity; the module reports
backpressure but does not retry. The Go side should treat this stream as
best-effort telemetry and correlate it with NGINX access logs when lossless
audit records are required.

## Compatibility caveats

- Dynamic modules are ABI-sensitive. Build with the same NGINX version and
  configure flags where possible; test `nginx -t` before reload.
- The module uses HTTP phases only and is not a stream module.
- It assumes Linux/POSIX `AF_UNIX`, `SOCK_DGRAM`, `O_NONBLOCK`, and
  `MSG_NOSIGNAL`.
- It sends no request body and intentionally avoids blocking I/O, DNS, locks,
  network calls, and calls into Go/cgo.
- A client-selected SNI is routing metadata, not client identity. TLS session
  resumption, ECH, proxies, and TLS termination topology can make a field
  unavailable or misleading.
- The module must not be used as the sole security decision for bot detection;
  fingerprint signals are probabilistic and can be replayed.

## References

- NGINX build options: <https://nginx.org/en/docs/configure.html>
- NGINX development guide: <https://nginx.org/en/docs/dev/development_guide.html>
- HTTP phases and module lifecycle: <https://nginx.org/en/docs/dev/development_guide.html>
- TLS preread/SNI/ALPN: <https://nginx.org/en/docs/stream/ngx_stream_ssl_preread_module.html>
- TLS stream variables: <https://nginx.org/en/docs/stream/ngx_stream_ssl_module.html>
- TLS 1.3: <https://www.rfc-editor.org/rfc/rfc9846>
- TLS 1.3 original specification: <https://www.rfc-editor.org/rfc/rfc8446>
- SNI: <https://www.rfc-editor.org/rfc/rfc6066>
- ALPN: <https://www.rfc-editor.org/rfc/rfc7301>
- HTTP/2: <https://www.rfc-editor.org/rfc/rfc9113>
- HTTP semantics: <https://www.rfc-editor.org/rfc/rfc9110>
- Encrypted ClientHello: <https://www.rfc-editor.org/rfc/rfc9849>
- Forwarded: <https://www.rfc-editor.org/rfc/rfc7239>
- JA3 reference: <https://github.com/salesforce/ja3>
- JA4+ reference: <https://github.com/FoxIO-LLC/ja4>
- curl-impersonate: <https://github.com/lwthiker/curl-impersonate>
