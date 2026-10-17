# Manifest и capabilities

Подробный Manifest, settings schema и invocation-mode contract принадлежат
подключённому service; общий REST lifecycle shape задаёт Service SDK. Manifest
является service-owned документом, который Service SDK передаёт без интерпретации
продуктовых полей. Core может проверить его общую JSON-структуру и service-owned
settings schema, но не выводит capability→mode registry и не проверяет
доступность методов других services. `wool` Manifest не переносит и
lifecycle не обслуживает.

Instance metadata и desired settings принадлежат Core. Settings JSON
передаётся Management API непосредственно как service-owned JSON object, без
общей обёртки. Core сохраняет точные исходные UTF-8 bytes и их SHA-256 в
versioned SQLite generation, отклоняет duplicate keys и проверяет schema без
декодирования и пересериализации продуктовых полей. Core вызывает REST
`Reload(generation)`, после чего service сам pull-ит эту immutable revision и
подтверждает совпадающий digest до readiness. Service не читает application
environment/config files.

Routing и capabilities связываются через service-owned JSON settings, а не
через Core route DSL. Service проверяет режим по собственному поддерживаемому
enum, строит runtime-конфигурацию и напрямую вызывает target через generic
`wool`. Существование и авторизация application method определяются на
peer-границе при вызове; v1 не вводит `DispatchApply` или централизованный
capability registry.

## Typed manifest provider

SDK предоставляет `models.ManifestDescriptor` и `presentation.NewManifestProvider`.
Composition root service передаёт typed descriptor и service-owned JSON schema:

```go
metadata, err := presentation.NewManifestProvider(
    models.ManifestDescriptor{ID: "sample", Version: "1.0.0", Capabilities: caps},
    []byte(`{"type":"object"}`),
)
handlers, err := presentation.NewHandlerSet(presentation.HandlerConfiguration{
    Contracts: contracts, Lifecycle: lifecycle, Readiness: readiness,
    Registration: registration, Metadata: metadata, Metrics: metrics,
    AdminSurface: adminSurface, AdminActions: adminActions,
})
```

SDK сам добавляет `manifestVersion: service-runtime/v1`, `sdkVersion`, default
`wool` version и стандартное `runtime.healthEndpoint: /healthz`;
кастомный health path отвергается. SDK проверяет лимиты, canonicalizes JSON и
публикует документы на своих REST endpoints. Canonical live response получает
Core через свой SDK client. CLI и Studio не вызывают эти endpoints напрямую;
они видят только Core-backed Management API responses.

SDK также фиксирует универсальные observability routes: `GET /healthz` и
`GET /metrics`. Их точные paths, methods, status/body, content types и
Prometheus exposition принадлежат SDK contract; service composition root
передаёт только lifecycle/readiness и SDK collector. SDK отвергает собственный
health handler с другим путём, семантикой или форматом.
Поэтому любой подключённый service имеет одинаковую health и
metrics поверхность без product-specific копий.

## Declarative configuration UI

Manifest может содержать необязательный `configurationUI` с версией
`service-configuration-ui/v1`. SDK-модель ограничивает descriptor локализованными
RU/EN группами, JSON Pointer путями к полям, порядком и allowlisted hints:
`auto`, `text`, `textarea`, `number`, `slider`, `toggle`, `select`, `array`,
`resource`, `peer`. Ресурсный hint обязан назвать тип логической ссылки
(`file`, `certificate`, `site` или `artifact`); peer hint ссылается на объявленный
opaque peer contract. Исполняемого JS/HTML, callbacks расширений Studio
(StudioPlugins), remote schemas,
credentials и абсолютных путей в descriptor нет.

Поле `widget: peer` применимо только к JSON Schema string и сохраняет ID
сервиса из текущего Project. `peerContractId` должен быть объявлен самим
сервисом-владельцем descriptor; Studio показывает выбор ID сервиса после
проверки его live manifest через typed CLI → Core. Выбранный сервис должен
объявлять тот же opaque contract ID. UI не сохраняет адреса/методы и не вызывает
peer; бизнес-вызов остаётся data-plane обязанностью сервиса через Wool.
Object-valued peer bindings пока не входят в этот versioned contract: SDK
отклоняет их до появления явного безопасного mapping descriptor.

Manifest дополнительно может объявлять bounded `resources` catalog. Каждый
resource имеет стабильный ID и один из видов `file`, `certificate`, `site` или
`artifact`. Provider `project` обращается к безопасному Project catalog через
Studio host; provider `service-admin` задаёт только page/resource IDs и JSON
Pointers value/label, а не URL или исполняемый запрос. Поле
`configurationUI` с `widget: resource` связывает `resourceKind` и `resourceId`
с объявленным catalog entry того же kind. SDK проверяет эту связь и отвергает
дубликаты, неизвестные provider/kind, секретные поля и path-like значения. Для
`service-admin` перечисление выполняется только через typed CLI → Core
Management API → SDK Admin Surface; прямого Service SDK клиента в Studio/CLI
нет. Необязательная `publication`-ссылка содержит только `pageId` и `actionId`
service-owned artifact action. Перед использованием CLI обязан сверить её с
актуальным Admin Surface, полученным через Core, и убедиться, что action
объявлен именно как artifact action; manifest-ссылка не является разрешением
и не содержит URL, limits, transport или секретов. Core/SDK не интерпретируют
эту привязку. Provider `project` не может объявлять remote publication.
Catalog не содержит значений сертификатов, ключей или иных секретов.

`presentation.NewManifestProvider` проверяет descriptor против service-owned
JSON Schema: локальные field paths и `$ref` должны разрешаться, тип widget должен
соответствовать schema, slider требует `minimum`/`maximum`, а
read-only/write-only/password/sensitive fields нельзя пометить редактируемыми.
Неизвестная версия, widget или внешний `$ref` закрывают metadata fail-closed.
Schema остаётся единственным источником типов, bounds, enum и validation;
descriptor не может их ослабить. SDK детерминированно сортирует группы и поля и
публикует descriptor только внутри live manifest. Core передаёт его как opaque
metadata; CLI возвращает данные Studio через typed Core-backed commands.
Ни CLI, ни Studio не вызывают Service SDK REST напрямую.

Wire schema и examples не копируются в эту документацию; каноническая модель
описана в [целевой архитектуре Core](/core/architecture/target),
[service deployment](/core/architecture/service-deployment) и
[service Admin Pages](/services/admin-pages).
