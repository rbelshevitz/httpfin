# TLS/HTTP2 connection observer: проектная записка

> **Статус: проектная записка.** TLS/HTTP2 connection observer пока не
> реализован в текущем релизе. Отдельного процесса, пути захвата raw-пакетов,
> вычисления стандартных JA3/JA4 и наблюдения за HTTP/2-соединением сейчас нет.

Документ описывает будущий TLS/HTTP2 connection observer и отделяет признаки на
уровне соединения от request-level данных текущего HTTP-модуля.

## Рекомендуемая реализация

Первую версию стоит писать на Go отдельным package и command в этом репозитории.
`crypto/tls` даст доступ к доступным полям `ClientHelloInfo`; слой наблюдения
за HTTP/2 добавляется там, где компонент владеет уже расшифрованным соединением.
Observer не должен работать внутри Nginx worker. Его записи нужно связывать с
наблюдениями httpfin через явный connection ID или request ID.

## План прототипа и библиотеки Go

Первый прототип должен быть локальным и включаемым явно Go-listener. Он
принимает TLS-соединение, назначает connection ID, сохраняет ограниченный
префикс bytes, которые читает `tls.Server`, и выдаёт одну versioned observation
record. Текущий Nginx-модуль менять не нужно; packet capture тоже не нужен.
Прототип полезен даже при ошибке parser: лимит capture, ошибка разбора и исход
handshake сами являются данными о качестве наблюдения.

| Кандидат | Роль в прототипе | Решение |
|---|---|---|
| Go `crypto/tls` | Direct TLS termination, `ClientHelloInfo`, negotiated TLS/ALPN | Обязательная основа; стандартная библиотека |
| Go `x/net/http2` | Наблюдение HTTP/2 frames и SETTINGS там, где observer владеет соединением | Использовать только на connection-aware границе; не зависеть от внутренних деталей `net/http` |
| `github.com/psanford/tlsfingerprint` | Кандидат для JA3/JA4 из входящего ClientHello | Проверить в spike через adapter и fixtures перед подключением |
| `github.com/dreadl0ck/ja3` | Кандидат и comparison oracle для JA3 | Сначала только в тестах; packet-decoding dependency не нужна первому capture path |
| `github.com/google/gopacket` | PCAP/L4 capture и packet decoding | Отложить: не решает наблюдение расшифрованного HTTP/2 |
| `github.com/refraction-networking/utls` | Формирование ClientHello и client impersonation | Не использовать для observer; пригодится для генерации тестовых клиентов |

Готовые fingerprint-пакеты — сторонний код, а не источник истины протокола.
Модель входных данных observer нужно держать независимой от них: тогда parser
можно заменить, сравнить две реализации и сохранить raw normalized fields.

### Этапы прототипа

1. **Capture:** bounded `net.Conn` tee перед `tls.Server`; никакого второго
   reader и чтения `ClientHelloInfo.Conn` в callback.
2. **Parse:** разобрать один полный ClientHello либо вернуть `incomplete`,
   `malformed` или `limit_exceeded`; сохранить normalized cipher, extension,
   group, signature, SNI и ALPN inputs.
3. **Fingerprint:** считать JA3 и JA4 только при полном наборе входных данных;
   отдельно хранить версии algorithm/parser, GREASE policy, normalized input и
   digest.
4. **HTTP/2:** переходить к нему только когда ALPN выбрал `h2` и observer владеет
   расшифрованным соединением; сохранять preface, initial SETTINGS, ACK и
   summary streams/frames — без значений headers и body.
5. **Join:** публиковать connection evidence отдельно, а requests связывать по
   доверенному `connection_id` и HTTP/2 `stream_id`. Нельзя угадывать связь по IP
   и времени.

### Fixtures и критерии готовности

Для code review прототип должен иметь offline fixtures для TLS 1.2 и 1.3, SNI и
его отсутствия, `h2` и `http/1.1`, GREASE, resumption, HelloRetryRequest,
обрезанного и чрезмерно большого ClientHello. Трафик можно генерировать
стандартным Go-клиентом, `curl` и browser-like clients, но их User-Agent не
является эталоном.

Каждый fixture должен проверять parsed fields, canonical JA3/JA4 input, digest,
статус capture и failure state. Direct-TLS smoke должен доказать, что обрезанный
capture не ломает handshake и что request path Nginx не изменился. На соединение
должен быть жёсткий memory limit; raw payload по умолчанию не попадает в обычный
отчёт.

Первый milestone должен закончиться на capture ClientHello и fingerprints.
HTTP/2 — второй milestone: стандартный `net/http` обычно владеет соединением,
поэтому frame observer требует явной точки владения или tee.

## Что доступно в разных точках

| Точка наблюдения | TLS ClientHello | Точный JA3/JA4 | Признаки HTTP/2-соединения | HTTP-поля |
|---|---:|---:|---:|---:|
| Go TLS listener до `tls.Server` | доступны raw bytes | да, при наличии parser | только при instrumented HTTP/2 | да |
| callback Go `tls.ClientHelloInfo` | только parsed fields | не гарантируется | нет | позже |
| Nginx HTTP module после TLS termination | нет | нет | нет | да |
| Nginx `stream ssl_preread` | summary SNI/ALPN/version | нет | нет | нет |
| L4 sidecar до TLS termination | доступны raw bytes | да, при наличии parser | только при расшифрованном HTTP/2 | plaintext headers нет |

Текущий Nginx-модуль остаётся request-level адаптером. Он не должен объявлять
JA3/JA4 или HTTP/2 fingerprint по данным своего request-level hook.

## Схема записи наблюдения

Каждый признак должен храниться вместе с происхождением, а не как
непрозрачная строка. Первая версия схемы должна быть явной и совместимой с
будущим расширением:

```json
{
  "schema_version": "observation.v1",
  "connection_id": "tls-01H...",
  "request_id": "req-01H...",
  "signal": {
    "name": "tls.ja4",
    "value": "t13d1516h2_...",
    "state": "inferred",
    "source": "go-tls-http2-observer",
    "observation_point": "before_tls_server",
    "parser_version": "clienthello/1",
    "algorithm_version": "ja4-tls/1",
    "captured_at": "2026-11-25T18:00:00Z"
  }
}
```

Состояния признаков нужно использовать единообразно:

- `declared`: заявлено клиентом, например User-Agent или offered ALPN;
- `negotiated`: выбрано протоколом, например TLS version или ALPN;
- `inferred`: выведено httpfin из одного или нескольких наблюдений;
- `unknown`: признак ожидался, но данных недостаточно или они неоднозначны;
- `unavailable`: данная точка наблюдения не может получить поле.

`source` обозначает producer, `observation_point` — место, где были видны
байты, а версии parser/algorithm делают последующие сравнения
воспроизводимыми. Пустое значение нельзя трактовать как mismatch. Ошибка
parser должна быть `unknown` с ограниченным error code, а не выдуманным
fingerprint.

Connection и request identity нужно хранить раздельно. Одно TLS-соединение
может обслуживать много HTTP/2 requests, а request может прийти без записи
TLS/HTTP2 observer. Нельзя связывать записи только по source IP и времени.

## TLS

TLS 1.3 допускает произвольный порядок расширений ClientHello, кроме отдельных
ограничений самого протокола (например, `pre_shared_key` должен быть последним).
Для fingerprint нужны исходные списки cipher и
extensions, GREASE policy, supported groups, point formats, signature
algorithms, ALPN, состояние SNI и предложенные версии TLS. Parsed summary полезна
как evidence, но не равна исходному wire-сообщению.

`tls.ClientHelloInfo` в Go отдаёт cipher suites, curves, signature schemes,
ALPN, supported versions и IDs extensions. Его `Conn` нельзя читать или писать
в callback. Поэтому raw capture должен быть в wrapper над соединением до TLS
parser либо в отдельном observer.

TLS 1.3 resumption, HelloRetryRequest, ECH и отсутствие SNI должны получать
явный статус `unknown` или отдельный observation state. Их нельзя автоматически
считать mismatch браузера. GREASE нужно обрабатывать по правилам выбранного
алгоритма; само наличие GREASE не является признаком не-браузера.

## JA3 и JA4

JA3 и JA4 — соглашения fingerprinting, а не поля TLS-протокола. Рядом с digest
нужно хранить нормализованные входные данные, чтобы повторить расчёт после
изменения алгоритма. Для каждой записи нужны method version, parser version,
GREASE policy и точка наблюдения.

Сначала стоит реализовать только TLS JA4 и зафиксировать версию upstream-
описания. JA4H, JA4T и остальные методы JA4+ нельзя молча называть JA4. В
репозитории JA4 лицензия TLS JA4 отделена от лицензий остальных методов JA4+;
перед распространением производного кода нужно проверить upstream license.

## HTTP/2

SETTINGS относится ко всему соединению, а не к отдельному request. Request-level
access-log не может надёжно приписать initial SETTINGS каждому запросу, особенно
при multiplexing. Нужны connection ID и связь request ID с этим соединением.

Первая запись HTTP/2 должна сохранять:

- наличие client connection preface;
- порядок initial SETTINGS pairs;
- поведение SETTINGS ACK;
- initial WINDOW_UPDATE и PRIORITY/PRIORITY_UPDATE, если они наблюдались;
- summary HEADERS/CONTINUATION без значений заголовков;
- reuse соединения и число streams.

Оборванное или malformed-соединение нужно хранить как incomplete evidence, а не
тихо считать mismatch браузера.

## Bounded tee и правила связи

Direct-TLS capture должен быть пассивным bounded tee, а не вторым reader TLS:

1. принять raw connection и назначить `connection_id` до запуска TLS parser;
2. копировать каждый байт, который читает `tls.Server`, в буфер фиксированного
   размера до достижения лимита;
3. всегда возвращать полный результат чтения TLS parser, включая байты, которые
   после достижения лимита уже не сохраняются;
4. отдельно записывать `captured_bytes`, `capture_limit`, `truncated`,
   `read_error` и `handshake_result`;
5. при ошибке capture или parser продолжать обычный TLS path, если это возможно,
   и помечать evidence как incomplete;
6. не читать `ClientHelloInfo.Conn` из callback и не создавать неограниченную
   goroutine или queue для capture.

Лимит должен настраиваться, но оставаться жёстко ограниченным. Его значение по
умолчанию должно покрывать обычный ClientHello, а чрезмерно большой handshake
может занимать только предусмотренный буфер. Лимит capture ограничивает
сохранённые evidence, но не размер TLS-сообщения: TLS всё равно должен получить
исходные байты.

Правила связи:

- `connection_id` создаётся в первой точке наблюдения и не меняется в течение
  TCP/TLS-соединения;
- HTTP/2 evidence связывается с request через `connection_id` и `stream_id`;
- Nginx event использует свой `request_id`; raw TLS можно связать только при
  наличии доверенного connection или handoff ID;
- proxy hops получают отдельные записи и не объединяются только из-за
  совпадения адреса источника;
- при отсутствии ID создаются отдельные записи с
  `join_state=unavailable`, без догадок о связи.

## Рекомендуемый порядок работ

1. Сделать Go direct-TLS path с bounded tee начальных bytes до того, как их
   потребит `tls.Server`.
2. Парсить и хранить versioned raw ClientHello summary.
3. Считать JA3/JA4 в отдельном package с golden fixtures и metadata версии
   parser.
4. Добавить connection ID и connection-aware HTTP/2 observer.
5. Оставить Nginx HTTP module request-level и связывать observations через
   явный request/handoff identifier, если topology это позволяет.

Не добавлять raw packet capture, eBPF или второй Nginx-модуль, пока direct TLS
path не получит fixtures и измеряемые loss counters.

## Источники

- [RFC 8446: TLS 1.3](https://www.rfc-editor.org/rfc/rfc8446.html)
- [RFC 6066: TLS extensions, включая Server Name Indication](https://www.rfc-editor.org/rfc/rfc6066.html)
- [RFC 8701: GREASE](https://www.rfc-editor.org/rfc/rfc8701.html)
- [RFC 9849: TLS Encrypted Client Hello](https://www.rfc-editor.org/rfc/rfc9849.html)
- [Go `crypto/tls.ClientHelloInfo`](https://pkg.go.dev/crypto/tls#ClientHelloInfo)
- [Go `x/net/http2` frame API](https://pkg.go.dev/golang.org/x/net/http2)
- [RFC 9113: HTTP/2](https://www.rfc-editor.org/rfc/rfc9113.html)
- [Nginx stream SSL preread](https://nginx.org/en/docs/stream/ngx_stream_ssl_preread_module.html)
- [`psanford/tlsfingerprint`: кандидат Go-библиотеки для JA3/JA4](https://github.com/psanford/tlsfingerprint)
- [`dreadl0ck/ja3`: кандидат Go-библиотеки для JA3](https://github.com/dreadl0ck/ja3)
- [`google/gopacket`: packet и PCAP decoding](https://github.com/google/gopacket)
- [`refraction-networking/utls`: client-side TLS fingerprint construction](https://github.com/refraction-networking/utls)
- [JA4 technical details](https://github.com/FoxIO-LLC/ja4/blob/main/technical_details/JA4.md)
