# httpfin: наблюдение за HTTP-клиентами

[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![Nginx](https://img.shields.io/badge/Nginx-1.28.3-009639?logo=nginx&logoColor=white)](Dockerfile)
[![Alpine](https://img.shields.io/badge/Alpine-3.23-0D597F?logo=alpinelinux&logoColor=white)](Dockerfile)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)](docker-compose.yml)
[![CI](https://github.com/rbelshevitz/httpfin/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/rbelshevitz/httpfin/actions/workflows/ci.yml)

> **Наблюдаем клиента. Объясняем путь запроса.**

`httpfin` — диагностический сервис перед API. Он собирает доступные в точке
наблюдения признаки HTTP-клиента и создаёт JSON API, UI и читаемый текстовый
отчёт.

`httpfin` не является системой аутентификации, не определяет личность клиента и
не блокирует запросы. `curl-impersonate` используется только для генерации
эталонного трафика; этот проект сопровождается под аккаунтом
[`lwthiker`](https://github.com/lwthiker).

## Быстрый запуск

```bash
go run ./cmd/httpfin
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/api/connections
curl -fsS http://127.0.0.1:8080/api/report > httpfin-report.txt
```

UI доступен по адресу <http://127.0.0.1:8080/>. По умолчанию сборщик слушает
только `127.0.0.1:8080` и хранит ограниченное число последних наблюдений в
памяти. Демонстрационные записи включаются только через `httpfin_demo=1`.

## Основной режим

```text
клиент -- TLS --> Nginx -- HTTP --> API
                    │
                    └─ модуль httpfin → Go-сборщик → отчёт/API
```

В рекомендуемом режиме Nginx завершает TLS и передаёт модулю данные HTTP,
согласованные версию TLS, SNI и ALPN, а затем отправляет ограниченное
неблокирующее событие в Go через Unix datagram. Если сборщик недоступен, Nginx
продолжает обслуживать запросы, но событие может потеряться.

Отчёт создаётся при запросе к `/api/report` и не записывается сервисом в файл.
Сохранить его можно перенаправлением вывода, как в примере выше.

## Что показывает текущий httpfin

- метод HTTP, путь, статус и исходный IP-адрес;
- User-Agent и эвристический профиль клиента;
- TLS-версию, SNI и ALPN, если они доступны в точке наблюдения;
- изменения признаков между последовательными наблюдениями;
- JSON API, UI и текстовый отчёт.

Время обработки и размер ответа берутся из прямого Go-режима или журнала
доступа Nginx. Текущий HTTP-модуль Nginx не передаёт их в своём событии.

## Важные ограничения

`ClientHello` — первое сообщение TLS от клиента. В нём находятся предложенные
версии TLS, шифры и расширения. В обычной HTTP-фазе Nginx этот пакет уже
недоступен, поэтому модуль не считает стандартные JA3/JA4 и не видит настройки
HTTP/2 на уровне соединения.

При TLS passthrough Nginx только пересылает зашифрованные данные. `httpfin` не
видит там URL, заголовки, User-Agent и статус ответа. `ssl_preread` может дать
Nginx SNI и предложенный ALPN для маршрутизации, но не полный отпечаток
ClientHello.

User-Agent и профиль можно подделать. Отсутствие признака означает, что он
недоступен в выбранной точке наблюдения, а не что клиент не совпал с эталоном.
Отчёт предназначен для диагностики, а не для блокировки или идентификации.

## Docker

```bash
docker compose up --build
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/healthz
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/api/report > httpfin-report.txt
```

Без Compose-плагина:

```bash
docker build --pull=false -t httpfin:local .
docker run -d --rm --name httpfin -p 8443:8443 httpfin:local
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/healthz
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/api/report > httpfin-report.txt
docker rm -f httpfin
```

Образ создаёт временный самоподписанный сертификат только для проверки. В рабочей
среде сертификат и ключ нужно передавать Nginx как секреты только для чтения.
Если Alpine не может скачать пакеты из-за DNS, повторите сборку с
`--network=host`.

## Документация

- [`docs/USE_CASES_TEAM.ru.md`](docs/USE_CASES_TEAM.ru.md) — сценарии для DevOps, QA и разработчиков;
- [`docs/NONTECHNICAL.ru.md`](docs/NONTECHNICAL.ru.md) — инструкция для нетехнических сотрудников;
- [`docs/OPERATING_MODES.ru.md`](docs/OPERATING_MODES.ru.md) — TLS, сертификаты и режимы Nginx;
- [`docs/INTEGRATION_FORMAT.ru.md`](docs/INTEGRATION_FORMAT.ru.md) — событие модуля и формат отчёта;
- [`docs/WIRE_OBSERVER.ru.md`](docs/WIRE_OBSERVER.ru.md) — границы ClientHello и HTTP/2;
- [`docs/DEVOPS_RUNBOOK.ru.md`](docs/DEVOPS_RUNBOOK.ru.md) — эксплуатационная диагностика;
- [`TODO.ru.md`](TODO.ru.md) — текущий план работ.

## Проверки разработки

```bash
go test ./...
go vet ./...
./tests/nginx-module-static.sh
./tests/container-smoke.sh
```

Полная сборка Docker/Nginx-модуля запускается вручную через GitHub Actions
workflow `Manual Docker build`. Быстрый CI проверяет Go-код и исходник модуля.

English documentation: [`README.md`](README.md).
