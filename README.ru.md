# httpfin — инспектор отпечатка HTTP-клиента

[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![Лицензия](https://img.shields.io/badge/license-MIT-2ea44f?logo=opensourceinitiative&logoColor=white)](LICENSE)
[![Nginx](https://img.shields.io/badge/Nginx-1.28.3-009639?logo=nginx&logoColor=white)](Dockerfile)
[![Alpine](https://img.shields.io/badge/Alpine-3.23-0D597F?logo=alpinelinux&logoColor=white)](Dockerfile)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)](docker-compose.yml)

> **Наблюдаем клиента. Объясняем путь запроса.**

`httpfin` — диагностический сервис на Go для входящих HTTP-клиентов. Он
собирает доступные в точке наблюдения HTTP-, TLS/SNI/ALPN- и User-Agent-данные,
а затем строит JSON API, UI и Squid-style отчёт.

Это инструмент наблюдения, а не система аутентификации и не автоматический
блокировщик. `curl-impersonate` используется только как генератор эталонного
трафика; его профили могут устаревать. Upstream поддерживает [`lwthiker`](https://github.com/lwthiker).

## Быстрый запуск

```bash
go run ./cmd/httpfin
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/api/connections
curl -fsS http://127.0.0.1:8080/api/report > httpfin-report.txt
```

UI: <http://127.0.0.1:8080/>.

Основные endpoints:

- `/api/connections` — последние наблюдения в JSON;
- `/api/report` — отчёт Squid-style по запросу;
- `/healthz` — health check;
- `/` — локальный UI.

Отчёт создаётся при обращении и не сохраняется сервером в отдельную папку.
При перенаправлении вывода файл создаётся в текущем каталоге shell, который
запустил `curl`.

## Docker: Nginx + TLS + модуль

```bash
docker compose up --build
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  --http2 https://inspector.example.test:8443/healthz
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/api/report > httpfin-report.txt
```

Тестовый entrypoint создаёт короткоживущий self-signed сертификат. В production
настоящие сертификат и ключ нужно монтировать в Nginx как read-only secrets.
Если TLS завершается в Nginx, Go collector не нужен доступ к private key.

## Как работает интеграция с Nginx

```text
клиент → Nginx / ingress → приложение
             │
             └─ native module → Unix datagram → Go httpfin → отчёт/API
```

Native-модуль работает в HTTP-жизненном цикле Nginx и отправляет одно
ограниченное неблокирующее событие после появления финального status. Go
принимает событие, нормализует, хранит и показывает его в отчёте.

Go не подключается обратно к модулю. Если Go недоступен, Nginx продолжает
обслуживать запросы; событие является best-effort telemetry и может потеряться.

Модуль не разбирает raw ClientHello, не считает JA3/JA4 и не видит
connection-level HTTP/2 SETTINGS. Для этих сигналов нужен direct Go TLS mode
или отдельный wire-observer.

Подробности: [`nginx-module/README.md`](nginx-module/README.md) и
[`docs/INTEGRATION_FORMAT.ru.md`](docs/INTEGRATION_FORMAT.ru.md).

## Что может показать httpfin

- User-Agent, заявленный клиентом;
- наблюдаемую или согласованную TLS version, SNI и ALPN;
- HTTP method, target, status, timing, bytes и source address;
- эвристические профили вроде `Chrome-like` и `Generic HTTP`;
- JA3/JA4 и HTTP/2 только при наличии подходящего wire-observer.

User-Agent и fingerprint — управляемые клиентом или имитируемые признаки.
Пустой fingerprint означает отсутствие нужных данных в точке наблюдения, а не
mismatch браузера.

## Практическая польза

- сравнение curl, SDK, браузеров и `curl-impersonate`;
- отладка API-клиентов, proxy, ingress и CDN-маршрутов;
- проверка TLS termination, SNI, ALPN и source IP;
- поиск сетевых регрессий после обновления Nginx, OpenSSL, браузера или SDK;
- incident timeline для DevOps, API QA и security-команд;
- подготовка данных для внешнего WAF, policy-сервиса или Fail2ban.

Подробные материалы:

- [`docs/USE_CASES_TEAM.ru.md`](docs/USE_CASES_TEAM.ru.md)
- [`docs/DEVOPS_RUNBOOK.ru.md`](docs/DEVOPS_RUNBOOK.ru.md)
- [`docs/OPERATING_MODES.ru.md`](docs/OPERATING_MODES.ru.md)
- [`docs/WIRE_OBSERVER.ru.md`](docs/WIRE_OBSERVER.ru.md)
- [`docs/FAIL2BAN.ru.md`](docs/FAIL2BAN.ru.md)
- [`docs/openapi.yaml`](docs/openapi.yaml)
- [`TODO.ru.md`](TODO.ru.md)

## Проверки разработки

```bash
go test ./...
go vet ./...
./tests/nginx-module-static.sh
./tests/container-smoke.sh
```

Последние две проверки требуют локальные зависимости сборки Nginx и Docker.

English documentation: [`README.md`](README.md).
