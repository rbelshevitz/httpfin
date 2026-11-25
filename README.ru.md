# httpfin: инспектор отпечатка HTTP-клиента

[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![Nginx](https://img.shields.io/badge/Nginx-1.28.3-009639?logo=nginx&logoColor=white)](Dockerfile)
[![Alpine](https://img.shields.io/badge/Alpine-3.23-0D597F?logo=alpinelinux&logoColor=white)](Dockerfile)
[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)](docker-compose.yml)
[![CI](https://github.com/rbelshevitz/httpfin/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/rbelshevitz/httpfin/actions/workflows/ci.yml)

> **Наблюдаем клиента. Объясняем путь запроса.**

`httpfin` — диагностический сервис на Go для входящих HTTP-клиентов. Он
собирает доступные в точке наблюдения данные HTTP, TLS/SNI/ALPN и User-Agent,
а затем предоставляет JSON API, UI и отчёт в стиле Squid.

Это инструмент наблюдения перед API: он собирает признаки клиента и строит
понятный отчёт. Сам по себе `httpfin` не является системой аутентификации и не
блокирует запросы. `curl-impersonate` используется только как генератор
эталонного трафика; его профили могут устаревать. Проект публикуется и
сопровождается под GitHub-аккаунтом [`lwthiker`](https://github.com/lwthiker).

## Быстрый запуск

```bash
go run ./cmd/httpfin
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/api/connections
curl -fsS http://127.0.0.1:8080/api/report > httpfin-report.txt
```

UI: <http://127.0.0.1:8080/>.

В production collector по умолчанию слушает только `127.0.0.1:8080` и не
добавляет синтетические записи. Демонстрационные данные включаются только
явно: `httpfin_demo=1`.

Основные endpoints:

- `/api/connections` — последние наблюдения в JSON;
- `/api/report` — отчёт в стиле Squid по запросу;
- `/healthz` — health check;
- `/` — локальный UI.

## Где стоит httpfin

Основной режим — наблюдатель после точки завершения TLS:

```text
клиент -- TLS --> Nginx -- HTTP --> приложение
                    |
                    └─ модуль httpfin → Go-отчёт/API
```

Nginx владеет сертификатом и расшифровывает соединение. После этого модуль
видит HTTP-запрос, согласованные версию TLS, SNI и ALPN. Именно этот режим
поддерживает текущий продукт и Docker-пример.

В режиме TLS passthrough Nginx только пересылает зашифрованные данные другому
TLS-сервису. `httpfin` не видит там HTTP-запрос. `ssl_preread` может дать Nginx
SNI и предложенный ALPN для маршрутизации, но текущий HTTP-модуль не получает
полный отпечаток ClientHello.

`ClientHello` — первое сообщение TLS от клиента. В нём находятся предложенные
версии TLS, шифры и расширения. Из этих данных вычисляют JA3/JA4. Для их сбора
нужен TLS-aware listener в точке завершения TLS. Текущий HTTP-модуль Nginx не
является таким listener; в будущем это можно добавить в прямой Go-режим с
подходящей библиотекой.

ClientHello — это не HTTP-запрос, не сертификат и не пользовательские данные.
Он отправляется один раз при установлении нового TLS-соединения. Его поля
отличаются у TLS 1.2 и TLS 1.3, а также у настольных и мобильных браузеров,
SDK и командных клиентов. В нём могут быть SNI, список ALPN вроде `h2` и
`http/1.1`, шифры, расширения, поддерживаемые группы и схемы подписи. SNI или
ALPN могут отсутствовать, при возобновлении TLS появляются данные PSK или
сессии, а Encrypted ClientHello (ECH) может скрыть настоящий SNI и внутреннюю
часть handshake. Поэтому отпечаток ClientHello — это признак для сравнения,
а не постоянный идентификатор клиента.

Отчёт создаётся по запросу и не сохраняется сервером в отдельную папку. При
перенаправлении вывода файл создаётся в текущем каталоге shell, из которого
запущен `curl`.

## Docker: Nginx + TLS + модуль

```bash
docker compose up --build
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  --http2 https://inspector.example.test:8443/healthz
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/api/report > httpfin-report.txt
```

Тестовый entrypoint создаёт короткоживущий self-signed сертификат. В production
настоящие сертификат и ключ нужно монтировать в Nginx как секреты только для
чтения. Если TLS завершается в Nginx, сборщику Go не нужен доступ к закрытому
ключу. Этот Compose-режим предназначен для наблюдения и smoke-тестов. Он не
меняет трафик приложения и состояние firewall хоста.

## Как работает интеграция с Nginx

```text
клиент → Nginx / ingress → приложение
             │
             └─ native module → Unix datagram → Go httpfin → отчёт/API
```

Нативный модуль работает в HTTP-жизненном цикле Nginx и отправляет одно
ограниченное неблокирующее событие после появления итогового статуса. Go
принимает событие, нормализует, сохраняет и показывает его в отчёте.

Go не подключается обратно к модулю. Если Go недоступен, Nginx продолжает
обслуживать запросы; передача события выполняется в режиме best-effort, поэтому
событие может потеряться.

Модуль не разбирает исходный ClientHello, не считает JA3/JA4 и не видит
настройки HTTP/2 на уровне соединения. Для этого нужен отдельный TLS-вход с
поддержкой такого анализа.

Подробности: [`nginx-module/README.md`](nginx-module/README.md) и
[`docs/INTEGRATION_FORMAT.ru.md`](docs/INTEGRATION_FORMAT.ru.md).

## Что может показать httpfin

- User-Agent, заявленный клиентом;
- наблюдаемую или согласованную версию TLS, SNI и ALPN;
- HTTP method, target, status, timing, bytes и исходный IP-адрес;
- эвристические профили desktop и mobile-клиентов: `Chrome-like`,
  `Chrome Android-like`, `Edge-like`, `Firefox-like`, `Samsung Internet-like`,
  `Android WebView-like` и `Generic HTTP`;

Текущий HTTP-модуль Nginx не создаёт JA3/JA4 и отпечаток HTTP/2 на уровне
соединения. Для этого нужна отдельная программа перед Nginx или вместо него на
входе TLS. Она принимает TLS, читает ClientHello и кадры HTTP/2, а затем
передаёт результаты в `httpfin`.

User-Agent и fingerprint — управляемые клиентом или имитируемые признаки.
Пустой fingerprint означает отсутствие нужных данных в точке наблюдения, а не
mismatch браузера.

## Для какой задачи каких данных достаточно

| Задача | Обычно достаточно | Углубляться нужно только если |
|---|---|---|
| API QA и регрессия | метод, URI, статус, время, размер, User-Agent, протокол, TLS, SNI и ALPN | нужно сравнить точный TLS handshake или установку HTTP/2-соединения |
| Отладка Nginx, прокси и ingress | исходный IP, Host, SNI, ALPN, версия TLS, статус и сравнение с access log | нужно сравнить исходный handshake до завершения TLS |
| Сравнение браузеров, SDK и мобильных клиентов | профиль, транспортные поля и baseline по версии клиента | изменение нельзя объяснить версией клиента или сетевым маршрутом |
| Разбор различий ClientHello | поля прямого Go TLS-режима и локальная подпись | нужны стандартные JA3/JA4 или исходные поля handshake |
| Разбор поведения HTTP/2 | версия HTTP и результат запроса | важны SETTINGS, порядок кадров или поведение соединения |
| Учёт каждого запроса | отчёт из памяти для короткой проверки | нужен Nginx access log или будущее долговременное хранилище |

Более глубокий уровень не всегда лучше. Он добавляет сложность развёртывания,
работу с сертификатами или перехватом пакетов и увеличивает объём хранения.
Переходить к нему стоит только для конкретного вопроса, на который текущий
отчёт не отвечает. Ни одно из этих полей само по себе не удостоверяет человека
и не является основанием для разрешения или запрета клиента.

## Практическая польза

- сравнение curl, SDK, браузеров и `curl-impersonate`;
- отладка API-клиентов, прокси, ingress и CDN-маршрутов;
- проверка TLS termination, SNI, ALPN и исходного IP-адреса;
- поиск сетевых регрессий после обновления Nginx, OpenSSL, браузера или SDK;
- временная шкала инцидента для DevOps, API QA и команд безопасности;
- подготовка повторяемого отчёта для инцидента, rollout или QA-проверки.

Подробные материалы:

- [`docs/USE_CASES_TEAM.ru.md`](docs/USE_CASES_TEAM.ru.md)
- [`docs/DEVOPS_RUNBOOK.ru.md`](docs/DEVOPS_RUNBOOK.ru.md)
- [`docs/NONTECHNICAL.ru.md`](docs/NONTECHNICAL.ru.md) — инструкция для сотрудников и API-тестировщиков
- [`docs/OPERATING_MODES.ru.md`](docs/OPERATING_MODES.ru.md)
- [`docs/WIRE_OBSERVER.ru.md`](docs/WIRE_OBSERVER.ru.md)
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
