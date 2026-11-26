# Режимы работы httpfin: сценарии, TLS и сертификаты

`httpfin` — контрольная точка между клиентом, ingress и приложением. Выберите
точку наблюдения по задаче: HTTP, прямой TLS, Nginx termination или passthrough.

## Короткая шпаргалка

| Сценарий | Кто принимает TLS | Сертификат для httpfin | Что видно | Основной результат |
|---|---|---:|---|---|
| Локальная отладка HTTP API | никто | нет | HTTP-запрос | проверить метод, URL, UA, статус |
| `httpfin` принимает HTTPS напрямую | `httpfin`/Go | да | TLS, SNI, ALPN и HTTP | прямая диагностика TLS |
| Nginx termination | Nginx | нет для Go | Nginx TLS-поля + HTTP handoff | production ingress и API troubleshooting |
| Nginx `stream` passthrough | downstream-сервис | нет для Nginx | SNI, предложенный ALPN, заявленная версия TLS | проверить маршрутизацию до backend |
| тестовый контейнер | Nginx | генерируется автоматически | HTTPS/SNI/HTTP/2 | smoke-тест интеграции |

## Архитектура и границы наблюдения

```text
клиент → Nginx → приложение
           └─ модуль httpfin → Unix-датаграмма → Go-сборщик → API/UI/отчёт
```

Nginx продолжает обслуживать запросы, если сборщик недоступен. Модуль после
обработки запроса отправляет ограниченное неблокирующее событие: он не ждёт Go
и не выполняет в worker HTTP-, DNS-, database- или fingerprint-операции. Go
нормализует событие, хранит ограниченную историю в памяти и отдаёт UI и отчёты.
После перезапуска наблюдения исчезают.

Событие Nginx содержит ID запроса, метод, URI, Host, User-Agent, исходный адрес,
статус, время запроса, размер ответа, протокол, TLS version, SNI и ALPN, если
они доступны. После TLS termination исходный ClientHello недоступен, поэтому
этот путь не вычисляет стандартные JA3/JA4 и fingerprint соединения HTTP/2.

## 1. Отладка API без TLS

Используйте этот режим, когда нужно понять, что реально дошло до обработчика:
метод, URL, User-Agent, статус, задержка и размер ответа.

```bash
go run ./cmd/httpfin
curl -v http://127.0.0.1:8080/healthz
curl -v -X POST http://127.0.0.1:8080/login \
  -H 'content-type: application/json' \
  -d '{"test":true}'
```

Сертификат не нужен. Этот режим подходит для:

- отладки SDK и API-клиента;
- сравнения `curl`, Go client, Python requests и браузера;
- проверки, не меняет ли reverse proxy метод или URL;
- первичного воспроизведения инцидента без усложнения TLS.

## 2. `httpfin` принимает HTTPS напрямую

Этот режим нужен для прямой диагностики TLS и HTTP без Nginx. Текущий Go-
слушатель получает отдельные поля ClientHello через callback TLS и сохраняет
локальную подпись, но ещё не считает стандартные JA3/JA4. Это отдельная задача.

```bash
httpfin_tls_cert=/etc/httpfin/tls/server.crt \
httpfin_tls_key=/etc/httpfin/tls/server.key \
httpfin_tls_addr=:8443 \
go run ./cmd/httpfin

curl -k --resolve api.example.test:8443:127.0.0.1 \
  https://api.example.test:8443/healthz
```

Здесь сертификат нужен обязательно: Go выступает TLS-сервером и должен
завершить handshake. Для локальной проверки допустим self-signed сертификат.
Для общего стенда или production нужен сертификат на настоящее имя, а
приватный ключ должен быть доступен только пользователю процесса.

Рекомендуемая раскладка вне контейнера:

```text
/etc/httpfin/tls/server.crt  # сертификат и цепочка, режим 0644
/etc/httpfin/tls/server.key  # приватный ключ, режим 0600
```

Проверка сертификата:

```bash
openssl x509 -in /etc/httpfin/tls/server.crt -noout \
  -subject -issuer -dates -ext subjectAltName
openssl s_client -connect 127.0.0.1:8443 \
  -servername api.example.test -alpn h2,http/1.1 </dev/null
```

Сертификат не делает fingerprint доверенным. Он нужен для роли TLS-сервера;
SNI по-прежнему является заявкой клиента и не заменяет аутентификацию.

## 3. Nginx завершает TLS: основной production-сценарий

Это рекомендуемый вариант для DevOps-команды. Сертификат находится на Nginx,
а Go получает request-level событие через Unix `SOCK_DGRAM` и/или JSON access
log.

```text
client --TLS--> Nginx --HTTP/proxy--> application
                   ├─ certificate/key
                   ├─ SNI/ALPN/TLS access log
                   └─ httpfin module -> /run/httpfin/events.sock -> Go
```

Для Go-сервера сертификат не нужен. Сертификат должен быть установлен в
Nginx-конфигурации:

```nginx
server {
    listen 443 ssl;
    server_name api.example.test;

    ssl_certificate     /etc/nginx/tls/api.example.test.fullchain.pem;
    ssl_certificate_key /etc/nginx/tls/api.example.test.key;

    httpfin on;
    proxy_pass http://backend;
}
```

Если Nginx запущен на хосте, ключ хранится на хосте и не монтируется в Go.
Если Nginx работает в контейнере, сертификаты монтируются в контейнер Nginx
как secret или read-only volume:

```yaml
services:
  httpfin:
    volumes:
      - /etc/letsencrypt/live/api.example.test/fullchain.pem:/etc/nginx/certs/server.crt:ro
      - /etc/letsencrypt/live/api.example.test/privkey.pem:/etc/nginx/certs/server.key:ro
```

Встроенный Docker-пример `httpfin` генерирует временный self-signed сертификат
для `inspector.example.test`. Это только smoke-тест, не production-хранилище
сертификатов.

Для локального smoke-теста создайте короткоживущий самоподписанный сертификат
и направьте примерное имя хоста на localhost:

```bash
sudo install -d -m 0750 /etc/nginx/tls
sudo openssl req -x509 -nodes -newkey rsa:2048 -days 7 \
  -keyout /etc/nginx/tls/api.example.test.key \
  -out /etc/nginx/tls/api.example.test.fullchain.pem \
  -subj '/CN=api.example.test' \
  -addext 'subjectAltName=DNS:api.example.test'
echo '127.0.0.1 api.example.test' | sudo tee -a /etc/hosts
sudo nginx -t && sudo systemctl reload nginx
```

`curl -k` используйте только с этим локальным сертификатом. Для настоящего
публичного имени получите доверенный сертификат, например через Certbot:

```bash
sudo certbot --nginx -d api.example.com
```

Обычно Certbot хранит сертификат и приватный ключ в
`/etc/letsencrypt/live/api.example.com/`. Самоподписанный сертификат нельзя
использовать в production; приватный ключ должен быть доступен только Nginx.

Важно: после TLS termination исходный ClientHello не передаётся приложению.
Nginx может записать `$ssl_server_name`, `$ssl_protocol`,
`$ssl_alpn_protocol`, cipher и HTTP-поля, но точный JA3/JA4 требует сбора
ClientHello до termination.

## 4. TLS passthrough через Nginx

Используйте этот режим, если Nginx не должен расшифровывать трафик.

```nginx
stream {
    map $ssl_preread_server_name $backend {
        api.example.test api_backend;
        default          reject_backend;
    }

    upstream api_backend    { server app:443; }
    upstream reject_backend { server 127.0.0.1:9443; }

    server {
        listen 443;
        proxy_pass $backend;
        ssl_preread on;
    }
}
```

Сертификат для Nginx не нужен: он только читает ClientHello. Сертификат нужен
конечному `app:443`, потому что именно этот сервис завершает TLS.

В passthrough доступны SNI, список ALPN и версия TLS, заявленная в ClientHello,
но недоступны согласованная версия TLS, HTTP method, URL, headers, status и
User-Agent. Поэтому это режим для
проверки маршрутизации и TLS-совместимости, а не для полного HTTP-отчёта.

## 5. Хранение наблюдений

По умолчанию `httpfin` хранит ограниченное число последних наблюдений в памяти,
а подробные запросы — в access log. Это предсказуемый режим для production:
он не требует отдельной базы, не разрастается бесконечно и не делает отчёт
зависимым от состояния диска.

После перезапуска история из памяти исчезает. Это ожидаемо и не означает, что
Nginx перестал принимать запросы. Для расследований сохраняйте access log и
собирайте отчёт до перезапуска.

SQLite сейчас не является обязательной частью развёртывания и в штатный запуск
не включён. Он понадобится только если потребуется переживающая перезапуск
история с поиском и заданным сроком хранения. В таком случае нужно отдельно
задать очистку старых записей, защиту файла, контроль свободного места и
восстановление после ошибки диска; простая запись в SQLite без этих правил не
считается production-ready.

## 6. Что проверять команде

### DevOps/SRE после изменения ingress

1. Выполнить один и тот же запрос напрямую в приложение и через Nginx.
2. Сравнить SNI, negotiated ALPN, HTTP protocol, client IP, status и время запроса.
3. Проверить `/api/connections`, `/api/ingest` и access log.
4. Если Nginx менялся — убедиться, что модуль пересобран под ту же версию.

### API-тестировщику

Сохранить отчёт для обычного клиента, `curl-impersonate` и SDK. После обновления
SDK сравнить не только response body, но и TLS/ALPN/HTTP/2. Изменение
транспортного признака — повод проверить регресс и сохранить отчёт.

### Приёмке production-развёртывания

```bash
nginx -t
nginx -V 2>&1
curl -vk --resolve api.example.test:443:127.0.0.1 \
  https://api.example.test/healthz
curl -sS http://127.0.0.1:8080/api/connections
```

Не считать отсутствие события в Go доказательством отсутствия запроса: handoff
является best-effort telemetry. `/api/ingest` показывает состояние событий,
которые достигли socket сборщика, но не потери до этой точки. Для полного
аудита сохраняйте access log и сверяйте его с `/api/connections`.

## Связанные документы

- [`README.ru.md`](../README.ru.md) — быстрый старт;
- [`DEVOPS_RUNBOOK.ru.md`](DEVOPS_RUNBOOK.ru.md) — эксплуатационная диагностика;
- [`USE_CASES.ru.md`](USE_CASES.ru.md) — практические сценарии для API, QA и эксплуатации;
- [`nginx-module/README.md`](../nginx-module/README.md) — C-модуль и ABI-ограничения.

Протокольные источники: [RFC 9846](https://www.rfc-editor.org/rfc/rfc9846),
[RFC 6066](https://www.rfc-editor.org/rfc/rfc6066),
[RFC 7301](https://www.rfc-editor.org/rfc/rfc7301),
[RFC 9113](https://www.rfc-editor.org/rfc/rfc9113),
[RFC 9110](https://www.rfc-editor.org/rfc/rfc9110) и
[RFC 7239](https://www.rfc-editor.org/rfc/rfc7239),
[RFC 9849](https://www.rfc-editor.org/rfc/rfc9849), а также исторический
[RFC 8446](https://www.rfc-editor.org/rfc/rfc8446).
