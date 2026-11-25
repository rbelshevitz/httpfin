# httpfin for DevOps teams

## What httpfin provides

`httpfin` helps answer questions that an ordinary access log often cannot:

- did the request reach Nginx and the Go collector;
- which source address does the edge actually see;
- which SNI and ALPN arrived from the client;
- was HTTP/2 or HTTP/1.1 selected;
- where did latency appear;
- were telemetry events dropped because of Unix-socket backpressure;
- can the result be attached to an incident ticket as a readable report.

## Run the Docker image

```bash
docker build -t httpfin:dev .
docker run --rm -p 8443:8443 httpfin:dev
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/healthz
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/api/report
```

The generated certificate is for diagnostics only. Mount a real certificate
and key into `/etc/nginx/certs` in production.

## Post-deployment checks

### TLS and SNI

```bash
openssl s_client -connect 127.0.0.1:8443 \
  -servername inspector.example.test -alpn h2,http/1.1 \
  </dev/null 2>/dev/null | openssl x509 -noout -subject -issuer
```

The report should show SNI, TLS version and negotiated ALPN. An empty SNI is a
real observation; do not replace it with the HTTP `Host` value.

### Module handoff

The Nginx access log contains a diagnostic `handoff` variable. It may still
show `not_sent` because Nginx log variables can be evaluated before the module
log handler runs. Verify delivery through `/api/connections` and collector
errors in the log rather than that access-log field alone. Runtime loss counters
are not enabled yet.

Possible module states include `sent`, `unavailable`,
`dropped_backpressure`, `dropped_error`, and `oversize`. Backpressure is a
telemetry-load signal, not a reason to block a request. Use access logs or a
durable queue for lossless audit requirements.

### Client address

Configure trusted proxy hops explicitly:

```nginx
set_real_ip_from 10.0.0.0/8;
real_ip_header X-Forwarded-For;
real_ip_recursive on;
```

Never trust an arbitrary client-supplied `X-Forwarded-For` value.

## Common incidents

### Everything became 502

Check `nginx -t`, module loading, the `127.0.0.1:8080` upstream, port
collisions, and whether Nginx was upgraded without rebuilding the module.

### SNI is missing

Possible causes are a client without SNI, TLS termination before Nginx, a
plain HTTP listener, passthrough with the wrong module phase, or encrypted
client hello details unavailable to the observer.

### The module slows Nginx

The module must use non-blocking `SOCK_DGRAM`. It must not perform HTTP, DNS,
database, cgo, or collector-wait operations in a request phase.

## Rollout guidance

1. Enable the module on one canary ingress.
2. Compare collector events with access-log volume.
3. Keep reports for a limited period and mask secrets in URLs and query strings.
4. Upgrade Nginx and the native module as one compatible pair.
5. Treat fingerprints as diagnostic signals, not as the only blocking input.

Emergency bypass: set `httpfin off;` in the affected `server` or `location`,
run `nginx -t`, and reload Nginx. This removes module handoff from the request
path while leaving the proxy and access log available. Keep the Go collector
running until the incident is diagnosed, then restore `httpfin on;` on the
canary first and repeat the event/access-log comparison.
