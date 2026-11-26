# Practical httpfin use cases

Use `httpfin` to compare client, proxy, and ingress behavior when the response
body is only part of the diagnosis.

## QA quick check

1. Check `/healthz` and expect `204`.
2. Send the agreed request through the product entry point.
3. Verify the product response, then find the observation in
   `/api/connections`.
4. Save `/api/report` with the test result. Use `/api/ingest` when checking
   whether the Nginx handoff reached the collector.

## API client debugging

Send the same request through ordinary curl, an SDK, a browser and
`curl-impersonate`. Compare User-Agent, TLS version, SNI, ALPN, HTTP version,
latency, status and bytes in `/api/report`.

## Incoming-client diagnostics

Use the service as a temporary edge endpoint to inspect requests from mobile
clients, partner integrations, webhooks, SDKs and automation. The report shows
what was actually received; it does not claim that a User-Agent is genuine.

## Proxy and ingress verification

Compare direct and proxied requests to find changes in source address, SNI,
ALPN, HTTP version, `Host`, `Forwarded`, User-Agent and timing.

## Outgoing request inspection

Temporarily point an SDK, webhook sender, job or integration test at
`httpfin`. It acts as a controlled test receiver: the report shows what the
sender actually emitted, including User-Agent, HTTP version, TLS/SNI/ALPN when
available, status and timing. It is not an outbound proxy and does not create
traffic by itself.

```bash
go run ./cmd/httpfin

# from another terminal
curl -H 'X-Test-Case: plain-curl' http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/api/report > outgoing-report.txt
```

For TLS/SNI testing, use the HTTPS listener or the Docker Nginx endpoint and
send the request with the expected hostname.

## TLS and SNI troubleshooting

Use direct HTTPS mode for direct TLS diagnostics. The current Go listener does
not yet parse raw ClientHello. Use Nginx stream `ssl_preread` for passthrough
routing. Use Nginx termination plus the native module when HTTP fields and
operational integration matter more than raw ClientHello data.

## Regression testing

Store a baseline containing the client, library/browser version, Nginx/OpenSSL
version and observation date. Re-run after upgrades and review drift instead
of treating every change as abuse.

## Protocol references

- [RFC 6066 — SNI](https://www.rfc-editor.org/rfc/rfc6066)
- [RFC 7301 — ALPN](https://www.rfc-editor.org/rfc/rfc7301)
- [RFC 9113 — HTTP/2](https://www.rfc-editor.org/rfc/rfc9113)
- [RFC 9110 — HTTP semantics](https://www.rfc-editor.org/rfc/rfc9110)
- [RFC 7239 — Forwarded](https://www.rfc-editor.org/rfc/rfc7239)
- [RFC 9846 — TLS 1.3](https://www.rfc-editor.org/rfc/rfc9846)
- [RFC 8446 — original TLS 1.3](https://www.rfc-editor.org/rfc/rfc8446)
- [RFC 9849 — Encrypted ClientHello](https://www.rfc-editor.org/rfc/rfc9849)

For the meaning and limitations of JA3, JA3S, JA4 and JA4H, see
[`WIRE_OBSERVER.md`](WIRE_OBSERVER.md).
