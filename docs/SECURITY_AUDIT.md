# Security and reliability audit

This is an engineering audit of the current implementation, not a claim of
formal security certification. The highest-priority items should be fixed
before exposing the direct listener or the Unix socket to an untrusted
environment.

## Findings

### P0 — possible Nginx worker memory corruption — fixed in working tree

`nginx-module/ngx_http_httpfin_module.c` builds a 4096-byte JSON event with
unchecked `ngx_cpymem()` calls. The string helper can stop at the buffer
boundary, but the caller does not know that truncation happened and continues
writing field prefixes and the closing suffix. A long URI, host or User-Agent
can therefore write past the stack buffer and crash or corrupt an Nginx worker.

The working tree now uses a bounded builder that checks every append and returns
`oversize` before writing beyond the buffer. A real Nginx source build and
container smoke test are still required before release validation.

### P0/P1 — unbounded ClientHello map — fixed in working tree

`internal/tlsinspect.Observer` stores a Hello on every ClientHello and removes
it only when an HTTP request later consumes the entry. Aborted or incomplete
handshakes remain in the map. A handshake flood can grow memory without bound.

The observer now applies a TTL and a hard maximum and removes stale entries.
Connection-level cleanup is still limited by the Go TLS callback API.

### P1 — missing HTTP/TLS timeouts — fixed in working tree

Both Go servers are created without read, write, header, idle or maximum-header
limits. Slow clients can hold goroutines and file descriptors for too long.

Both listeners now set read, write, header, idle and maximum-header limits.

### P1 — socket spoofing and event injection — open

The Unix socket is changed to mode `0666`. Any local process that can reach the
path can inject observations or flood the collector. Use a protected runtime
directory, a dedicated group and mode `0660`; validate event version and field
limits.

### P1 — no reconnect after collector restart — open

The Nginx worker connects its datagram socket once. Restarting Go creates a new
socket, but existing Nginx workers do not reconnect automatically. Events are
dropped until workers are reloaded or the module gains reconnect logic.

### P2 — silent ingest failure — open

The Go datagram reader exits on a non-timeout read error while the HTTP service
continues to report healthy. Add logging and an ingest health signal.

### P2 — inefficient bounded store — open

The store is capped at 5000 records, but prepending each record copies the
whole slice. A ring buffer would reduce allocation and GC pressure at high
request rates. Field lengths should also be capped before storing user input.

### P2 — incomplete graceful shutdown — open

`log.Fatal` exits immediately and bypasses deferred cleanup. Add signal-driven
shutdown for HTTP servers, the TLS listener and the Unix socket.

### P2 — response writer compatibility — open

The logging wrappers do not preserve optional interfaces such as `Flusher` and
`Hijacker`. This can affect streaming or protocol-upgrade handlers even though
the current inspector UI and API do not require them.

## Checks performed

```text
go test ./...
go test -race ./...
go vet ./...
./tests/nginx-module-static.sh
```

The container smoke test additionally requires an available Docker daemon.
