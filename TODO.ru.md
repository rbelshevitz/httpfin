# План развития httpfin

Следующее направление — анти-бот-привратник перед API. Текущий inspector
нужно сохранить как read-only диагностический компонент, а policy-логику
вводить отдельными понятными этапами.

## Сначала — надёжность

- [x] Собрать Nginx-модуль под закреплённый исходный код Nginx и пройти полный
  container smoke test.
- [ ] Добавить fuzz-тесты для C-сборщика JSON и Go-декодера событий.
- [ ] Добавить нагрузочные тесты для TLS handshakes, HTTP/2 и Unix datagram.

## Слой наблюдения anti-bot

- [ ] Определить версионируемый формат наблюдения: заявленные, согласованные и
  вычисленные поля хранить отдельно.
- [x] Добавить первый loader baseline-файла для TLS termination в Go; явно
  хранить неизвестные signatures и не превращать mismatch напрямую в ban.
- [ ] Добавить точку интеграции с wire-observer для raw ClientHello, JA3/JA4 и
  признаков HTTP/2; недоступные поля оставлять явно пустыми.
- [ ] Для каждого признака хранить источник, точку наблюдения, proxy hop и
  версию baseline.
- [ ] Добавить privacy controls: allowlist полей, redaction, срок хранения и
  аудит доступа.
- [ ] Делать baseline отдельным для endpoint, а не одним глобальным профилем
  браузера.

## Policy-слой: API-привратник

- [ ] Вынести policy в отдельный сервис или модуль, который получает
  наблюдения и контекст приложения; не помещать rule engine в Nginx worker.
- [ ] Начать с режима **observe-only**: показывать причины и трассу решения,
  не изменяя запрос.
- [x] Добавить первый anti-bot evaluator по двум сигналам с
  `decision=observe`; не включать `decision=deny` до появления wire-level
  baseline и replay-тестов.
- [ ] Добавить **shadow mode**: считать решения `challenge/throttle/allow` и
  сравнивать их с фактическими результатами операторов.
- [ ] Определить объяснимые действия: `allow`, `review`, `challenge`,
  `throttle`, `deny`, с кодом причины и сроком действия.
- [ ] Настроить fail-open и fail-closed отдельно для endpoint и действия;
  задать безопасное поведение при недоступности collector.
- [ ] Добавить подписанную и аутентифицированную передачу признаков от edge к
  API, чтобы приложение не доверяло произвольному клиентскому fingerprint
  header.
- [ ] Подключать WAF, API gateway или service mesh только после проверки
  результатов shadow mode.

## Интеграция с Fail2ban

Архитектурное решение: Fail2ban должен читать отдельный security-log или
journal, а не парсить `/api/report` и JSON наблюдений. `httpfin` поставляет
события, Fail2ban на хосте считает пороги и применяет firewall action. См.
[`docs/FAIL2BAN.ru.md`](docs/FAIL2BAN.ru.md).

- [x] Определить стабильный построчный формат security event для Fail2ban:
  timestamp, source IP, endpoint, reason code, confidence и correlation ID;
  не писать в ban-log URL, токены, cookies и полный User-Agent.
- [x] Добавить opt-in security log или exporter отдельно от человекочитаемого
  отчёта и JSON API наблюдений.
- [x] Подготовить пример `jail.local` и filter для повторяющихся и объяснимых
  событий: превышение rate limit, abuse авторизации, malformed requests или
  решение будущего policy-gatekeeper.
- [ ] Добавить режим `dry-run`/observe-only и явный порог срабатываний до
  включения действия firewall.
- [ ] Описать настройку trusted proxy, чтобы Fail2ban блокировал реальный IP
  только при правильно настроенной и доверенной цепочке proxy.
- [ ] Проверить IPv4, IPv6, NAT, общий корпоративный egress, CDN-адреса,
  container network и firewall backends `nftables`/`iptables`.
- [ ] Явно описать срок блокировки, recovery, allowlist, ignore rules и
  аварийный unban; значение по умолчанию должно безопасно сохранять доступность
  API.
- [ ] Не блокировать только по User-Agent, JA3/JA4 или heuristic browser
  profile; объединять повторяемое поведение с rate, authentication, session и
  контекстом endpoint.
- [ ] Добавить integration tests: вредоносный клиент вызывает временную
  блокировку, но не может внедрить произвольный source IP или reason в поток
  событий Fail2ban.

Источники: [настройка jail в Fail2ban](https://github.com/fail2ban/fail2ban/blob/master/config/jail.conf),
[разработка и тестирование filter](https://github.com/fail2ban/fail2ban/blob/master/FILTERS),
[реальный IP за reverse proxy](https://fail2ban.dev/troubleshooting/reverse-proxy-ip/)
и [примеры для Nginx](https://fail2ban.dev/guides/nginx/).

## Качество обнаружения

- [ ] Измерять false positive и false negative по endpoint и версии клиента.
- [ ] Проверять обновления браузеров и SDK, proxy, NAT, мобильные сети, ECH,
  TLS resumption и повторное использование HTTP/2-соединений.
- [ ] Рассматривать fingerprint как признак корреляции и объединять его с
  rate, authentication, session и application behavior.
- [ ] Собрать replay fixtures: обычные браузеры, SDK, automation и известные
  партнёрские клиенты.

## Работа оператора

- [ ] Добавить в отчёт объяснение каждой причины простым языком.
- [ ] Добавить OpenAPI endpoints для наблюдений, baseline, preview policy и
  decision trace.
- [ ] Добавить метрики потери событий, доступности observer, задержки policy и
  количества действий.
- [ ] Добавить руководство по canary rollout и аварийный bypass.
