# Практические сценарии httpfin

httpfin полезен как диагностический стенд и как пассивный observability-слой. Он не должен принимать решение «это точно бот» по одному отпечатку: fingerprint — сигнал, который лучше объединять с частотой запросов, auth-событиями, IP/ASN и поведением сессии.

## 1. Отладка собственного API-клиента

Цель — понять, чем реально отличается клиент от браузера или эталонного `curl-impersonate`.

Запустить httpfin:

```bash
go run ./cmd/httpfin
```

Отправить запросы:

```bash
curl -v http://127.0.0.1:8080/diagnostic
curl_chrome116 -v http://127.0.0.1:8080/diagnostic
```

Для своего Go-клиента направить запрос на тот же URL и сравнить в UI:

- `User-Agent` и его соответствие заявленному профилю;
- HTTP/1.1 против HTTP/2;
- TLS version и ALPN, если запрос идёт через HTTPS listener;
- latency, status и bytes в Squid-style отчёте.

Это полезно при миграции `net/http` на другой transport, включении HTTP/2, смене прокси или TLS-библиотеки.

## 2. Проверка входящих клиентов на API

httpfin можно поставить перед тестовым API как отдельный ingress/sidecar или встроить middleware в Go-сервис. Для каждого запроса сохраняются timestamp, elapsed, client IP, method, URL, status, bytes, protocol, User-Agent и профиль клиента.

Минимальный тестовый маршрут:

```go
handler := httpfin.NewTLSHandler(apiHandler, store, tlsinspect.NewObserver())
http.ListenAndServeTLS(":8443", "server.crt", "server.key", handler)
```

В production лучше не смешивать debug-панель и боевую auth-зону. Отправляйте в httpfin только нужные поля, маскируйте query-параметры с секретами и задавайте короткое retention window.

## 3. Проверка заявленного браузера

Классический сигнал аномалии — User-Agent говорит «Chrome», а TLS/ALPN/HTTP/2 похожи на generic HTTP client. Это не повод блокировать запрос автоматически, но хороший повод пометить его как `Review` и проверить endpoint, rate и auth-контекст.

Сценарий:

1. Отправить обычный Python `requests` с User-Agent Chrome.
2. Отправить `curl_chrome116`.
3. Сравнить профиль, JA3/JA4, ALPN, HTTP/2 и UA consistency.
4. Проверить, одинаково ли ведут себя `/login`, `/token`, `/download` и чувствительные API-методы.

Такой подход соответствует практикам anti-bot: сначала использовать пассивные network signals, а browser-side telemetry подключать только когда это необходимо.

## 4. Эталонные тесты с curl-impersonate

`curl-impersonate` поддерживает обычные curl-флаги и browser profiles. Для регрессии удобно хранить пары `profile → expected signals`:

```bash
curl_chrome116 --compressed -v https://inspector.example.test/fixture
curl_ff117 --compressed -v https://inspector.example.test/fixture
curl_safari15_5 --compressed -v https://inspector.example.test/fixture
```

После обновления Nginx, OpenSSL, HTTP/2 proxy или `curl-impersonate` сравнить экспорт `/api/report`. Изменение fingerprint не всегда означает поломку, но его нужно зафиксировать как изменение сетевого поведения.

Важно: baseline привязывается к версии. Храним `profile`, `browser_version`, `curl_impersonate_version`, `checked_at`, `source` и `status`. Новый Chrome или новый wrapper может законно изменить JA3/JA4, поэтому сравнение должно показывать `drifted`, а не автоматически объявлять клиента подозрительным.

## 5. Nginx reverse proxy и потеря данных

Если Nginx терминирует TLS, backend видит уже расшифрованный HTTP. Поэтому access log передаёт полезные request-level поля, но не весь исходный ClientHello.

Для обычной диагностики включить `configs/nginx/httpfin.conf` и проверить:

- `$remote_addr` после real IP/proxy protocol настройки;
- `$request_time`, `$status`, `$bytes_sent`;
- `$server_protocol`, `$ssl_protocol`, `$ssl_alpn_protocol`;
- `$http_user_agent` и request ID.

Если нужен SNI/ALPN до TLS termination, смотреть в сторону `stream_ssl_preread`. Встроенный TLS listener httpfin использует Go `GetConfigForClient` и сохраняет SNI, предложенный ALPN и стабильную подпись наблюдаемых ClientHello-параметров. Для строгого JA3/JA4 нужен отдельный parser всех полей ClientHello; один access log Nginx для этого недостаточен.

### Что происходит при TLS и SNI

1. Клиент сначала отправляет `ClientHello`, где может быть SNI и список ALPN.
2. В режиме `http { listen 443 ssl; }` Nginx принимает ClientHello, выбирает сертификат/виртуальный server, завершает TLS и затем видит HTTP.
3. В режиме `stream { ssl_preread on; }` Nginx читает ClientHello без расшифровки и может маршрутизировать по SNI/ALPN, но HTTP method, URL, status и User-Agent недоступны.
4. После TLS termination исходный ClientHello уже не передаётся в upstream автоматически. Для JA3/JA4 нужен отдельный сборщик или parser до termination. Текущий Nginx-модуль передаёт TLS version, SNI и ALPN, но не выдаёт JA3/JA4 из HTTP-фазы.

Для проверки встроенного TLS listener:

```bash
httpfin_tls_cert=./server.crt httpfin_tls_key=./server.key go run ./cmd/httpfin
curl -k --resolve api.example.test:8443:127.0.0.1 https://api.example.test:8443/fixture
curl -s http://127.0.0.1:8080/api/report
```

В отчёте должны появиться `sni=api.example.test`, `tls=TLS 1.3`, `alpn=h2` или `http/1.1`, а также `offered_alpn` и `hello`. Если SNI отсутствует, это фиксируется как пустое значение — нельзя подставлять Host header и выдавать его за TLS SNI.

SNI — это имя, предложенное клиентом для выбора сертификата. Оно не доказывает, что клиент действительно принадлежит этому домену. `Forwarded`/`X-Forwarded-For` также нельзя принимать безусловно: доверять им следует только от заранее известных proxy hop.

Готовый пример preread-режима находится в [`configs/nginx/httpfin-stream.conf`](../configs/nginx/httpfin-stream.conf). Он логирует SNI, ALPN и stream-level bytes/session time, но намеренно не обещает HTTP-level fingerprint.

## 6. Анализ отправок и проблем с прокси

httpfin можно использовать как controlled endpoint для исходящих отправок:

```bash
httpfin_addr=:8080 go run ./cmd/httpfin

# в другом окне
curl -H 'X-Test-Case: plain-curl' http://127.0.0.1:8080/echo
curl_chrome116 -H 'X-Test-Case: chrome-profile' http://127.0.0.1:8080/echo
```

Для каждого варианта сравниваются HTTP protocol, User-Agent, response status, latency и размер ответа. Это помогает понять, где меняется запрос: в SDK, прокси, ingress, TLS termination или upstream.

## 7. Incident triage

Отчёт удобно использовать как временную выборку:

```bash
curl -sS http://127.0.0.1:8080/api/report > incident-2026-11-25.txt
```

В отчёте искать:

- один и тот же JA4/HTTP2 fingerprint на множестве IP;
- профиль Chrome-like при `http/1.1` и противоречивом UA;
- рост 401/403/429 по одному fingerprint;
- скачки latency, размера ответа или URL-паттернов;
- повторяющиеся POST на login/token/reset.

## 8. Как не ошибиться с интерпретацией

- Сигнатуры могут совпадать у многих клиентов и могут меняться после обновления браузера.
- Прокси и TLS termination могут скрыть или изменить признаки исходного клиента.
- IP не равен устройству: NAT, VPN и мобильные сети меняют картину.
- Не храните raw fingerprint дольше, чем нужно расследованию; для anti-bot лучше хешировать/усекать значения и ограничивать срок хранения.
- Для high-value операций используйте auth, session binding и rate limits, а не только fingerprint.

## 9. Нормативная модель протокола

В реализации разделяем стандартизированные наблюдения и эвристические fingerprints:

- TLS ClientHello и negotiated version — модель TLS 1.3; в текущем RFC Editor актуальная редакция TLS 1.3 — [RFC 9846](https://www.rfc-editor.org/rfc/rfc9846), которая обновляет RFC 8446.
- ALPN — [RFC 7301](https://www.rfc-editor.org/rfc/rfc7301). Значение `h2` означает выбор HTTP/2, но само по себе не доказывает браузер.
- HTTP/2 connection и binary framing — [RFC 9113](https://www.rfc-editor.org/rfc/rfc9113). SETTINGS, порядок кадров и приоритеты могут быть полезными признаками, но это уже наблюдаемая реализация клиента, а не отдельный RFC fingerprint.
- HTTP semantics — [RFC 9110](https://www.rfc-editor.org/rfc/rfc9110): метод, target, status и поля запроса должны храниться отдельно от TLS-признаков.
- Цепочка прокси — [RFC 7239](https://www.rfc-editor.org/rfc/rfc7239) (`Forwarded`). Доверять `Forwarded` или `X-Forwarded-For` можно только от явно доверенного proxy hop; иначе клиент может подделать исходный IP.
- SNI — [RFC 6066](https://www.rfc-editor.org/rfc/rfc6066), а ограничения видимости имени при ECH — [RFC 9849](https://www.rfc-editor.org/rfc/rfc9849).

JA3/JA4 не являются заменой RFC-модели и не должны называться «идентификатором клиента». В коде это отражается отдельными полями `JA3`, `JA4`, `Reference*` и `BaselineStatus`.

## Источники и техническая база

- [curl-impersonate README](https://github.com/lwthiker/curl-impersonate) — browser-like TLS/HTTP handshakes и профили;
- [curl-impersonate Quick Start](https://curl-impersonate.readthedocs.io/en/latest/quick_start.html) — запуск wrapper profiles;
- [NGINX log module](https://nginx.org/en/docs/http/ngx_http_log_module.html) — `log_format`, JSON escaping и request timing;
- [NGINX ssl_preread](https://nginx.org/en/docs/stream/ngx_stream_ssl_preread_module.html) — SNI/ALPN из ClientHello без termination;
- [OWASP Bot Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Bot_Management_and_Anti-Automation_Cheat_Sheet.html) — passive network signals и privacy guidance;
- [AWS client identification guidance](https://docs.aws.amazon.com/prescriptive-guidance/latest/bot-control/client-identification-controls.html) — практические сценарии TLS/device fingerprinting;
- [RFC 8446](https://www.rfc-editor.org/rfc/rfc8446) — исходная спецификация TLS 1.3, заменённая RFC 9846;
- [RFC 9849](https://www.rfc-editor.org/rfc/rfc9849) — Encrypted ClientHello;
- [`ARCHITECTURE.ru.md`](ARCHITECTURE.ru.md) — расшифровка JA3/JA3S/JA4/JA4H и диаграммы границ наблюдения;
- [Squid logformat](https://www.squid-cache.org/Doc/config/logformat/) — поля и модель access report.
