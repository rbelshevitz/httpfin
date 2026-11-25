# How teams use httpfin

`httpfin` is a network observation point for answering what reached an API,
through which TLS/HTTP path, and with what result.

## DevOps / SRE

After changing TLS, HTTP/2, proxy protocol, real-IP handling or an upstream:

1. run `httpfin` beside a test ingress;
2. send the same request through Nginx and directly to the application;
3. compare SNI, ALPN, protocol, source address, latency, status and bytes;
4. keep `/api/report` as the rollout artifact.

This exposes lost client IPs, HTTP/2-to-HTTP/1.1 changes, incorrect virtual
host selection, hidden TLS termination and upstream latency differences.

## API QA

API tests usually assert JSON and status only. `httpfin` adds protocol context:
HTTP version, offered and negotiated ALPN, SNI, timing, bytes, User-Agent and
profile consistency.

```bash
curl -sS https://api.example.test/health
curl_chrome116 --compressed https://api.example.test/health
curl_ff117 --compressed https://api.example.test/health
```

Keep the report with the test run. Negative cases should include missing SNI,
unsupported ALPN, HTTP/1.1 instead of HTTP/2, a false Chrome User-Agent, and
trusted versus untrusted proxy paths.

## Backend teams

When one SDK receives a different result from another, send both variants to
`httpfin` and compare method, URL, headers, body size, protocol, TLS, latency
and status. This is useful during migrations between `net/http`, libcurl,
mobile SDKs, gRPC gateways and service-mesh sidecars.

## Swagger / OpenAPI

The OpenAPI file describes the `httpfin` API, not a browser identity. Import
[`docs/openapi.yaml`](openapi.yaml) into Swagger Editor, Insomnia or Postman,
or use it to generate a typed client.

```bash
go run ./cmd/httpfin &
curl --fail http://127.0.0.1:8080/healthz
curl --fail http://127.0.0.1:8080/api/connections
curl --fail http://127.0.0.1:8080/api/report
```

## Nginx choices

- **Access log + external log consumer:** easiest retrofit; the current
  `httpfin` Go process does not read an Nginx log file automatically and there
  is no raw ClientHello in an ordinary access log.
- **TCP passthrough + `ssl_preread`:** exposes SNI/ALPN for routing, but not
  HTTP method, URL or headers.
- **Native module + Unix datagram:** sends a bounded structured event with
  request ID, HTTP fields, TLS version, SNI and negotiated ALPN.

The module is best-effort and non-blocking. Keep storage, classification,
baselines and reports in Go. See [Nginx dynamic modules](https://docs.nginx.com/nginx/admin-guide/dynamic-modules/).
