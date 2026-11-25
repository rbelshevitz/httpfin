# httpfin — инспектор отпечатка HTTP-клиента

> **Наблюдаем клиента. Объясняем путь запроса.**

Проект использует upstream [curl-impersonate](https://github.com/lwthiker/curl-impersonate)
как генератор контрольного трафика. Создатель и основной сопровождающий
upstream-проекта публикуется под GitHub-аккаунтом
[`lwthiker`](https://github.com/lwthiker); подтверждённое настоящее имя в
публичном профиле не указано. Это важно учитывать: версии
`curl-impersonate` и эталонные browser-like признаки могут устаревать вместе с
браузерами, TLS-библиотеками и HTTP/2-реализациями.

Диагностический сервер на Go для пассивного анализа входящих HTTP-клиентов. `curl-impersonate` используется как генератор эталонных запросов: можно сравнивать обычный curl, `curl_chrome116`, настоящий браузер и другие клиентские библиотеки.

## Быстрый запуск

```bash
go run ./cmd/httpfin
# открыть http://127.0.0.1:8080
```

Эндпоинты:

- `GET /api/connections` — последние наблюдения в JSON;
- `GET /api/report` — отчёт в формате Squid-style TXT;
- `GET /healthz` — health check.

HTTPS/SNI capture включается сертификатом:

```bash
httpfin_tls_cert=./server.crt \
httpfin_tls_key=./server.key \
httpfin_tls_addr=:8443 \
go run ./cmd/httpfin
```

В этом режиме httpfin фиксирует согласованную TLS version, negotiated ALPN,
предложенные клиентом ALPN, SNI и локальную сводку ClientHello. Эта сводка не
является JA3: она зависит от полей и порядка, которые отдаёт Go TLS API. Точный
JA3/JA4 будет отдельным parser-модулем: текущая Go-модель не подменяет
стандартный fingerprint эвристическим хешем.

Для native Nginx-модуля можно включить Unix datagram ingest:

```bash
httpfin_socket=/run/httpfin/events.sock go run ./cmd/httpfin
```

Модуль отправляет один JSON event на запрос; отсутствие сокета не должно блокировать Nginx worker. Go принимает события, добавляет их в общее хранилище и отчёты.

## Архитектурный подход

Практическая матрица режимов, сценарии для DevOps/API QA и правила работы с
сертификатами находятся в [`docs/OPERATING_MODES.ru.md`](docs/OPERATING_MODES.ru.md).
Коротко: сертификат нужен только если TLS завершает сам `httpfin`; при
termination в Nginx сертификат принадлежит Nginx, а при passthrough — конечному
TLS-сервису.

Рекомендуемая схема: Nginx остаётся data-plane — принимает TLS, проксирует запросы и ограничивает rate; Go становится collector/analysis-plane — собирает события, считает признаки, хранит историю и строит отчёты.

```text
клиент → Nginx / ingress → приложение
             │
             └─ access log + TLS/ALPN metadata → Go collector → UI / отчёты
```

### Границы Nginx-модуля

В репозитории уже есть рабочий native-модуль на C. Он остаётся тонким
integration layer: снимает request/TLS-поля, создаёт request ID и передаёт
ограниченное JSON-событие в Go через неблокирующий Unix datagram.

Модуль не хранит историю, не считает JA3/JA4, не обращается к БД и не
принимает тяжёлых policy-решений внутри worker. Анализ, baseline, risk,
отчёты и UI остаются в Go. Это разделение позволяет обновлять правила и
формат отчёта без пересборки Nginx.

Для TLS без termination Nginx может использовать `ngx_stream_ssl_preread_module` и получить SNI/ALPN из ClientHello. Для точного JA3/JA4 нужен raw ClientHello или полноценный preread-парсер. После TLS termination нужно передать в Go хотя бы `$ssl_protocol`, `$ssl_alpn_protocol`, `$ssl_cipher`, `$ssl_server_name`, `$http_user_agent` и исходный IP.

Отдельный preread-конфиг для SNI/ALPN находится в [`configs/nginx/httpfin-stream.conf`](configs/nginx/httpfin-stream.conf). Он не видит HTTP после TLS и потому используется для маршрутизации и stream-диагностики, а не для полного отчёта запроса.

### Что именно можно узнать о клиенте

`httpfin` фиксирует наблюдаемое поведение, а не удостоверяет личность
клиента:

- полный User-Agent, присланный клиентом;
- согласованные TLS version, SNI и ALPN в точке, где доступен TLS-контекст;
- список предложенных ALPN и сводный ClientHello signature при прямом HTTPS
  listener `httpfin`;
- метод, URI, статус, задержку, объём ответа и адрес источника;
- эвристический профиль `Chrome-like`, `Firefox-like`, `Safari-like` или
  `Generic HTTP`.

По User-Agent можно сказать, что клиент заявил себя как браузер или библиотека,
а по совокупности TLS/ALPN/HTTP/2 — насколько это заявление согласуется с
наблюдением. Нельзя честно утверждать, что запрос точно отправлен настоящим
Chrome: User-Agent подделывается, а `curl-impersonate` как раз предназначен
для browser-like TLS и HTTP/2. Точные JA3/JA4 и HTTP/2 fingerprint требуют
raw ClientHello и кадров HTTP/2; из одного запроса после TLS termination их
восстановить нельзя.

`JA3` — нормализованный fingerprint TLS ClientHello; `JA3S` описывает ответ
сервера. `JA4` — более новый TLS fingerprint FoxIO, `JA4H` описывает поведение
HTTP-клиента, а `JA4+` — название расширенного семейства. Это сигналы для
группировки, а не поля RFC и не аутентификация клиента. Текущий модуль
`httpfin` их не считает, но может показать значения, переданные внешним
wire-observer.

## ⚡ Цели проекта

`httpfin` нужен, чтобы быстро ответить на вопрос: «какой клиент реально
пришёл на этот endpoint и что изменилось по дороге?». Проект решает такие
практические задачи:

- ⚡ отладка API-клиента: сравнить curl, SDK, Go/Java/Python-клиент,
  браузер и `curl-impersonate`;
- ⚡ проверка ingress и reverse proxy: не потерялись ли IP, SNI, ALPN,
  HTTP-версия, User-Agent и время запроса;
- ⚡ контроль TLS: какой протокол согласован, какой SNI выбран, какой ALPN
  получил сервер и где именно завершается TLS;
- ⚡ поиск расхождений: заявлен Chrome, а набор сетевых признаков похож на
  generic HTTP-клиент;
- ⚡ регрессионные проверки: изменился ли профиль после обновления Nginx,
  OpenSSL, браузера, SDK или прокси;
- ⚡ диагностика отправок: сравнить прямой запрос, запрос через proxy,
  ingress, CDN или service mesh;
- ⚡ incident triage: связать время, IP, endpoint, статус, User-Agent и
  доступные fingerprint-сигналы в одной временной шкале;
- ⚡ подготовка данных для security-анализа и anti-bot правил без принятия
  автоматического решения о блокировке внутри `httpfin`.

Результат — наблюдаемая техническая картина запроса и понятный отчёт для
DevOps/SRE, API QA, backend-разработчика и security-команды. `httpfin` не
подменяет WAF, SIEM, полноценную браузерную телеметрию или систему
аутентификации.

Подробные сценарии для DevOps/SRE, API-тестировщиков, backend и security находятся в [`docs/USE_CASES_TEAM.ru.md`](docs/USE_CASES_TEAM.ru.md). Режимы TLS, сертификаты и сборка модуля вне Docker описаны в [`docs/OPERATING_MODES.ru.md`](docs/OPERATING_MODES.ru.md) и [`docs/INTEGRATION_FORMAT.ru.md`](docs/INTEGRATION_FORMAT.ru.md). Интерфейс Swagger/OpenAPI — [`docs/openapi.yaml`](docs/openapi.yaml).
Схемы режимов наблюдения и потоков данных находятся в [`docs/ARCHITECTURE.ru.md`](docs/ARCHITECTURE.ru.md).

Пример расширенного Nginx-лога:

```nginx
log_format httpfin escape=json
  '{"ts":"$time_iso8601",'
  '"elapsed_ms":$request_time,'
  '"client_ip":"$remote_addr",'
  '"method":"$request_method",'
  '"url":"$request_uri",'
  '"status":$status,'
  '"bytes":$bytes_sent,'
  '"protocol":"$server_protocol",'
  '"tls_version":"$ssl_protocol",'
  '"alpn":"$ssl_alpn_protocol",'
  '"user_agent":"$http_user_agent",'
  '"request_id":"$request_id"}';

access_log /var/log/nginx/httpfin.jsonl httpfin;
```

## Практические сценарии

1. Диагностика клиента: сравнить curl, curl-impersonate, Go HTTP client и браузер по TLS, ALPN, User-Agent и HTTP/2.
2. Anti-bot и abuse triage: обнаружить ситуацию, когда заявлен Chrome, но TLS/ALPN/HTTP/2 похожи на generic client.
3. Расследование инцидента: связать один fingerprint с разными IP, URL-паттернами и временной шкалой.
4. Проверка reverse proxy: убедиться, что ingress не теряет IP, protocol и request timing.
5. Регрессионные тесты сетевого ПО: хранить эталонные fingerprints и сравнивать версии curl-impersonate, прокси и TLS-библиотек.

Подробные пошаговые сценарии с командами находятся в [`docs/USE_CASES.ru.md`](docs/USE_CASES.ru.md): от отладки API-клиента и входящих запросов до анализа отправок через прокси и incident triage.

Протокольные границы и RFC-разделение также описаны там: TLS 1.3 (RFC 9846 и
исторический RFC 8446), SNI (RFC 6066), ALPN (RFC 7301), HTTP/2 (RFC 9113),
HTTP semantics (RFC 9110), `Forwarded` (RFC 7239) и Encrypted ClientHello
(RFC 9849).

## Отчётность

`/api/report` выдаёт два слоя:

- access summary в стиле Squid: время, latency, клиент, статус, bytes, метод/URL, профиль и risk;
- fingerprint details: TLS, ALPN, JA3, JA4, HTTP/2, User-Agent и consistency.

Отпечаток — сигнал, а не доказательство личности клиента. Для блокировки нужно объединять его с rate, auth, IP reputation, поведением сессии и application logs.

Отчёт ценен как диагностическая сводка: он связывает в одной строке время,
клиент, запрос, протокол, TLS/SNI/ALPN, User-Agent, профиль и уровень риска.
Это удобно для отладки API, проверки ingress, регрессионных тестов после
обновления Nginx/OpenSSL/клиентской библиотеки и первичного incident triage.
Пустое поле JA3/JA4/HTTP/2 означает, что текущая точка наблюдения не получила
нужные данные, а не то, что запрос был ошибочным. Решения о блокировке нужно
принимать только вместе с rate limits, аутентификацией, репутацией IP,
поведением сессии и логами приложения.

### Суть отчёта и как его создать

`httpfin` строит отчёт по наблюдениям, уже принятым Go collector. Он не
пересканирует трафик задним числом и не придумывает отсутствующие TLS-данные.
Одна запись объединяет операционную сводку и доступные в точке наблюдения
сигналы:

- время, request ID, адрес источника, метод и URL;
- статус, размер ответа и задержку;
- HTTP protocol, TLS version, SNI и ALPN;
- User-Agent, эвристический профиль и результат consistency;
- JA3, JA4 и HTTP/2 — только если их передал подходящий wire-наблюдатель.

JSON для CI, дашборда или дальнейшей обработки:

```bash
curl -fsS http://127.0.0.1:8080/api/connections > connections.json
```

Читаемый Squid-style TXT для incident ticket, rollout или результата API-теста:

```bash
curl -fsS http://127.0.0.1:8080/api/report > httpfin-report.txt
```

JSON удобен для автоматизации, TXT — для человека и приложенного к задаче
артефакта. Пустое поле fingerprint означает, что точка наблюдения не получила
нужные данные; его нельзя автоматически трактовать как несовпадение.

### Docker: что проверено и какие есть нюансы

`Dockerfile` собирает Nginx 1.28.3 из исходников в Alpine 3.23 с
`--with-compat` и динамическим модулем, а Go-бинарник собирает статически.
Это даёт ровную совместимость Nginx и `.so`: модуль не берётся из одной
системы, а Nginx — из другой. В runtime нужны OpenSSL, PCRE2 и zlib для самого
Nginx; модуль не является самостоятельной программой и не должен запускаться
через `ldd` как обычное приложение.

В образе сертификат создаётся entrypoint-скриптом только для smoke-теста и
имеет короткий срок действия. В рабочем окружении монтируйте сертификат и
ключ в `/etc/nginx/certs` read-only, например через Docker/Kubernetes secret.
При termination в Nginx ключ Go-процессу не нужен. При TLS passthrough
сертификат находится у конечного backend. При прямом HTTPS на `httpfin`
сертификат и ключ передаются через `httpfin_tls_cert` и `httpfin_tls_key`.

Теги `alpine:3.23`, `golang:1.27-alpine` и версия Nginx заданы явно на уровне
веток/версий, а `apk` получает актуальные security fixes этой ветки во время
сборки. Для полностью воспроизводимого релиза дополнительно фиксируйте digest
базовых образов и проводите регулярное обновление этих digest через CI.
Перед выпуском запускайте:

```bash
go test ./...
go vet ./...
./tests/nginx-module-static.sh
./tests/container-smoke.sh
```

### Baseline не должен быть вечным

Профиль `Chrome-like` — это классификация, а не обещание конкретной версии браузера. `curl-impersonate` и реальные браузеры меняются, поэтому baseline нужно хранить как данные: имя профиля, версию Chrome/Firefox/Safari, версию `curl-impersonate`, дату проверки, источник и статус (`matched`, `drifted`, `unknown`). При обновлении браузера сначала прогоняем контрольные запросы, фиксируем drift, затем обновляем baseline. Старые fingerprints не удаляем: они нужны для расследования исторического трафика.

## Структура исходников

- `cmd/httpfin` — запуск сервера и HTTP-маршруты;
- `internal/model` — модели соединения и потокобезопасное хранилище;
- `internal/fingerprint` — профиль клиента и утилиты отпечатка;
- `internal/report` — Squid-style TXT-отчёт;
- `internal/demo` — демонстрационные записи для UI.
