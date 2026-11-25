# httpfin для DevOps-команды

## Что даёт httpfin

httpfin нужен, когда обычного access log недостаточно, чтобы быстро ответить:

- дошёл ли запрос до Nginx и до Go collector;
- какой IP реально видит edge;
- какой SNI и ALPN пришли от клиента;
- был ли выбран HTTP/2 или HTTP/1.1;
- где возникла задержка;
- потерялись ли события из-за backpressure Unix socket;
- можно ли приложить доказательный отчёт к incident ticket.

## Локальный запуск через Docker

```bash
docker build -t httpfin:dev .
docker run --rm -p 8443:8443 httpfin:dev
```

Проверка запроса с SNI:

```bash
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/healthz
```

Открыть UI и отчёт через тот же Nginx:

```bash
curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/

curl -k --resolve inspector.example.test:8443:127.0.0.1 \
  https://inspector.example.test:8443/api/report
```

Docker-режим выше предназначен для наблюдения и smoke-тестов. Для production
используйте отдельный host/systemd-режим: Nginx и `httpfin` работают как службы
хоста, а socket и права доступа управляются systemd.

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
Сначала проверьте `/healthz`, `/api/connections`, права на
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

В отчёте должны появиться SNI, TLS version и negotiated ALPN. Если SNI пустой — это отдельный факт, нельзя подменять его значением HTTP `Host`.

### 2. Handoff модуля

В Nginx access log есть диагностическая переменная `handoff`. В той же строке
она может остаться `not_sent`, потому что встроенный log-модуль Nginx вычисляет
переменные раньше обработчика handoff. Факт доставки проверяйте по появлению
события в `/api/connections` и по ошибкам Go-приёмника в журнале. Отдельных
счётчиков потерь в текущем runtime нет.

Значения:

- `sent` — событие принято Unix socket;
- `unavailable` — Go collector не поднял socket до старта Nginx;
- `dropped_backpressure` — socket переполнен;
- `dropped_error` — ошибка отправки;
- `oversize` — событие превысило лимит 4096 bytes.

`dropped_backpressure` — сигнал нагрузки, а не причина блокировать запрос. Для lossless audit нужен отдельный access log или очередь с подтверждением.

### 3. Client IP

Проверять отдельно:

```nginx
set_real_ip_from 10.0.0.0/8;
real_ip_header X-Forwarded-For;
real_ip_recursive on;
```

CIDR должен содержать только доверенные proxy hop. Нельзя принимать любой `X-Forwarded-For` от клиента.

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

Проверить, что используется именно `SOCK_DGRAM` + `O_NONBLOCK`. Модуль не должен вызывать HTTP, DNS, Go/cgo, SQLite или ждать ответа collector в request phase.

## Рекомендация по эксплуатации

1. Сначала включать модуль на одном canary ingress.
2. Сравнивать число событий в `/api/connections` с access log; `handoff` в
   строке не использовать как единственный критерий доставки.
3. Хранить отчёты короткое время и маскировать секреты из URL/query.
4. Обновлять Nginx и native module одной версионной парой.
5. Использовать fingerprint как диагностический сигнал, а не единственную блокирующую политику.

Аварийный bypass: выставить `httpfin off;` в затронутом `server` или
`location`, выполнить `nginx -t` и reload Nginx. Это убирает handoff модуля из
request path, сохраняя proxy и access log. Go collector можно оставить
запущенным до выяснения причины. Возвращать `httpfin on;` следует сначала на
canary и повторить сравнение событий с access log.
