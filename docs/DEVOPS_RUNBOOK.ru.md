# httpfin для DevOps-команды

## Что даёт httpfin

Используйте `httpfin`, чтобы проверить путь запроса на границе API:

- подтвердить доставку события из Nginx в Go-сборщик;
- проверить исходный IP, SNI, ALPN, HTTP-версию, статус и время запроса;
- сопоставить handoff Nginx, `/api/ingest` и число записей в access log;
- приложить читаемый отчёт к задаче по инциденту.

## Локальный запуск через Docker

При прямом запуске status-dashboard и HTTP API доступны на порту `8080`:
<http://127.0.0.1:8080/>. В Docker smoke-сценарии внешний HTTPS-доступ идёт
через Nginx на порту `8443`; внутренний Go-сборщик при этом использует свой
служебный HTTP-порт.

```bash
docker build -t httpfin:dev .
docker run --rm -p 8443:8443 httpfin:dev
```

Проверка запроса с SNI:

```bash
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/healthz
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/api/ingest
```

Открыть UI и отчёт через тот же Nginx:

```bash
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/

curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/api/report
```

Этот образ предназначен для наблюдения и smoke-тестов. В production используйте
host/systemd-режим: Nginx и `httpfin` работают как службы хоста, а сокет и права
доступа управляются systemd.

Это demo/smoke-образ: путь `/app/` обслуживает встроенный mock API. Для реального
приложения используйте host/systemd-режим и настройте Nginx на нужный upstream.

Готовые заготовки:

- [`deploy/systemd/httpfin.service`](../deploy/systemd/httpfin.service);
- [`deploy/systemd/httpfin.env.example`](../deploy/systemd/httpfin.env.example).

Минимальная установка на Ubuntu:

```bash
sudo useradd --system --home /var/lib/httpfin --shell /usr/sbin/nologin httpfin
sudo install -d -o httpfin -g httpfin -m 0750 /etc/httpfin
sudo install -m 0640 -o root -g httpfin \
  deploy/systemd/httpfin.env.example /etc/httpfin/httpfin.env
sudo install -D -m 0644 deploy/systemd/httpfin.service \
  /etc/systemd/system/httpfin.service
```

Пользователь Nginx должен иметь доступ к Unix-сокету. Добавьте пользователя
воркера (`nginx` или `www-data`) в группу `httpfin`, затем перезапустите Nginx:

```bash
sudo usermod -aG httpfin nginx
sudo systemctl daemon-reload
sudo systemctl enable --now httpfin
sudo systemctl restart nginx
```

Если в системе используется `www-data`, замените имя пользователя в команде.
Сначала проверьте `/healthz`, `/api/ingest`, `/api/connections`, права на
`/run/httpfin/events.sock` и появление новых наблюдений.

## Что проверять после деплоя

### 1. TLS и SNI

```bash
openssl s_client \
  -connect 127.0.0.1:8443 \
  -servername inspector.example.test \
  -alpn h2,http/1.1 </dev/null 2>/dev/null \
  | openssl x509 -noout -subject -issuer
```

В отчёте должны появиться SNI, TLS version и negotiated ALPN. Пустой SNI —
самостоятельный результат наблюдения; его нельзя подменять значением HTTP
`Host`.

### 2. Handoff модуля

В Nginx access log есть диагностическая переменная `handoff`. В той же строке
она может остаться `not_sent`, потому что встроенный log-модуль Nginx вычисляет
переменные раньше обработчика handoff. Факт доставки проверяйте по появлению
события в `/api/connections`, значению `/api/ingest` и по ошибкам Go-приёмника
в журнале. `/api/ingest` показывает состояния `received`, `decoded`, `stored`
и `dropped` на стороне сборщика. Эти счётчики не доказывают, что Nginx
отправил каждый запрос: отсчёт начинается только после попадания датаграммы в
socket сборщика.

Возможные значения:

- `sent` — событие принято Unix socket;
- `unavailable` — Go-сборщик не поднял socket до старта Nginx;
- `dropped_backpressure` — socket переполнен;
- `dropped_error` — ошибка отправки;
- `oversize` — событие превысило лимит 4096 bytes.

`dropped_backpressure` — сигнал нагрузки, а не причина отклонять запрос.
Счётчики `/api/ingest` начинают отсчёт только после попадания датаграммы в
socket сборщика. Для расследования потерь до этой точки используйте handoff
Nginx и access log. Для полного аудита нужна отдельная очередь с гарантией
доставки.

### 3. Client IP

Настройте доверенные proxy hop явно, например:

```nginx
set_real_ip_from 10.0.0.0/8;
real_ip_header X-Forwarded-For;
real_ip_recursive on;
```

CIDR должен содержать только доверенные proxy hop. Нельзя принимать любой
`X-Forwarded-For` от клиента.

## Типовые инциденты

### «Всё стало 502»

Проверить:

1. `nginx -t`;
2. загрузился ли `.so` через `load_module`;
3. поднят ли upstream `127.0.0.1:8080`;
4. не занял ли Go нужный порт;
5. не менялась ли версия Nginx без пересборки модуля.

### «В отчёте нет SNI»

Возможные причины:

- клиент не отправил SNI;
- TLS терминируется раньше Nginx;
- запрос пришёл не на TLS listener;
- используется passthrough, а HTTP-модуль пытается прочитать HTTP-phase переменные;
- включён ECH, и внешний наблюдатель не видит открытое имя.

### «Модуль тормозит Nginx»

Проверьте, что используются именно `SOCK_DGRAM` и `O_NONBLOCK`. Модуль не
должен вызывать HTTP, DNS, Go/cgo или SQLite, а также ждать ответа сборщика в
фазе обработки запроса.

## Рекомендация по эксплуатации

1. Сначала включить модуль на одном canary ingress.
2. Сравнивать число событий в `/api/connections` с access log; `handoff` в
   строке не использовать как единственный критерий доставки.
3. Хранить отчёты короткое время и маскировать секреты из URL/query.
4. Обновлять Nginx и native module согласованной версионной парой.
5. Использовать fingerprint только как диагностическое свидетельство, а не как
   удостоверение клиента.

Аварийный bypass: выставить `httpfin off;` в затронутом `server` или
`location`, выполнить `nginx -t` и reload Nginx. Это убирает handoff модуля из
пути обработки запроса, сохраняя proxy и access log. Go-сборщик можно оставить
запущенным до выяснения причины. Возвращать `httpfin on;` следует сначала на
canary и повторить сравнение событий с access log.
