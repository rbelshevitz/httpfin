# Нюансы реализации wire observer

Этот документ фиксирует границу для первого приоритета: raw TLS- и HTTP/2-
признаки. HTTP-модуль не должен изображать, будто request-level hook способен
честно восстановить connection-level fingerprint.

## Что доступно в разных точках

| Точка наблюдения | TLS ClientHello | Точный JA3/JA4 | Признаки HTTP/2-соединения | HTTP-поля |
|---|---:|---:|---:|---:|
| Go TLS listener до `tls.Server` | доступны raw bytes | да, при наличии parser | только при instrumented HTTP/2 | да |
| callback Go `tls.ClientHelloInfo` | только parsed fields | не гарантируется | нет | позже |
| Nginx HTTP module после TLS termination | нет | нет | нет | да |
| Nginx `stream ssl_preread` | summary SNI/ALPN/version | нет | нет | нет |
| L4 sidecar до TLS termination | доступны raw bytes | да, при наличии parser | только при расшифрованном HTTP/2 | plaintext headers нет |

Текущий Nginx-модуль остаётся request-level адаптером. Он не должен объявлять
JA3/JA4 или HTTP/2 fingerprint по данным своего hook.

## TLS

TLS 1.3 допускает произвольный порядок расширений ClientHello, кроме отдельных
ограничений самого протокола. Для fingerprint нужны исходные списки cipher и
extensions, GREASE policy, supported groups, point formats, signature
algorithms, ALPN, состояние SNI и предложенные версии TLS. Parsed summary полезна
как evidence, но не равна исходному wire-сообщению.

`tls.ClientHelloInfo` в Go отдаёт cipher suites, curves, signature schemes,
ALPN, supported versions и IDs extensions. Его `Conn` нельзя читать или писать
в callback. Поэтому raw capture должен быть в wrapper над соединением до TLS
parser либо в отдельном observer.

TLS 1.3 resumption, HelloRetryRequest, ECH и отсутствие SNI должны получать
явный статус `unknown` или отдельный observation state. Их нельзя автоматически
считать mismatch браузера.

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
- [Go `crypto/tls.ClientHelloInfo`](https://pkg.go.dev/crypto/tls#ClientHelloInfo)
- [RFC 9113: HTTP/2](https://www.rfc-editor.org/rfc/rfc9113.html)
- [Nginx stream SSL preread](https://nginx.org/en/docs/stream/ngx_stream_ssl_preread_module.html)
- [JA4 technical details](https://github.com/FoxIO-LLC/ja4/blob/main/technical_details/JA4.md)
