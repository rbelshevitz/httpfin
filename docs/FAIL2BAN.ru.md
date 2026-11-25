# Интеграция с Fail2ban

Эта инструкция показывает отдельный host-level режим для Linux-хоста. Fail2ban
не является частью Docker Compose и не должен запускаться внутри контейнера
`httpfin`.

В Docker-режиме запускаются Nginx и Go-наблюдатель. В режиме блокировки на хосте
запускаются Nginx/`httpfin`, security-log и Fail2ban, причём Fail2ban работает
с журналом и firewall самого хоста.

Интеграция включается явно: `httpfin` пишет security-события, Fail2ban читает
их и при необходимости временно блокирует IP через firewall. `httpfin` не
запускает firewall-команды из Nginx worker.

## Что уже работает

- `httpfin_security_log=/path/security.log` включает security-log;
- HTTP `401`, `403` и `429` дают нейтральные события `http_status_denied`;
- анти-бот-признаки используют `event=bot_suspected`;
- anti-bot filter принимает только `decision=deny`;
- `observe`, `review` и `challenge` не вызывают блокировку;
- один User-Agent, JA3/JA4 или browser profile не создаёт ban.

Совпадение `failregex` само по себе ничего не блокирует. Для реального бана
одновременно нужны: запись в security-log, совпадение фильтра, включённый jail
(`enabled = true`), выбранный firewall-action и права Fail2ban на изменение
firewall. `fail2ban-regex` только проверяет совпадения и firewall не меняет.
Пример jail в репозитории намеренно выключен, поэтому установка файлов не
включает блокировку.

Текущая anti-bot policy работает в режиме наблюдения. Настоящий
`decision=deny` появится только после проверки wire-level baseline и правил для
конкретных endpoint. Включать jail с ожиданием автоматической блокировки
обычных browser-like клиентов пока нельзя.

Следующий baseline-слой можно подключить через
`httpfin_baseline_file=/etc/httpfin/baseline.json`. Пример находится в
[`configs/baseline.example.json`](../configs/baseline.example.json). Он отмечает
известные ClientHello summary как `matched`, `mismatch` или `unknown` и пишет
источник baseline в отчёт. Сам по себе baseline не создаёт ban; значения в
примере нужно заменить измерениями собственных клиентов команды.

## Как связаны компоненты

```text
клиент
  ↓
Nginx / Go TLS observer
  ↓
/var/log/httpfin/security.log
  ↓  failregex
Fail2ban jail
  ↓  nftables[type=multiport] или другой локальный action
временная блокировка IP
```

JSON API и Squid-style report предназначены для операторов и CI. Fail2ban
должен читать отдельный построчный security-log, а не парсить `/api/report`.

## 1. Включить security-log

Создайте каталог, доступный процессу `httpfin` и Fail2ban:

```bash
sudo install -d -o httpfin -g fail2ban -m 0750 /var/log/httpfin
sudo touch /var/log/httpfin/security.log
sudo chown httpfin:fail2ban /var/log/httpfin/security.log
sudo chmod 0640 /var/log/httpfin/security.log
```

Запустите `httpfin` с переменной:

```bash
httpfin_security_log=/var/log/httpfin/security.log \
  /usr/local/bin/httpfin
```

Logger создаёт отсутствующий файл с mode `0640`, но не создаёт родительские
каталоги и не меняет владельца или группу. Это нужно настроить заранее.

Строка имеет ограниченный формат:

```text
httpfin-security ts=2026-11-25T18:00:00Z src=198.51.100.23 event=http_status_denied endpoint=/v1/login status=401 reason=status_401
```

Logger убирает query string и отклоняет неразбираемый адрес источника. В файл не
попадают cookies, Authorization headers, tokens, полный User-Agent и fingerprint
blobs.

## 2. Установить файлы Fail2ban

Скопируйте готовые файлы из репозитория:

```bash
sudo install -D -m 0644 configs/fail2ban/filter.d/httpfin-bot-deny.conf \
  /etc/fail2ban/filter.d/httpfin-bot-deny.conf
sudo install -D -m 0644 configs/fail2ban/jail.d/httpfin-bot.local \
  /etc/fail2ban/jail.d/httpfin-bot.local
```

В репозитории есть и `httpfin-http-status.conf`. Оставьте его выключенным,
если команда специально не решила учитывать повторяющиеся HTTP-denial при
блокировках. Это не anti-bot detector.

## 3. Проверить filter до включения jail

Создайте fixture с deny и observe:

```bash
cat >/tmp/httpfin-fail2ban.log <<'EOF'
httpfin-security ts=2026-11-25T18:00:00Z src=198.51.100.23 event=bot_suspected endpoint=/v1/login decision=deny confidence=0.98 reason=ua_tls_baseline_mismatch
httpfin-security ts=2026-11-25T18:01:00Z src=198.51.100.23 event=bot_suspected endpoint=/v1/login decision=observe confidence=0.95 reason=ua_tls_baseline_mismatch
EOF
fail2ban-regex /tmp/httpfin-fail2ban.log \
  /etc/fail2ban/filter.d/httpfin-bot-deny.conf
```

Первая строка должна совпасть. Строка с `observe` — не должна. До production
также проверьте trusted monitoring IP и IPv6.

## 4. Включать jail поэтапно

Пример из репозитория намеренно выключен:

```ini
[httpfin-bot]
enabled = false
filter = httpfin-bot-deny
logpath = /var/log/httpfin/security.log
backend = auto
maxretry = 3
findtime = 10m
bantime = 10m
port = http,https
banaction = nftables[type=multiport]
```

Сначала оставьте `enabled = false` и проверьте matches. Только после проверки
trusted proxy, allowlist и тестового `deny` включайте jail в отдельном `.local`:

```ini
[httpfin-bot]
enabled = true
filter = httpfin-bot-deny
logpath = /var/log/httpfin/security.log
banaction = nftables[type=multiport]
action = %(action_)s
maxretry = 3
findtime = 10m
bantime = 10m
ignoreip = 127.0.0.1/8 ::1
```

В старых пакетах Ubuntu может использоваться имя `nftables-multiport`. Если
современный action отсутствует, проверьте доступные actions командой
`fail2ban-client -d` и используйте имя, которое предоставляет установленная
версия Fail2ban.

После этого используйте короткий ban:

```bash
sudo fail2ban-client reload
sudo fail2ban-client status httpfin-bot
sudo nft list ruleset
```

До rollout добавьте `ignoreip` для мониторинга, офисного egress, партнёров и
внутреннего NAT. Проверьте восстановление:

```bash
sudo fail2ban-client set httpfin-bot unbanip 198.51.100.23
```

Название action и требования к firewall зависят от дистрибутива. Убедитесь,
что выбранный action (`nftables[type=multiport]` или совместимый legacy-вариант)
доступен, и проверьте созданные правила до включения production jail.

## Reverse proxy: нельзя блокировать proxy

Если Nginx находится за load balancer или CDN, real-IP module нужно включать
только для известных proxy-сетей:

```nginx
set_real_ip_from 10.0.0.0/8;
set_real_ip_from 2001:db8:1234::/48;
real_ip_header X-Forwarded-For;
real_ip_recursive on;
```

Нельзя доверять произвольному forwarding header от клиента. Исходный edge-адрес
храните отдельно. Перед включением ban проверьте запросом из недоверенной сети,
что в security-log попадает реальный клиентский IP.

## Почему это не Docker-режим

Пример `docker compose` предназначен для наблюдения и smoke-тестов. Не нужно
добавлять Fail2ban в этот Compose и не нужно выдавать контейнеру доступ к
firewall хоста.

Если приложение обязательно работает в контейнере, это уже отдельная схема:
security-log должен быть явно доступен host Fail2ban через bind-mount или
проверенный сборщик журналов. Произвольный Docker JSON log использовать нельзя:
сначала нужно проверить rotation, временные метки, права и точный формат строк.
Для штатной реализации Fail2ban предпочтителен запуск `httpfin` и Nginx на
хосте либо отдельная host-интеграция с журналом.

## Диагностика

Нет совпадений:

```bash
fail2ban-regex /var/log/httpfin/security.log \
  /etc/fail2ban/filter.d/httpfin-bot-deny.conf
sudo fail2ban-client status httpfin-bot
```

Если заблокирован proxy или `127.0.0.1`, неправильно настроен trusted proxy/IP.
Остановите jail, исправьте real-IP в Nginx и снимите ban.

Отсутствие ban для `decision=observe` — правильное поведение: anti-bot filter
принимает только `decision=deny`.

## Источники

- [Как работает Fail2ban](https://fail2ban.dev/how-it-works/)
- [Настройка jail](https://github.com/fail2ban/fail2ban/blob/master/config/jail.conf)
- [Разработка filter и `fail2ban-regex`](https://github.com/fail2ban/fail2ban/blob/master/FILTERS)
- [Nftables action](https://github.com/fail2ban/fail2ban/blob/master/config/action.d/nftables.conf)
- [Nginx real-IP module](https://nginx.org/en/docs/http/ngx_http_realip_module.html)
