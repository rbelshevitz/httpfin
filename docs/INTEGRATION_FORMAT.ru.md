# Формат интеграции httpfin и Nginx-модуля

## Ответственность компонентов

```text
Client
  │ TLS + HTTP
  ▼
Nginx ── TLS termination, SNI/ALPN, proxy, access log
  │
  ├─ ngx_http_httpfin_module.so
  │     request id, bounded event, non-blocking Unix datagram
  │
  └─ /run/httpfin/events.sock
          │ best-effort JSON
          ▼
      Go httpfin
        │ normalize, profile, baseline, store, report, UI/API
```

### Nginx

Nginx остаётся data-plane:

- принимает или пропускает TLS;
- выбирает виртуальный сервер по SNI;
- согласует ALPN;
- проксирует HTTP и пишет access log;
- передаёт компактное событие в локальный Unix socket.

Nginx-модуль не должен хранить историю, считать JA3/JA4, обращаться к базе,
делать DNS/HTTP-запросы или ждать Go collector.

### Модуль

Модуль — тонкий адаптер границы:

- создаёт request ID;
- на `POST_READ` включает наблюдение запроса;
- на `LOG` формирует JSON размером до 4096 байт;
- отправляет его через `SOCK_DGRAM|O_NONBLOCK`;
- при недоступном collector отбрасывает событие и не задерживает worker.

Это best-effort telemetry, а не lossless-аудит.

## Зачем нужен модуль и обязателен ли Go

Короткий ответ для API-разработчика: обычно запускаются два процесса — Nginx
и `httpfin`. Nginx принимает запрос, а `httpfin` показывает результат через
UI, JSON API и TXT-отчёт. Модуль связывает эти два процесса.

Модуль нужен, когда Nginx уже принимает TLS и проксирует запросы, но команде
нужен структурированный handoff, более точный и быстрый для анализа, чем
разбор access log. Он добавляет request ID, выбранные HTTP-поля, TLS-переменные
и статус доставки, не перенося storage и анализ внутрь Nginx worker.

Для интеграции через native-модуль Go-исполняемый файл нужен:

```text
Nginx worker
  └─ ngx_http_httpfin_module.so
       └─ ограниченный Unix datagram
            └─ исполняемый файл httpfin на Go
                 ├─ in-memory store
                 ├─ profile/risk/baseline processing
                 ├─ /api/connections и /api/report
                 └─ inspector UI
```

Один `.so` не умеет хранить события, строить отчёты, отдавать API или UI и не
содержит надёжной очереди. Если Go-приёмник не запущен, модуль помечает событие
как недоступное или отбрасывает его, а обработка запроса в Nginx продолжается.
Если нужны только access logs, модуль не подключайте. Если достаточно прямого
наблюдения HTTPS, запускайте Go без Nginx. Пара «модуль + Go» нужна для
production-сценария с Nginx termination и структурированным анализом.

### Кто к кому подключается

Связка односторонняя и локальная:

1. Go при старте создаёт Unix datagram socket `/run/httpfin/events.sock` и
   начинает его слушать.
2. Nginx worker при запуске подключает свой `SOCK_DGRAM` к этому пути.
3. После каждого обработанного HTTP-запроса модуль делает `send()` с одним
   JSON-событием.
4. Go читает datagram, преобразует JSON в запись `Connection` и добавляет её
   в store.

Между Nginx и Go нет HTTP-запроса, TCP-порта или двустороннего RPC. Go не
подключается к модулю: Go слушает socket, а модуль отправляет в него данные.
Если Go ещё не запущен или socket недоступен, Nginx продолжает обслуживать
запросы, но событие теряется.

Не нужно отдельно вызывать `.so`: Nginx загружает его через `load_module`.
Не нужно писать собственный Unix-клиент: `httpfin` сам создаёт и слушает
сокет. В Docker оба процесса запускает один entrypoint.

### Go/httpfin

Go-процесс владеет семантикой продукта:

- принимает и нормализует события;
- хранит историю;
- строит профиль, risk и baseline;
- выдаёт `/api/connections`, `/api/report` и UI;
- должен быть заменяемым без пересборки Nginx-модуля.

## Что записывается

### 1. Nginx access log

Это операционный источник истины по каждому обработанному запросу:

```json
{"request_id":"15-0","handoff":"not_sent","remote_addr":"172.17.0.1","request":"GET /healthz HTTP/2.0","status":204,"request_time":0.001,"sni":"inspector.example.test","alpn":"h2"}
```

`handoff=not_sent` в этой же строке не обязательно означает потерю. Встроенный
Nginx log handler может вычислить переменные раньше обработчика модуля в
`LOG_PHASE`. Поэтому доставку сейчас проверяют по Go API и сопоставлением с
access log, а не по одному полю этой строки. Отдельные счётчики потерь пока не
включены в runtime.

### 2. Unix datagram от модуля

Модуль отправляет компактное событие:

```json
{"version":1,"request_id":"15-0","method":"GET","uri":"/healthz","host":"inspector.example.test","user_agent":"curl/8.10.1","remote_addr":"172.17.0.1","status":204,"tls_version":"TLS 1.3","sni":"inspector.example.test","alpn":"h2"}
```

Это источник данных для `/api/connections`. Поля TLS могут быть пустыми, если
Nginx не завершал TLS, модуль включён в plain HTTP server или соответствующая
переменная недоступна.

### 3. Go API

После нормализации событие выглядит так:

```json
{
  "id": "15-0",
  "client_ip": "172.17.0.1",
  "method": "GET",
  "url": "/healthz",
  "status": 204,
  "protocol": "HTTP",
  "tls_version": "TLS 1.3",
  "sni": "inspector.example.test",
  "alpn": "h2",
  "detected_profile": "Generic HTTP",
  "ua_consistency": "Unknown",
  "risk": "Info"
}
```

Профиль Nginx handoff определяется по заявленному User-Agent и потому остаётся
эвристикой. Если User-Agent отсутствует или неизвестен, будет `Unknown`. JA3/JA4
и HTTP/2 fingerprint требуют отдельного наблюдения ClientHello и HTTP/2
connection frames; их нельзя честно восстановить из одного HTTP request event.

### 4. Squid-style отчёт

`GET /api/report` объединяет operational summary и fingerprint details. В отчёте
есть отдельная сводка клиентов: число запросов, последний профиль и сравнение
с baseline. Для каждого
наблюдения показывается изменение TLS/ALPN/ClientHello/JA3/JA4/HTTP2 относительно
предыдущего события того же client IP. Рядом указываются точка наблюдения и
состояние evidence (`http-only`, `transport-only`, `partial-wire`). При отсутствии предыдущего события или
данных выводится `unknown`, а не mismatch. Секретные query-параметры
редактируются на границе отчёта:

```text
httpfin CLIENT FINGERPRINT INSPECTOR REPORT
Evidence policy: missing fields are unknown; fingerprints are signals, not identity proof
Observations: 1

CLIENTS
CLIENT                       REQUESTS LATEST PROFILE     BASELINE       EVIDENCE
172.17.0.1                   1        Unknown            unavailable    http-only

ACCESS SUMMARY
TIME                 MS     CLIENT                       ST     BYTES    METHOD           PROFILE        RISK
2026-11-25 15:43:34  0      172.17.0.1                  204    0        GET /healthz     Unknown        Info

OBSERVATION DETAILS
[2026-11-25T15:43:34Z] id=15-0 client=172.17.0.1 request=GET /healthz status=204 bytes=0 duration_ms=0
  observation: point=nginx-http-module evidence=transport-only
  transport: protocol=HTTP tls=TLS 1.3 sni=inspector.example.test alpn=h2 offered_alpn=unknown
  fingerprints: client_hello=unknown ja3=unknown ja4=unknown http2=unknown
  fingerprint_change: no_previous_observation
```

Для отчёта важно разделять факт доставки события и наличие fingerprint:
доставка события ещё не означает, что доступен raw ClientHello.

Отчёт создаётся синхронно при обработке `GET /api/report` из текущего
хранилища в памяти. Сам сервис не создаёт файл в каталоге и не запускает
фоновый exporter. Файл появляется только если клиент сохраняет HTTP-ответ,
например через `> httpfin-report.txt`. После перезапуска runtime-наблюдения
исчезают; долговременное хранение относится к access log или внешнему storage.

## TLS и сертификаты

### Nginx termination

Сертификат и ключ лежат на Nginx. Go-серверу сертификат не нужен.

```nginx
ssl_certificate     /etc/nginx/tls/api.fullchain.pem;
ssl_certificate_key /etc/nginx/tls/api.key;
```

В контейнере монтируйте каталог read-only или secret. Ключ не должен попадать
в образ и не должен передаваться Go collector без необходимости.

### Прямой HTTPS listener Go

Сертификат нужен самому `httpfin`:

```bash
httpfin_tls_cert=/etc/httpfin/tls/server.crt \
httpfin_tls_key=/etc/httpfin/tls/server.key \
httpfin_tls_addr=:8443 \
./httpfin
```

### TLS passthrough

Nginx с `stream.ssl_preread on` сертификат не использует. Он принадлежит
конечному TLS backend. Nginx видит SNI/ALPN, но не HTTP method, URL и headers.

## Сборка модуля вне Docker

### Правило совместимости

`.so` нужно собирать под тот же Nginx, который будет его загружать:

1. та же версия Nginx;
2. та же ОС и libc, например Alpine/musl или Debian/glibc;
3. максимально близкие `./configure` flags;
4. `--with-compat` включён и при сборке Nginx, и при сборке модуля;
5. после установки обязательно `nginx -t`.

`--with-compat` не превращает модуль в универсальный бинарник и не исправляет
несовместимость musl/glibc.

### Alpine

Для Alpine 3.23 пакетная ветка содержит Nginx 1.28.3 и пакет `nginx-mod-dev`.
Набор для сборки:

```bash
apk add nginx nginx-mod-dev \
  build-base linux-headers \
  openssl-dev pcre2-dev zlib-dev
```

Если доступен source tree ровно той же версии:

```bash
cd /path/to/nginx-1.28.3
./configure --with-compat \
  --with-http_ssl_module \
  --with-http_v2_module \
  --with-stream \
  --with-stream_ssl_preread_module \
  --add-dynamic-module=/path/to/httpfin/nginx-module
make modules
install -D -m 0644 objs/ngx_http_httpfin_module.so \
  /usr/lib/nginx/modules/ngx_http_httpfin_module.so
nginx -t
```

### Debian/Ubuntu/RHEL-like host

Сначала сохраните конфигурацию установленного бинарника:

```bash
nginx -V 2>&1 | tee /tmp/nginx-build.txt
```

Затем получите исходники ровно той же версии и установите development-пакеты
своего дистрибутива: compiler, `pcre2-dev`/`libpcre2-dev`, `zlib-dev` или
`zlib1g-dev`, `openssl-dev` или `libssl-dev` и headers Nginx. Повторите
`./configure` с теми же существенными flags и добавьте:

```bash
--with-compat --add-dynamic-module=/path/to/httpfin/nginx-module
```

Соберите `make modules`, скопируйте `.so` в каталог модулей конкретной
установки и проверьте:

```bash
nginx -t
systemctl reload nginx
```

Не используйте `.so`, собранный в Alpine, на glibc-хосте или наоборот.

## Проверка стабильности

Перед rollout:

```bash
go test ./...
go vet ./...
./tests/nginx-module-static.sh
./tests/container-smoke.sh
```

Контейнерный smoke дополнительно проверяет наличие runtime `libssl`,
`libcrypto`, PCRE2 и zlib у Nginx, отсутствие прямой криптозависимости у
модуля, загрузку `.so`, TLS 1.3, SNI, ALPN и доставку события в Go API.

## Нормативные и практические источники

- [RFC 9846 — TLS 1.3](https://www.rfc-editor.org/rfc/rfc9846) — актуальная редакция TLS 1.3;
- [RFC 8446](https://www.rfc-editor.org/rfc/rfc8446) — исходная спецификация TLS 1.3;
- [RFC 6066](https://www.rfc-editor.org/rfc/rfc6066) — SNI в TLS ClientHello;
- [RFC 7301](https://www.rfc-editor.org/rfc/rfc7301) — ALPN;
- [RFC 9113](https://www.rfc-editor.org/rfc/rfc9113) — HTTP/2;
- [RFC 9110](https://www.rfc-editor.org/rfc/rfc9110) — HTTP semantics;
- [RFC 7239](https://www.rfc-editor.org/rfc/rfc7239) — `Forwarded` через proxy;
- [RFC 9849](https://www.rfc-editor.org/rfc/rfc9849) — Encrypted ClientHello;
- [NGINX configure](https://nginx.org/en/docs/configure.html) — сборка и `--with-compat`;
- [NGINX dynamic modules](https://docs.nginx.com/nginx/admin-guide/dynamic-modules/) — загрузка `.so`;
- [NGINX HTTP log module](https://nginx.org/en/docs/http/ngx_http_log_module.html) — `log_format` и access log;
- [NGINX SSL preread](https://nginx.org/en/docs/stream/ngx_stream_ssl_preread_module.html) — SNI/ALPN без termination;
- [curl-impersonate](https://github.com/lwthiker/curl-impersonate) и [Quick Start](https://curl-impersonate.readthedocs.io/en/latest/quick_start.html) — эталонные browser-like запросы;
- [Squid logformat](https://www.squid-cache.org/Doc/config/logformat/) — модель access-отчёта.
