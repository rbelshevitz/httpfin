# httpfin integration tests

These tests are intentionally outside the runtime packages and do not modify
the Go server, the Dockerfile, or the native module source.

## Unix datagram format

```sh
go test ./tests ./internal/ingest
```

`unix_datagram_format_test.go` sends the exact newline-terminated JSON format
emitted by `ngx_http_httpfin_emit`, verifies decoding into a stored connection,
and verifies malformed datagrams are ignored.

## Native module static checks

```sh
./tests/nginx-module-static.sh
```

For a real build against an already downloaded NGINX source tree:

```sh
NGINX_SOURCE=/path/to/nginx-source ./tests/nginx-module-static.sh
```

The script checks the non-blocking Unix datagram, bounded payload, lifecycle
phase, variables, and event fields. It does not claim that a source scan is a
replacement for `nginx -t`.

## Container TLS/SNI smoke test

```sh
./tests/container-smoke.sh
```

## Dashboard bot-farm smoke test

Generate synthetic browser and API-client traffic, validate the canonical
snapshot, and capture the dashboard:

```bash
./tests/dashboard-smoke.sh
```

The screenshot is written to `/tmp/httpfin-dashboard-smoke.png` by default.
This test uses a standalone local collector and never sends traffic to a real
endpoint.

The script builds the existing pinned image, starts the existing Compose stack,
uses `curl --resolve` so the TLS ClientHello contains
`inspector.example.test`, requests HTTP/2 when the host curl supports it, and
checks runtime dependencies, and checks both sides of the path:

* NGINX resolves its OpenSSL, PCRE2 and zlib runtime libraries;
* the httpfin module has no direct OpenSSL/PCRE2/zlib dependency;
* NGINX access JSON contains the SNI and negotiated ALPN;
* the Go API exposes the handed-off `/healthz` request.

The access log still contains the handoff variable for diagnostics. Depending
on Nginx log-handler ordering, that value can be `not_sent` in the same line
even though the datagram is delivered later in the log phase; the authoritative
smoke assertion is the event visible through the Go API.

If Docker or Compose is unavailable, the script exits successfully with a
`SKIP` message. This makes it suitable for environments where Docker cannot be
run; the command remains reproducible when Docker and Compose are available.
When only the Docker CLI is available, use the direct `docker build` and
`docker run` smoke path documented in the main README.
