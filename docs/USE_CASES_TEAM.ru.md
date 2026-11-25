# Как команды используют httpfin

httpfin — не «ещё один логгер», а контрольная точка для ответа на вопрос: что реально пришло по сети, каким клиентом, через какой TLS/HTTP2 путь и с каким результатом.

## DevOps / SRE

### Проверка ingress после изменения Nginx

После изменения TLS, HTTP/2, proxy protocol, real IP или upstream:

1. Запустить httpfin рядом с тестовым ingress.
2. Выполнить одинаковый запрос через Nginx и напрямую в приложение.
3. Сравнить SNI, ALPN, protocol, client IP, latency, status и bytes.
4. Скачать `/api/report` как артефакт изменения.

Проверяемые проблемы:

- пропал исходный IP из-за неверного `real_ip_header`;
- HTTP/2 до Nginx превратился в HTTP/1.1 до backend;
- TLS termination скрыла ClientHello;
- SNI маршрутизирует на неправильный virtual host;
- upstream отвечает дольше, чем показывает внешний мониторинг.

### Канарейка и регрессия

Хранить baseline по версии Nginx/OpenSSL/браузера и сравнивать его после rollout. Нормальный drift должен иметь статус `drifted`, а не автоматически превращаться в блокировку.

## API-тестировщики / QA

### Проверка реального сетевого поведения API

API-тест обычно видит только JSON и HTTP status. httpfin добавляет сетевой контекст:

- какой HTTP protocol был выбран;
- какой ALPN предложил и получил клиент;
- какой SNI пришёл;
- сколько занял запрос и сколько bytes ушло;
- какой User-Agent и профиль клиента определились;
- совпадает ли заявленный browser profile с транспортными сигналами.

Пример smoke-набора:

```bash
curl -sS https://api.example.test/health
curl_chrome116 --compressed https://api.example.test/health
curl_ff117 --compressed https://api.example.test/health
```

Для каждого запуска сохранять `/api/report` с идентификатором теста. Так можно увидеть, что после обновления SDK поменялся не только response body, но и TLS/HTTP2 поведение.

### Негативные тесты

Проверять:

- клиент без SNI;
- клиент с неподдерживаемым ALPN;
- HTTP/1.1 вместо ожидаемого HTTP/2;
- ложный Chrome User-Agent на Python/Go client;
- соединение через доверенный и недоверенный proxy;
- повторные POST на `/login`, `/token`, `/reset`.

## Backend-разработчики

### Отладка SDK и transport

Если один и тот же API-клиент получает разные ответы:

1. направить оба варианта на httpfin;
2. сравнить method, URL, headers, body size, protocol и TLS;
3. проверить, не меняет ли прокси `Host`, `Forwarded`, `User-Agent` или `Accept-Encoding`;
4. сравнить latency и status по попыткам.

Это полезно при переходе между `net/http`, libcurl, gRPC gateway, mobile SDK и service mesh sidecar.

## Security / Anti-bot

### Triage без немедленной блокировки

Сигналы для `Review`:

- Chrome-like User-Agent при generic TLS/HTTP2;
- один fingerprint на множестве IP;
- рост 401/403/429 по одному fingerprint;
- одинаковый fingerprint на login и scraping endpoints;
- неожиданное отсутствие SNI или ALPN.

`httpfin` должен предоставлять данные для расследования, а не единолично
принимать решение о блокировке. OWASP рекомендует начинать с пассивных
сетевых признаков и ограничивать срок хранения fingerprint-данных.
([OWASP Bot Management](https://cheatsheetseries.owasp.org/cheatsheets/Bot_Management_and_Anti-Automation_Cheat_Sheet.html))

### Практический anti-bot workflow

Используйте `httpfin` как точку наблюдения и сравнения, а не как блокировщик:

1. **Собрать:** зафиксировать на edge User-Agent, адрес источника, запрос,
   status, timing, TLS version, SNI и ALPN — только там, где эти поля видны.
2. **Сравнить:** сопоставить заявленный профиль браузера с транспортными
   признаками и baseline, привязанным к версии. User-Agent Chrome при общих
   признаках библиотечного клиента — это сигнал для проверки, а не готовый
   вердикт.
3. **Обогатить:** объединить отчёт со счётчиками rate limit, событиями
   аутентификации, поведением сессии и результатами приложения. Эти данные не
   входят в текущую ответственность модуля и Go collector.
4. **Проверить:** исследовать один fingerprint на множестве адресов, рост
   401/403/429, неожиданный drift профиля и повторяемые шаблоны запросов.
5. **Применить policy отдельно:** WAF, gateway или приложение решает, нужен ли
   challenge, throttle или block. `httpfin` лучше оставлять read-only в пути
   запроса.

Текущий Nginx handoff передаёт request/TLS metadata, но не считает JA3, JA4 и
   HTTP/2 fingerprint. Для этого нужен внешний wire-observer. В Nginx-сценарии
текущий профиль также остаётся `Unknown`; прямой Go HTTPS listener умеет
классифицировать заявленный User-Agent, но это всё равно эвристика.

## Интеграция со Swagger / OpenAPI

OpenAPI описывает API самого httpfin, а не fingerprint клиента. Файл [`docs/openapi.yaml`](openapi.yaml) можно:

- импортировать в Swagger Editor;
- подключить в Insomnia/Postman;
- использовать для генерации typed-клиента;
- добавить в CI smoke-тестов.

Рекомендуемый CI-шаг:

```bash
go run ./cmd/httpfin &
curl --fail http://127.0.0.1:8080/healthz
curl --fail http://127.0.0.1:8080/api/connections
curl --fail http://127.0.0.1:8080/api/report
```

OpenAPI здесь нужен как стабильное описание collector API и отчётности: paths, servers, response schemas и статусы должны меняться контролируемо. ([OpenAPI 3.1](https://spec.openapis.org/oas/v3.1.2.html))

## Nginx: три варианта

### Вариант A — access log + внешний log consumer

Nginx терминирует TLS и пишет `httpfin.jsonl`; отдельный log consumer читает и
агрегирует его. Текущий процесс `httpfin` не читает файл Nginx автоматически.
Это самый простой вариант для существующего ingress: не нужен C ABI, а
обновление Nginx-модуля не входит в rollout. Минус — в access log нет полного
исходного ClientHello.

### Вариант B — TCP passthrough + `ssl_preread`

Nginx не терминирует TLS, а маршрутизирует по SNI/ALPN. Это даёт stream-level SNI/ALPN, но не HTTP method/URL/headers. Конфиг: [`configs/nginx/httpfin-stream.conf`](../configs/nginx/httpfin-stream.conf).

### Вариант C — native Nginx module + Unix datagram

В репозитории этот вариант уже реализован. Он нужен, когда access log
недостаточно и требуется передать в Go структурированное событие сразу из
Nginx: request ID, HTTP-поля, TLS version, SNI и negotiated ALPN.

Модуль предоставляет:

- `httpfin on|off;`;
- `httpfin_socket /run/httpfin/events.sock;`;
- `$httpfin_request_id`;
- `$httpfin_handoff_status`;
- bounded JSON event до 4096 байт;
- non-blocking `SOCK_DGRAM` handoff.

Для передачи событий используется Unix datagram ingest:

```text
Nginx worker → sendmsg(AF_UNIX, SOCK_DGRAM) → /run/httpfin/events.sock → Go Store
```

Событие — версия `1` формата обмена, которое Go нормализует в `Connection`.
Отправка best-effort: модуль не ждёт ответа Go и не делает синхронный network
call в request phase. Если нужен lossless-аудит, сохраняйте access log и
сверяйте его с collector-метриками.

Не стоит помещать в Nginx-модуль SQLite, отчёты, baseline registry или сложный rule engine. Динамические модули загружаются через `load_module`, поэтому нужно собирать модуль совместимо с конкретной версией Nginx. ([NGINX dynamic modules](https://docs.nginx.com/nginx/admin-guide/dynamic-modules/))

Практический выбор:

1. начать с A, если нужен простой ingress observability;
2. добавить B, если нужен TLS passthrough и маршрутизация по SNI/ALPN;
3. использовать C, если нужен единый структурированный handoff в Go или
   request ID на границе Nginx;
4. не переносить fingerprint engine, хранилище и отчёты в C-модуль.
