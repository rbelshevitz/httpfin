# Практические сценарии httpfin

httpfin полезен как диагностический стенд и как пассивный слой наблюдения. Он
помогает сравнивать сетевое поведение клиентов, прокси и API-маршрутов, но не
удостоверяет личность клиента и не применяет правила доступа.

## Быстрая проверка для QA

1. Открыть `/healthz`; ожидаемый ответ — `204`.
2. Выполнить согласованный запрос через входную точку приложения.
3. Проверить ответ приложения и найти наблюдение в `/api/connections`.
4. Сохранить `/api/report` вместе с результатом теста. Если нужно проверить
   доставку события до сборщика, запросить `/api/ingest`.

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

httpfin можно поставить перед тестовым API как отдельный ingress/sidecar или
встроить как middleware в Go-сервис.

Для каждого запроса сохраняются timestamp, elapsed, client IP, method, URL,
status, bytes, protocol, User-Agent и профиль клиента.

Минимальный тестовый маршрут:

```go
handler := httpfin.NewTLSHandler(apiHandler, store, tlsinspect.NewObserver())
http.ListenAndServeTLS(":8443", "server.crt", "server.key", handler)
```

В production не смешивайте debug-панель с боевой auth-зоной. Передавайте в
httpfin только нужные поля, маскируйте секреты в query-параметрах и задавайте
короткий срок хранения.

## 3. Проверка заявленного браузера

Сравнение User-Agent с TLS, ALPN и HTTP/2 помогает заметить изменение сетевого
поведения клиента и проверить его по журналам API.

Сценарий:

1. Отправить обычный Python `requests` с User-Agent Chrome.
2. Отправить `curl_chrome116`.
3. Сравнить профиль, ALPN, HTTP/2 и UA consistency. JA3/JA4 сравнивать только
   если в тесте подключён отдельный наблюдатель ClientHello.
4. Проверить, одинаково ли ведут себя `/login`, `/token`, `/download` и чувствительные API-методы.


## 4. Эталонные тесты с curl-impersonate

`curl-impersonate` поддерживает обычные curl-флаги и browser profiles. Для регрессии удобно хранить пары `profile → expected signals`:

```bash
curl_chrome116 --compressed -v https://inspector.example.test/fixture
curl_ff117 --compressed -v https://inspector.example.test/fixture
curl_safari15_5 --compressed -v https://inspector.example.test/fixture
```

После обновления Nginx, OpenSSL, HTTP/2 proxy или `curl-impersonate` сравните
экспорт `/api/report`. Изменение fingerprint не всегда означает поломку, но
его следует зафиксировать как изменение сетевого поведения.

Важно: baseline привязывается к версии. Новый Chrome или новый wrapper может
законно изменить JA3/JA4, поэтому сравнение показывает `mismatch` или `unknown`
как изменение данных, а не как вердикт о клиенте.

## 5. Nginx reverse proxy и потеря данных

Если Nginx терминирует TLS, backend видит уже расшифрованный HTTP. Поэтому
access log передаёт полезные request-level поля, но не весь исходный
ClientHello.

Для обычной диагностики включить `configs/nginx/httpfin.conf` и проверить:

- `$remote_addr` после real IP/proxy protocol настройки;
- `$request_time`, `$status`, `$bytes_sent`;
- `$server_protocol`, `$ssl_protocol`, `$ssl_alpn_protocol`;
- `$http_user_agent` и request ID.

Если SNI или ALPN нужны до TLS termination, используйте
`stream_ssl_preread`.

Встроенный TLS listener httpfin использует Go `GetConfigForClient` и сохраняет
SNI, предложенный ALPN и стабильную подпись наблюдаемых параметров
ClientHello.

Для строгого JA3/JA4 нужен отдельный parser всех полей ClientHello. Одного
access log Nginx для этого недостаточно.

### Что происходит при TLS и SNI

1. Клиент сначала отправляет `ClientHello`, где может быть SNI и список ALPN.
2. В режиме `http { listen 443 ssl; }` Nginx принимает ClientHello, выбирает
   сертификат и виртуальный server, завершает TLS, а затем видит HTTP.
3. В режиме `stream { ssl_preread on; }` Nginx читает ClientHello без
   расшифровки и может маршрутизировать по SNI/ALPN. HTTP method, URL, status и
   User-Agent в этом режиме недоступны.
4. После TLS termination исходный ClientHello уже не передаётся в upstream
   автоматически. Для JA3/JA4 нужен отдельный сборщик до termination. Текущий
   Nginx-модуль передаёт TLS version, SNI и ALPN, но не выдаёт JA3/JA4 из
   HTTP-фазы.

Для проверки встроенного TLS listener:

```bash
httpfin_tls_cert=./server.crt httpfin_tls_key=./server.key go run ./cmd/httpfin
curl -k --resolve api.example.test:8443:127.0.0.1 https://api.example.test:8443/fixture
curl -s http://127.0.0.1:8080/api/report
```

В отчёте должны появиться `sni=api.example.test`, `tls=TLS 1.3`, `alpn=h2`
или `http/1.1`, а также `offered_alpn` и `hello`.

Если SNI отсутствует, это фиксируется как пустое значение. Нельзя подставлять
HTTP `Host` и выдавать его за TLS SNI.

SNI — это имя, предложенное клиентом для выбора сертификата. Оно не доказывает,
что клиент действительно принадлежит этому домену.

`Forwarded` и `X-Forwarded-For` также нельзя принимать безусловно. Доверяйте
им только от заранее известных proxy hop.

Готовый пример preread-режима находится в
[`configs/nginx/httpfin-stream.conf`](../configs/nginx/httpfin-stream.conf).
Он логирует SNI, ALPN и stream-level bytes/session time, но намеренно не
обещает HTTP-level fingerprint.

## 6. Анализ отправок и проблем с прокси

`httpfin` можно временно назначить адресатом исходящего запроса. В этом
сценарии он работает как тестовый приёмник и показывает в отчёте, что реально
отправил клиент: User-Agent, HTTP-версию, TLS/SNI/ALPN, статус и задержку.

Это не proxy и не генератор исходящего трафика.

```bash
httpfin_addr=:8080 go run ./cmd/httpfin

# в другом окне
curl -H 'X-Test-Case: plain-curl' http://127.0.0.1:8080/healthz
curl_chrome116 -H 'X-Test-Case: chrome-profile' http://127.0.0.1:8080/healthz
curl -sS http://127.0.0.1:8080/api/report > outgoing-report.txt
```

Для каждого варианта сравниваются HTTP protocol, User-Agent, response status,
latency и размер ответа. Это помогает понять, где меняется запрос: в SDK,
прокси, ingress, TLS termination или upstream. Для проверки TLS/SNI направьте
те же запросы на HTTPS listener и используйте `--resolve`.

## 7. Разбор сетевого инцидента

Отчёт удобно использовать как временную выборку:

```bash
curl -sS http://127.0.0.1:8080/api/report > incident-2026-11-25.txt
```

В отчёте искать:

- изменение TLS, SNI, ALPN или HTTP-версии после обновления;
- неожиданный профиль User-Agent или изменение его соответствия baseline;
- потерю исходного IP из-за proxy или real-IP-настройки;
- скачки задержки, размера ответа, статуса или URL;
- различия между браузером, SDK, прокси и прямым подключением.

## 8. Как не ошибиться с интерпретацией

- Сигнатуры могут совпадать у многих клиентов и могут меняться после обновления браузера.
- Прокси и TLS termination могут скрыть или изменить признаки исходного клиента.
- IP не равен устройству: NAT, VPN и мобильные сети меняют картину.
- Не храните raw fingerprint дольше, чем нужно расследованию; ограничивайте срок
  хранения и объём данных.
- Для проверки доступа используйте штатные механизмы API: аутентификацию,
  права и журнал приложения. `httpfin` предназначен для диагностики транспорта.

## 9. Нормативная модель протокола

В реализации разделяем стандартизированные наблюдения и эвристические fingerprints:

- TLS ClientHello и negotiated version — модель TLS 1.3. В текущем RFC Editor
  актуальная редакция TLS 1.3 — [RFC 9846](https://www.rfc-editor.org/rfc/rfc9846),
  которая обновляет RFC 8446.
- ALPN — [RFC 7301](https://www.rfc-editor.org/rfc/rfc7301). Значение `h2`
  означает выбор HTTP/2, но само по себе не доказывает, что клиент — браузер.
- HTTP/2 connection и binary framing — [RFC 9113](https://www.rfc-editor.org/rfc/rfc9113).
  SETTINGS, порядок кадров и приоритеты могут быть полезными признаками. Это
  наблюдаемая реализация клиента, а не отдельный RFC fingerprint.
- HTTP semantics — [RFC 9110](https://www.rfc-editor.org/rfc/rfc9110): метод,
  target, status и поля запроса должны храниться отдельно от TLS-признаков.
- Цепочка прокси — [RFC 7239](https://www.rfc-editor.org/rfc/rfc7239)
  (`Forwarded`). Доверять `Forwarded` или `X-Forwarded-For` можно только от
  явно доверенного proxy hop; иначе клиент может подделать исходный IP.
- SNI — [RFC 6066](https://www.rfc-editor.org/rfc/rfc6066), а ограничения видимости имени при ECH — [RFC 9849](https://www.rfc-editor.org/rfc/rfc9849).

JA3/JA4 не заменяют RFC-модель и не должны называться «идентификатором
клиента». В коде это отражается отдельными полями `JA3`, `JA4`, `Reference*` и
`BaselineStatus`.

## Источники и техническая база

- [curl-impersonate README](https://github.com/lwthiker/curl-impersonate) — browser-like TLS/HTTP handshakes и профили;
- [curl-impersonate Quick Start](https://curl-impersonate.readthedocs.io/en/latest/quick_start.html) — запуск wrapper profiles;
- [NGINX log module](https://nginx.org/en/docs/http/ngx_http_log_module.html) — `log_format`, JSON escaping и request timing;
- [NGINX ssl_preread](https://nginx.org/en/docs/stream/ngx_stream_ssl_preread_module.html) — SNI/ALPN из ClientHello без termination;
- [RFC 8446](https://www.rfc-editor.org/rfc/rfc8446) — исходная спецификация TLS 1.3, заменённая RFC 9846;
- [RFC 9849](https://www.rfc-editor.org/rfc/rfc9849) — Encrypted ClientHello;
- [`WIRE_OBSERVER.ru.md`](WIRE_OBSERVER.ru.md) — ограничения анализа ClientHello и HTTP/2;
- [Squid logformat](https://www.squid-cache.org/Doc/config/logformat/) — поля и модель access report.
