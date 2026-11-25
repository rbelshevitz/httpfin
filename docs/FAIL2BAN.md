# Fail2ban integration design

`httpfin` can now emit an opt-in Fail2ban security log. It does not execute
firewall commands. Enable it with:

```bash
httpfin_security_log=/var/log/httpfin/security.log go run ./cmd/httpfin
```

The default is disabled. The current logger emits bounded events for HTTP
statuses `401`, `403`, and `429`; it does not know application-specific
authentication or rate-limit semantics yet.

For container smoke tests only, `httpfin_test_endpoint=1` exposes a gated
`/__httpfin_test/unauthorized` endpoint. It is disabled by default and must not
be enabled in a production deployment.

The anti-bot policy is separate. It emits `event=bot_suspected` only when the
current evidence contains two independent mismatch signals. Its first action is
`decision=observe`; it never turns a User-Agent string into a ban. A future
policy adapter may emit `decision=deny` after a reviewed baseline and an
endpoint-specific rule.

## Responsibility split

```text
client
  ↓
Nginx / httpfin observation
  ↓
dedicated security event log or journal
  ↓
Fail2ban filter → jail thresholds → nftables/iptables action
  ↓
temporary host-level ban
```

`httpfin` should describe an event and its evidence. Fail2ban should decide
when repeated events cross a threshold and apply a temporary ban. The Nginx
worker must never invoke Fail2ban or a firewall command synchronously.

## Event format

Use one line per event and a fixed field order so `fail2ban-regex` can test it:

```text
httpfin-security ts=2026-11-25T18:00:00Z src=198.51.100.23 event=auth_failure endpoint=/v1/login status=401 reason=invalid_credentials
```

The first version should contain only bounded, allowlisted fields:

- `src`: normalized source address;
- `event`: a small enum such as `auth_failure`, `rate_limit`, or
  `malformed_request`;
- `endpoint`: normalized route name, not a raw query string;
- `status`: HTTP status;
- `reason`: stable reason code, not free text.

Do not write passwords, cookies, authorization headers, tokens, raw query
strings, complete User-Agent values, or fingerprint blobs into this log.

## Filter and jail prototype

The following is a prototype for a host-mounted log. Keep the jail disabled
until the filter is tested against real samples:

```ini
# /etc/fail2ban/filter.d/httpfin-api.conf
[Definition]
failregex = ^httpfin-security\s+ts=\S+\s+src=<HOST>\s+event=(auth_failure|access_denied|rate_limit)\s+endpoint=\S+\s+status=\d{3}\s+reason=\S+\s*$
ignoreregex =
```

For the anti-bot jail, match only an explicit deny decision:

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

Test the filter before enabling the jail:

```bash
fail2ban-regex /var/log/httpfin/security.log /etc/fail2ban/filter.d/httpfin-api.conf
fail2ban-client status httpfin-api
```

The exact action name and firewall prerequisites are distribution-dependent.
Validate the generated rules with `nft list ruleset` and test an explicit
unban path before production rollout.

## Reverse proxy and Docker boundaries

If Nginx is behind a load balancer or CDN, configure the real-IP module only
for known proxy networks:

```nginx
set_real_ip_from 10.0.0.0/8;
set_real_ip_from 2001:db8:1234::/48;
real_ip_header X-Forwarded-For;
real_ip_recursive on;
```

Never trust an arbitrary client-supplied forwarding header. Preserve the
original edge address separately for diagnostics. In Docker, a host Fail2ban
instance needs a host-visible file, journald entry, or another explicit log
transport; container stdout is not automatically a host file at the path used
by `logpath`.

## Safe rollout

1. Enable the `httpfin_security_log` stream with the jail disabled.
2. Run `fail2ban-regex` against positive and negative fixtures.
3. Enable observe-only reporting and inspect false positives.
4. Use a short temporary ban and a narrow port set.
5. Add allowlists for monitoring, office egress, partners, and internal NAT.
6. Exercise recovery and emergency unban before enabling longer bans.

Never ban solely on User-Agent, JA3/JA4, or a heuristic browser profile. These
signals can be shared, changed, replayed, or unavailable at the observation
point. Combine repeated behavior with rate, authentication, session, and
endpoint evidence.

## References

- [How Fail2ban works](https://fail2ban.dev/how-it-works/)
- [Fail2ban jail configuration](https://github.com/fail2ban/fail2ban/blob/master/config/jail.conf)
- [Fail2ban filter development](https://github.com/fail2ban/fail2ban/blob/master/FILTERS)
- [Nftables action](https://github.com/fail2ban/fail2ban/blob/master/config/action.d/nftables.conf)
- [Nginx real-IP module](https://nginx.org/en/docs/http/ngx_http_realip_module.html)
