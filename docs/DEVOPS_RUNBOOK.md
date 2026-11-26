# httpfin for DevOps teams

## What httpfin provides

Use `httpfin` to inspect the request path at the API edge:

- confirm delivery from Nginx to the Go collector;
- inspect the source address, SNI, ALPN, HTTP version, status, and request time;
- compare the Nginx handoff with `/api/ingest` and access-log volume;
- attach a readable report to an incident ticket.

## Run the Docker image

With a direct collector run, the status dashboard and HTTP API are available on
port `8080`: <http://127.0.0.1:8080/>. In the Docker smoke setup, external
HTTPS access goes through Nginx on port `8443`; Nginx proxies requests to the
collector.

```bash
docker build -t httpfin:dev .
docker run --rm -p 8443:8443 httpfin:dev
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/healthz
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/api/ingest
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/api/report
```

The generated certificate is for diagnostics only. Mount a real certificate
and key into `/etc/nginx/certs` in production.

This image is a demo/smoke environment: `/app/` is served by the bundled mock
API. It is not a production reverse proxy configuration. For a real
application, use the host/systemd deployment and point Nginx at that
application.

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
log handler runs. Verify delivery through `/api/connections`, `/api/ingest`,
and collector errors in the log rather than that access-log field alone.
`/api/ingest` reports the collector-side lifecycle: `received`, `decoded`,
`stored`, and `dropped`. These counters do not prove that Nginx emitted every
request; they begin when a datagram reaches the collector socket.

Possible module states include `sent`, `unavailable`,
`dropped_backpressure`, `dropped_error`, and `oversize`. Backpressure is a
telemetry-load signal, not a reason to block a request. The collector counters
start only after a datagram reaches its socket, so use the Nginx handoff field
and access logs when investigating losses before that point. Use a durable
queue for lossless audit requirements.

### Client address

Configure trusted proxy hops explicitly, for example:

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
5. Treat fingerprints as diagnostic evidence, not as identity proof.

Emergency bypass: set `httpfin off;` in the affected `server` or `location`,
run `nginx -t`, and reload Nginx. This removes module handoff from the request
path while leaving the proxy and access log available. Keep the Go collector
running until the incident is diagnosed, then restore `httpfin on;` on the
canary first and repeat the event/access-log comparison.
