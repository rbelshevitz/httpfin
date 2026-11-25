# Fail2ban integration

This guide shows how to connect `httpfin` to Fail2ban on a Linux host. The
integration is opt-in. `httpfin` writes security events; Fail2ban reads them
and may apply a temporary firewall ban. `httpfin` never runs firewall commands
from an Nginx request worker.

## What is implemented now

- `httpfin_security_log=/path/security.log` enables the security log;
- HTTP `401`, `403`, and `429` produce neutral `http_status_denied` events;
- anti-bot evidence uses `event=bot_suspected`;
- only `decision=deny` is accepted by the anti-bot filter;
- `observe`, `review`, and `challenge` never trigger that filter;
- User-Agent, JA3/JA4, or a browser label alone never creates a ban.

The current anti-bot policy is observe-only. A real `decision=deny` requires
reviewed wire-level baselines and endpoint-specific rules. Do not enable the
jail expecting the current build to block ordinary browser-like traffic.

The next baseline stage can load a JSON file with
`httpfin_baseline_file=/etc/httpfin/baseline.json`. The example is
[`configs/baseline.example.json`](../configs/baseline.example.json). It marks
known ClientHello summaries as `matched`, `mismatch`, or `unknown` and records
the baseline source in reports. It does not create a ban by itself, and the
placeholder values in the example must be replaced with measurements from the
team's own clients.

## How the pieces connect

```text
client
  ↓
Nginx / Go TLS observer
  ↓
/var/log/httpfin/security.log
  ↓  failregex
Fail2ban jail
  ↓  nftables-multiport or another local action
temporary IP ban
```

The JSON API and Squid-style report are for operators and CI. Fail2ban must
read the dedicated line-oriented security log, not scrape `/api/report`.

## 1. Enable the security log

Create a directory readable by the `httpfin` process and by Fail2ban:

```bash
sudo install -d -o httpfin -g fail2ban -m 0750 /var/log/httpfin
sudo touch /var/log/httpfin/security.log
sudo chown httpfin:fail2ban /var/log/httpfin/security.log
sudo chmod 0640 /var/log/httpfin/security.log
```

Start `httpfin` with:

```bash
httpfin_security_log=/var/log/httpfin/security.log \
  /usr/local/bin/httpfin
```

The logger creates a missing file with mode `0640`, but it does not create
parent directories or change group ownership. Set those outside the process.

Each line has bounded, allowlisted fields:

```text
httpfin-security ts=2026-11-25T18:00:00Z src=198.51.100.23 event=http_status_denied endpoint=/v1/login status=401 reason=status_401
```

The logger removes the query string and rejects unparseable source addresses.
It does not write cookies, authorization headers, tokens, full User-Agent
values, or fingerprint blobs.

## 2. Install the Fail2ban files

Copy the repository examples:

```bash
sudo install -D -m 0644 configs/fail2ban/filter.d/httpfin-bot-deny.conf \
  /etc/fail2ban/filter.d/httpfin-bot-deny.conf
sudo install -D -m 0644 configs/fail2ban/jail.d/httpfin-bot.local \
  /etc/fail2ban/jail.d/httpfin-bot.local
```

The repository also contains `httpfin-http-status.conf`. Keep that filter
disabled unless the team explicitly decides that repeated HTTP denials should
participate in host bans. It is not an anti-bot detector.

## 3. Test the filter before enabling the jail

Create a fixture with a deny event and a non-deny event:

```bash
cat >/tmp/httpfin-fail2ban.log <<'EOF'
httpfin-security ts=2026-11-25T18:00:00Z src=198.51.100.23 event=bot_suspected endpoint=/v1/login decision=deny confidence=0.98 reason=ua_tls_baseline_mismatch
httpfin-security ts=2026-11-25T18:01:00Z src=198.51.100.23 event=bot_suspected endpoint=/v1/login decision=observe confidence=0.95 reason=ua_tls_baseline_mismatch
EOF
fail2ban-regex /tmp/httpfin-fail2ban.log \
  /etc/fail2ban/filter.d/httpfin-bot-deny.conf
```

The first line must match. The `observe` line must not match. Also test a
trusted monitoring address and an IPv6 address before production.

## 4. Enable the jail in stages

The example is intentionally disabled:

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
banaction = nftables-multiport
```

Start with `enabled = false` and inspect matches. Then use a short ban:

```bash
sudo fail2ban-client reload
sudo fail2ban-client status httpfin-bot
sudo nft list ruleset
```

Before a longer rollout, configure `ignoreip` for monitoring, office egress,
partners, and internal NAT. Test recovery:

```bash
sudo fail2ban-client set httpfin-bot unbanip 198.51.100.23
```

The action name and firewall prerequisites vary by distribution. Confirm that
`nftables-multiport` exists and inspect generated rules before enabling a
production jail.

## Reverse proxy: do not ban the proxy

If Nginx is behind a load balancer or CDN, use the real-IP module only for
known proxy networks:

```nginx
set_real_ip_from 10.0.0.0/8;
set_real_ip_from 2001:db8:1234::/48;
real_ip_header X-Forwarded-For;
real_ip_recursive on;
```

Never trust an arbitrary forwarding header from the client. Keep the original
edge address separately for diagnostics. Validate this with a request from an
untrusted network before enabling bans.

## Docker

The container smoke setup writes `/run/httpfin/security.log` inside the
container. A host Fail2ban process cannot read that path after the container
is removed. For a real deployment choose one explicit transport:

- bind-mount a host directory containing `security.log`;
- forward structured security events to journald;
- ship container logs to a host log collector and point Fail2ban at the
  resulting file.

Do not point host Fail2ban at an arbitrary Docker JSON log without testing log
rotation, timestamps, permissions, and the exact line format.

## Troubleshooting

No matches:

```bash
fail2ban-regex /var/log/httpfin/security.log \
  /etc/fail2ban/filter.d/httpfin-bot-deny.conf
sudo fail2ban-client status httpfin-bot
```

Banning the proxy or `127.0.0.1` means the trusted proxy/IP configuration is
wrong. Stop the jail, fix Nginx real-IP handling, and unban the address.

No bans with `decision=observe` is correct: the anti-bot filter deliberately
accepts only `decision=deny`.

## References

- [How Fail2ban works](https://fail2ban.dev/how-it-works/)
- [Fail2ban jail configuration](https://github.com/fail2ban/fail2ban/blob/master/config/jail.conf)
- [Filter development and `fail2ban-regex`](https://github.com/fail2ban/fail2ban/blob/master/FILTERS)
- [Nftables action](https://github.com/fail2ban/fail2ban/blob/master/config/action.d/nftables.conf)
- [Nginx real-IP module](https://nginx.org/en/docs/http/ngx_http_realip_module.html)
