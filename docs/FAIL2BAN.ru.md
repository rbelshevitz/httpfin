# Интеграция с Fail2ban

`httpfin` уже умеет opt-in писать security-log для Fail2ban. Firewall-команды
он не выполняет. Включение:

```bash
httpfin_security_log=/var/log/httpfin/security.log go run ./cmd/httpfin
```

По умолчанию поток выключен. Сейчас logger пишет ограниченные события для
HTTP status `401`, `403` и `429`; прикладной смысл authentication или rate
limit он пока не знает.

Только для container smoke test переменная `httpfin_test_endpoint=1` включает
закрытый endpoint `/__httpfin_test/unauthorized`. По умолчанию он выключен и не
должен включаться в production.

Anti-bot policy отделён от этого общего abuse-сигнала. Он пишет
`event=bot_suspected` только при наличии двух независимых mismatch-признаков.
Первое действие — `decision=observe`; один User-Agent не превращается в бан.
Будущий policy adapter сможет писать `decision=deny` только после проверки
baseline и правила для конкретного endpoint.

## Разделение ответственности

```text
клиент
  ↓
Nginx / наблюдение httpfin
  ↓
отдельный security event log или journal
  ↓
Fail2ban filter → пороги jail → action nftables/iptables
  ↓
временная блокировка на уровне хоста
```

`httpfin` описывает событие и его признаки. Fail2ban считает повторения,
сравнивает их с порогом и применяет временную блокировку. Nginx worker не
должен синхронно запускать Fail2ban или firewall-команды.

## Формат события

Нужна одна строка на событие и фиксированный порядок полей, чтобы проверять
filter через `fail2ban-regex`:

```text
httpfin-security ts=2026-11-25T18:00:00Z src=198.51.100.23 event=auth_failure endpoint=/v1/login status=401 reason=invalid_credentials
```

В первой версии разрешить только ограниченный набор полей:

- `src` — нормализованный адрес источника;
- `event` — небольшой enum: `auth_failure`, `rate_limit`,
  `malformed_request`;
- `endpoint` — нормализованный route name без query string;
- `status` — HTTP status;
- `reason` — стабильный код причины, не произвольный текст.

Нельзя писать сюда пароли, cookies, Authorization headers, tokens, raw query
strings, полный User-Agent или fingerprint blobs.

## Прототип filter и jail

Это только прототип для host-mounted log. До проверки реальных samples jail
должен оставаться выключенным:

```ini
# /etc/fail2ban/filter.d/httpfin-api.conf
[Definition]
failregex = ^httpfin-security\s+ts=\S+\s+src=<HOST>\s+event=(auth_failure|access_denied|rate_limit)\s+endpoint=\S+\s+status=\d{3}\s+reason=\S+\s*$
ignoreregex =
```

Для anti-bot jail нужно принимать только явное решение deny:

```ini
# /etc/fail2ban/filter.d/httpfin-bot-deny.conf
[Definition]
failregex = ^httpfin-security\s+ts=\S+\s+src=<HOST>\s+event=bot_suspected\s+endpoint=\S+\s+decision=deny\s+confidence=\d+\.\d+\s+reason=\S+\s*$
ignoreregex =
```

```ini
# /etc/fail2ban/jail.d/httpfin-api.local
[httpfin-api]
enabled = false
filter = httpfin-api
logpath = /var/log/httpfin/security.log
backend = auto
maxretry = 5
findtime = 10m
bantime = 10m
port = http,https
banaction = nftables-multiport
```

Проверка до включения:

```bash
fail2ban-regex /var/log/httpfin/security.log /etc/fail2ban/filter.d/httpfin-api.conf
fail2ban-client status httpfin-api
```

Имя action и требования к firewall зависят от дистрибутива. После включения
проверять правила через `nft list ruleset` и отдельно тестировать unban.

## Reverse proxy и Docker

Если Nginx находится за load balancer или CDN, real-IP module нужно включать
только для известных proxy-сетей:

```nginx
set_real_ip_from 10.0.0.0/8;
set_real_ip_from 2001:db8:1234::/48;
real_ip_header X-Forwarded-For;
real_ip_recursive on;
```

Нельзя доверять произвольному forwarding header от клиента. Исходный адрес
edge нужно сохранять отдельно. В Docker host Fail2ban должен получить файл,
journald entry или другой явно настроенный transport: stdout контейнера сам по
себе не становится файлом для `logpath`.

## Безопасный rollout

1. Включить `httpfin_security_log` при выключенном jail.
2. Проверить positive и negative fixtures через `fail2ban-regex`.
3. Включить observe-only и проверить false positives.
4. Начать с короткой временной блокировки и узкого набора портов.
5. Добавить allowlist для мониторинга, офиса, партнёров и внутреннего NAT.
6. Проверить recovery и аварийный unban до длинных блокировок.

Нельзя блокировать только по User-Agent, JA3/JA4 или heuristic browser
profile. Эти признаки могут быть общими, изменяться, воспроизводиться или быть
недоступными в конкретной точке наблюдения.

## Источники

- [Как работает Fail2ban](https://fail2ban.dev/how-it-works/)
- [Настройка jail](https://github.com/fail2ban/fail2ban/blob/master/config/jail.conf)
- [Разработка filter](https://github.com/fail2ban/fail2ban/blob/master/FILTERS)
- [Nftables action](https://github.com/fail2ban/fail2ban/blob/master/config/action.d/nftables.conf)
- [Nginx real-IP module](https://nginx.org/en/docs/http/ngx_http_realip_module.html)
