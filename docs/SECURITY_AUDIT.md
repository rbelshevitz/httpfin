# Security and reliability audit

This is an engineering audit of the current implementation, not a claim of
formal security certification. The highest-priority items should be fixed
before exposing the direct listener or the Unix socket to an untrusted
environment.

## Findings

### P0: possible Nginx worker memory corruption, fixed in working tree

`nginx-module/ngx_http_httpfin_module.c` builds a 4096-byte JSON event with
unchecked `ngx_cpymem()` calls. The string helper can stop at the buffer
boundary, but the caller does not know that truncation happened and continues
writing field prefixes and the closing suffix. A long URI, host or User-Agent
can therefore write past the stack buffer and crash or corrupt an Nginx worker.

The working tree now uses a bounded builder that checks every append and returns
`oversize` before writing beyond the buffer. The pinned Nginx 1.28.3 source build
and container smoke test pass.

### P0/P1: unbounded ClientHello map, fixed in working tree

`internal/tlsinspect.Observer` stores a Hello on every ClientHello and removes
it only when an HTTP request later consumes the entry. Aborted or incomplete
handshakes remain in the map. A handshake flood can grow memory without bound.

The observer now applies a TTL and a hard maximum and removes stale entries.
Connection-level cleanup is still limited by the Go TLS callback API.

### P1: missing HTTP/TLS timeouts, fixed in working tree

Both Go servers are created without read, write, header, idle or maximum-header
limits. Slow clients can hold goroutines and file descriptors for too long.

Both listeners now set read, write, header, idle and maximum-header limits.

### P1: socket spoofing and event injection, mitigated

The socket now uses mode `0660`, supports an explicit `httpfin_socket_group`,
and rejects unknown event versions and oversized input fields. Keep the runtime
directory accessible only to the Nginx worker group; any member of that group
can still inject events by design.

### P1: no reconnect after collector restart, fixed in working tree

The module retries the connection on a later request when the collector socket
is unavailable and closes a failed connected socket so the next request can
reconnect after a Go restart.

### P2: silent ingest failure, mitigated

The reader logs fatal read errors and `/healthz` returns `503` when the ingest
loop has stopped. A deployment should still alert on health failures.

### P2: inefficient bounded store, fixed in working tree

The store is now a fixed-capacity ring buffer. Datagram fields are bounded
before they reach the store. Direct HTTP fields still follow the server header
limit and should be redacted at the report boundary for sensitive deployments.

### P2: incomplete graceful shutdown, fixed in working tree

The process handles `SIGINT`/`SIGTERM`, drains HTTP and TLS servers with a
deadline, closes the TLS listener and then stops the Unix socket reader.

### P2: Docker signal forwarding, fixed in working tree

The image entrypoint keeps a small supervisor shell so termination reaches both
Nginx and Go. The image pins the Nginx and Alpine versions used by the build
and starts Nginx workers in the `nobody` group, matching the protected socket.

### P2: response writer compatibility, fixed in working tree

The logging wrappers now preserve `Flusher`, `Hijacker` and `Unwrap` when the
underlying writer supports them. The current inspector does not use upgrades,
but the middleware no longer needlessly hides these capabilities.

## Checks performed

```text
go test ./...
go test -race ./...
go vet ./...
./tests/nginx-module-static.sh
```

The container smoke test passed with the pinned Alpine 3.23/Nginx 1.28.3 image:
Go build, `.so` compilation/loading, runtime library checks, TLS/SNI/ALPN and
Unix-datagram delivery were all exercised.
