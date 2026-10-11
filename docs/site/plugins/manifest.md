# Manifest и capabilities

Подробный Manifest, settings schema и invocation-mode contract принадлежат
подключённому plugin; общий REST lifecycle shape задаёт Plugin SDK. Manifest
является plugin-owned документом, который Plugin SDK передаёт без интерпретации
продуктовых полей. Core может проверить его общую JSON-структуру и plugin-owned
settings schema, но не выводит capability→mode registry и не проверяет
доступность методов других plugins. `pluginprotocol` Manifest не переносит и
lifecycle не обслуживает.

Instance metadata и desired settings принадлежат Core. Settings JSON
передаётся Management API непосредственно как plugin-owned JSON object, без
общей обёртки. Core сохраняет точные исходные UTF-8 bytes и их SHA-256 в
versioned SQLite generation, отклоняет duplicate keys и проверяет schema без
декодирования и пересериализации продуктовых полей. Core вызывает REST
`Reload(generation)`, после чего plugin сам pull-ит эту immutable revision и
подтверждает совпадающий digest до readiness. Plugin не читает application
environment/config files.

Routing и capabilities связываются через traffic JSON settings Server plugin,
а не Core route DSL. Server plugin проверяет режим по своему поддерживаемому
enum, строит runtime-конфигурацию и напрямую вызывает target через generic
`pluginprotocol`. Существование и авторизация product method определяются на
peer-границе при вызове; v1 не вводит `DispatchApply` или централизованный
capability registry.

## Typed manifest provider

SDK предоставляет `models.ManifestDescriptor` и `presentation.NewManifestProvider`.
Composition root plugin передаёт typed descriptor и plugin-owned JSON schema:

```go
metadata, err := presentation.NewManifestProvider(
    models.ManifestDescriptor{ID: "echo", Version: "1.0.0", Capabilities: caps},
    []byte(`{"type":"object"}`),
)
handlers, err := presentation.NewHandlerSet(presentation.HandlerConfiguration{
    Contracts: contracts, Lifecycle: lifecycle, Readiness: readiness,
    Registration: registration, Metadata: metadata, Metrics: metrics,
    AdminSurface: adminSurface, AdminActions: adminActions,
})
```

SDK сам добавляет `manifestVersion: studio-runtime/v1`, `sdkVersion`, default
`pluginprotocol` version и стандартное `runtime.healthEndpoint: /healthz`;
кастомный health path отвергается. SDK проверяет лимиты, canonicalizes JSON и
публикует документы на своих REST endpoints. Canonical live response получает
Core через свой SDK client. CLI и Studio не вызывают эти endpoints напрямую;
они видят только Core-backed Management API responses.

SDK также фиксирует универсальные observability routes: `GET /healthz` и
`GET /metrics`. Их точные paths, methods, status/body, content types и
Prometheus exposition принадлежат SDK contract; plugin composition root
передаёт только lifecycle/readiness и SDK collector. SDK отвергает собственный
health handler с другим путём, семантикой или форматом.
Поэтому Server, forms-db, Echo и любой новый plugin имеют одинаковую health и
metrics поверхность без product-specific копий.

Wire schema и examples не копируются в эту документацию; каноническая модель
описана в [целевой архитектуре Core](../core/architecture/target),
[plugin deployment](../core/architecture/plugin-deployment) и
[plugin Admin Pages](admin-pages).
