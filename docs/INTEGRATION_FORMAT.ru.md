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
        │ normalize, store, classify, report, UI/API
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
`LOG_PHASE`. Поэтому доставку проверяют по Go API и collector-метрикам, а не
по одному полю этой строки.

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
  "detected_profile": "Unknown",
  "ua_consistency": "Unknown",
  "risk": "Info"
}
```

Профиль `Unknown` для Nginx handoff — нормальное состояние текущего тонкого
адаптера. JA3/JA4 и HTTP/2 fingerprint требуют отдельного наблюдения ClientHello
и HTTP/2 connection frames; их нельзя честно восстановить из одного HTTP
request event.

### 4. Squid-style отчёт

`GET /api/report` объединяет operational summary и fingerprint details:

```text
httpfin CLIENT FINGERPRINT INSPECTOR REPORT
TIME                 MS     CLIENT                       ST     BYTES    METHOD           PROFILE        RISK
2026-11-25 15:43:34  0      172.17.0.1                  204    0        GET /healthz     Unknown        Info

FINGERPRINT DETAILS
[2026-11-25T15:43:34Z] id=15-0 172.17.0.1 | protocol=HTTP sni=inspector.example.test tls=TLS 1.3 alpn=h2 offered_alpn=- hello=- ja3=- ja4=- http2=- ua=curl/8.10.1 consistency=Unknown baseline=-/- status=-
```

Для отчёта важно разделять факт доставки события и наличие fingerprint:
доставка события ещё не означает, что доступен raw ClientHello.

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
- [OWASP Bot Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Bot_Management_and_Anti-Automation_Cheat_Sheet.html) — практическая интерпретация network signals;
- [Squid logformat](https://www.squid-cache.org/Doc/config/logformat/) — модель access-отчёта.
