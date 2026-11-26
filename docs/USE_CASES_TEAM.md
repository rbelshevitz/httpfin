# How to use httpfin

Place `httpfin` in front of a test or production API. It does not replace the
API, modify requests, or block clients. Its job is to show what reached the
entry point: HTTP fields, User-Agent, available TLS signals, and the request
result.

## Main workflow

```text
client → Nginx with httpfin → API under test
                         └──→ httpfin report
```

In the recommended mode, Nginx terminates TLS and the module sends an event to
the Go collector:

1. send the same request through the Nginx entry point;
2. verify the API result: status, body, and data changes;
3. find the request in `/api/connections`;
4. save `/api/report` with the test result or defect.

```bash
curl -fsS https://api.example.test/health
curl -fsS https://api.example.test/health \
  -H 'User-Agent: QA-client/1.0'
curl -fsS http://127.0.0.1:8080/api/report > httpfin-report.txt
```

The first two requests go to the API under test. The last request goes to the
`httpfin` collector and saves its report. `/healthz`, `/api/connections`, and
`/api/report` belong to `httpfin`, not to the product API.

## What each team checks

### DevOps and SRE

After changing Nginx, TLS, proxy settings, real-IP handling, or an upstream,
compare a direct request with one through the entry point:

- client address;
- Host and SNI;
- ALPN and HTTP version;
- negotiated TLS version;
- response status;
- the event in the report and access log.

The Nginx event includes request time and response size when the corresponding
Nginx variables are available; compare them with the access log when diagnosing
an upstream or logging discrepancy.

This helps find lost client addresses, wrong virtual hosts, HTTP/2 to HTTP/1.1
changes, and latency introduced between the edge and the application.

### API QA

Run the same API test with each relevant client: desktop browser, mobile
browser, WebView, SDK, and a generic HTTP client. Compare in the report:

- User-Agent and the detected profile;
- method, path, and status;
- protocol, TLS, SNI, and ALPN when available;
- changes between sequential observations;
- absence of secrets in the saved report.

Add timing and response size from the access log when performance matters.

If the API response changed, compare the client reports first. If only a
transport signal changed, check the browser, SDK, proxy, or TLS-library update;
that signal alone does not prove a client defect.

### API and SDK developers

When two clients receive different results, send both through the same
observation point and compare method, URL, Host, User-Agent, protocol, TLS,
timing, status, and response size. Take timing and size from the access log or
direct Go observation. This helps separate application defects from client,
proxy, or Nginx differences.

## Reading the report

The report shows:

- which clients were observed and how many requests they made;
- the latest User-Agent profile;
- available transport signals;
- changes from the previous observation for the same address;
- whether a baseline is available and needs review.

`Unknown`, `unknown`, and `unavailable` mean that evidence is missing. They do
not mean a mismatch. User-Agent and profiles can be spoofed, so the report is
diagnostic evidence, not proof of client identity.

## Choosing a mode

| Scenario | What httpfin sees | Use it when |
|---|---|---|
| Nginx terminates TLS | HTTP, User-Agent, status, and available TLS fields | Main deployment in front of an API |
| `httpfin` accepts HTTPS directly | HTTP, SNI, and ALPN after terminating TLS | Debugging TLS without Nginx |
| Nginx `stream` with `ssl_preread` | ClientHello routing signals | Passthrough only; no HTTP report |

With TLS passthrough, `httpfin` cannot see the URL, headers, User-Agent, or
response status. In the ordinary Nginx HTTP module, the original ClientHello is
already unavailable, so JA3/JA4 and connection-level HTTP/2 fingerprints do not
appear automatically.

## What not to assume

- User-Agent or a profile is proof of a browser;
- missing JA3/JA4 is an error in the ordinary mode;
- the report is a complete request log: observations are kept in memory;
- `httpfin` blocks requests or controls the firewall;
- `httpfin` service endpoints are not the same as the API under test.

See [`OPERATING_MODES.md`](OPERATING_MODES.md) for TLS, certificates, and Nginx
deployment. See [`INTEGRATION_FORMAT.md`](INTEGRATION_FORMAT.md) for the event
and report format.
