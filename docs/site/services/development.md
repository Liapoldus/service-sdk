# Разработка сервисов

Сервис реализует собственное продуктовое поведение и подключает общую
инфраструктуру через два независимых Go-модуля:

- Service SDK — обязательный единый REST lifecycle/control surface: bootstrap,
  Manifest/settings schema, health/readiness, `Reload(generation)`, exact config
  pull, metrics, logging и безопасные ошибки. Rollback — операция Core
  Management API; service получает новое active-поколение через `Reload`.
- `wool` — опциональная generic peer network для service-to-service
  вызовов. Service регистрирует собственные методы/handlers; library не знает
  product names, payload schema или lifecycle Core.

Библиотеки не зависят друг от друга. Canonical import path Service SDK —
`github.com/Liapoldus/service-sdk/v2`; точные REST routes принадлежат SDK, а wire
format и transport profiles — `wool`. Общие contracts не копируются
в service repos.

TLS/mTLS — ответственность используемой библиотеки, не самого service:
Service SDK защищает Core↔service REST, а `wool` — peer-соединения.
Сервис передаёт каждой библиотеке её security configuration/credentials через
публичный API и использует готовые client/listener/call abstractions; он не
реализует TLS, проверку сертификатов или revocation самостоятельно. Trust roots
для двух независимых каналов остаются раздельными. В peer-протоколе явный
plaintext допустим только для TCP loopback в development; remote и production
требуют mTLS. SDK REST production contract v1 требует mTLS. Разработчик может
явно выбрать opt-in loopback-only plaintext profile вызовом
`infrastructure.NewLoopbackHealthServer`; по умолчанию такой listener не
создаётся. SDK принимает только literal loopback TCP bind и сам ограничивает
маршрут `GET /healthz`. `/ready`, identity, Manifest, config schema,
Reload, metrics, admin actions и artifacts остаются mTLS-only. Exact config pull
и secret-grant clients/endpoints всегда используют mTLS; config bytes и secret
values никогда не выдаются через plaintext. TLS error не включает plaintext
fallback. Канонические значения profile находятся в
[`loopback-plaintext-profile.json`](../../../infrastructure/assets/service-sdk/v2/loopback-plaintext-profile.json).
Service только явно вызывает API SDK; собственную обработку TLS, сертификатов
или plaintext transport он не реализует.

### Обязательные observability endpoints

Каждый production service обязан собираться через Service SDK `HandlerSet`.
`GET /healthz` и `GET /metrics` являются универсальными SDK endpoints: SDK
создаёт их сам из versioned contract и общего metrics collector. Product service
не добавляет собственные health/metrics handlers, не меняет пути и не
дублирует response shape. На Core control listener оба endpoint обслуживаются
SDK REST surface с mTLS; отдельный operator-facing listener также монтирует
только SDK `NewObservabilityHandler`, а не собственную реализацию.

Control plane имеет единственный маршрут:

```text
Studio → CLI → Core Management API → Service SDK REST API
```

Data plane: независимые Services → Wool (`github.com/Liapoldus/wool/v3`).
Service SDK не зависит от Wool. StudioPlugins — только расширения редактора,
не runtime services. Каноническое описание SDK: [Service SDK](/service-sdk/).

CLI запускает workload и объявляет Core безопасные replica metadata. Core сам
читает live manifest, config schema, identity и readiness у service. Это
исключает второй CLI→service lifecycle client и сохраняет SDK REST ownership у
Core.

Каждый service владеет собственным Manifest, settings schema, capabilities,
ошибками, Admin Surface и product data. Settings сохраняются Core в SQLite и
получаются service-ом только pull-запросом точной immutable generation после
`Reload`. Management API принимает непосредственно service-owned JSON object в
body, без общей `{config: ...}` обёртки. Core сохраняет исходные JSON bytes,
проверяет синтаксис, дубликаты ключей и schema, не декодируя продуктовые поля и
не пересериализуя документ. Передавать application settings через environment,
argv или application config files запрещено. Secret values извлекаются отдельно
через scoped Core REST grants и не попадают в response, log, trace, error или
audit.

Для service-to-service взаимодействий вызывающий service применяет собственную
authorization policy через generic authorizer `wool`; Core не
публикует и не хранит такую policy и не проксирует payload. Peer connections
идут напрямую с отдельными identities и trust roots. Carrier/security profile настраивается отдельно от
application-level method names и payloads; смена carrier не меняет service
handlers.

См. [создание service](/core/architecture/guide),
[границы SDK/protocol](/core/architecture/protocol),
[deployment](/core/architecture/service-deployment) и
[матрицу приёмки](/core/configuration/acceptance).
