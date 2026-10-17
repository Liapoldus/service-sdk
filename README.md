# Liapoldus Service SDK

Отдельный четырёхслойный Go-модуль общего Core↔service lifecycle для отдельных
процессов и доверенных statically composed services.
Канонический раздел документации — `/service-sdk`; руководство по созданию
runtime Services — [разработка сервисов](/services/development).
Control plane: Studio → CLI → Core → Service SDK REST. Data plane: независимые
services → Wool. Только расширения редактора называются StudioPlugins.
Датированные результаты прежних проверок сохранены в [TODO](./todo);
они не подтверждают интеграцию после breaking nomenclature migration.

## Единственный источник contract

Каноническая межрепозиторная модель v3 находится в
[Core v3 architecture](https://liapoldus.github.io/core/architecture/v3).
REST и in-process adapters обязаны реализовывать одну lifecycle semantics; ни
один adapter не получает automatic fallback на другой.

Для trusted Go composition SDK предоставляет `infrastructure.InProcessReplica`.
Composition root создаёт `NewInProcessConfigurationSourceForInstance`, передаёт
source и service-owned `ConfigurationApplier` в `NewInProcessReplica`, а Core host
публикует immutable active/previous snapshot через `Publish`. Replica не
открывает listener, не читает SQLite и использует тот же `Reload`/ACK/readiness
контракт, что и REST+mTLS client.

Нормативные константы HTTP v2 и v2 lifecycle/poll/loopback определены типизированным
Go-кодом в `infrastructure/contract_definitions.go`. Production loaders сохраняют
свои API и возвращают независимые maps/slices без разбора статического JSON.
Маршруты, методы, media types, статусы, коды, лимиты, deadlines, TLS и outcomes
публикуются в versioned JSON artifacts под `infrastructure/assets/service-sdk/`.
`make contracts` детерминированно обновляет constants и schema документы;
`make contracts-check` проверяет их без записи и входит в `make check`.
Native Go tests проверяют JSON parity, validation и владение mutable values;
TypeScript suite запрещает дублирование wire constants вне типизированного владельца.

Две v2 JSON Schema определяются code-owned декларациями в `infrastructure/schemas.go`.
`PeerDirectorySchema` / `ReplicaLifecycleSchema` и публичные JSON artifacts
генерируются из этих деклараций; runtime не читает generated files.
Ниже описано только то, что меняет поведение потребителя.

## Breaking change в Core

SDK реализует согласованный target contract, и Core обязан перейти на него
одним coordinated breaking change. SDK не содержит совместимых alias-ов,
fallback-ов на старые endpoints и двух lifecycle-моделей: старые
экспортируемые конструкторы и error sentinels не сохранены.

Core обязан:

- **Pull exact generation.** На exact-generation pull публиковать все
  зарегистрированные contract-ом response headers, а не часть: `generation`,
  `sha256`, `schemaVersion`, `generationState`. Зарегистрированные значения
  берутся из типизированного contract. `generationState` публикует ровно два durable slot-а:
  `active` и `previous`. Любой другой slot, любое перечисление поколений и
  подстановка чужого generation не поддерживаются.
- **Reload acknowledgement.** Возвращать типизированный ACK с пятью
  обязательными полями: `generation`, `sha256`, `schemaVersion`, `applied`,
  `outcome`. Ответ без `schemaVersion`/`outcome` SDK не примет.
- **Readiness.** Публиковать readiness с семью полями: `ready`, `generation`,
  `sha256`, `schemaVersion`, `pendingGeneration`, `instanceId`, `replicaId`.
  `generation` — только реально применённое поколение; отказанное поколение
  публикуется в `pendingGeneration` и никогда не выдаётся за активное.
- **Registration.** Публиковать registration с полями, которые регистрирует
  contract (`contractVersion`, `instanceId`, `replicaId`,
  `appliedGeneration`, `ready`).

`Rollback` остаётся операцией Core Management API. Service-side rollback
endpoint не существует, и его появление ломает suite: production-дерево
проверяется на отсутствие любого service-side rollback.

## Lifecycle

`Reload` — это уведомление, а не передача конфигурации: descriptor несёт
только `generation`, `sha256`, `schemaVersion`. Service сам тянет ровно это
поколение и применяет документ.

Политика `Lifecycle.Reload` по порядку: отклонить descriptor, нарушающий
синтаксис; ответить на побайтовый повтор активного descriptor-а без повторного
pull; отклонить descriptor, противоречащий активному поколению, без pull;
pull-нуть точное поколение; отклонить поколение, которое Core больше не
желает; проверить generation, digest и schema version и по объявлению, и по
самим байтам; и только затем вызвать service-owned applier.

SDK никогда не повторяет pull или apply по собственной инициативе, не
применяет документ с непроверенным digest и не заменяет ранее активную
конфигурацию из-за отказа. Повторный `Reload` того же descriptor-а
идемпотентен (`alreadyActive`); расхождение под тем же именем — отдельный
outcome `generationConflict`. Все 15 contract-outcomes имеют каждый свой
status и уникальный код, и suite проверяет, что outcome не отвечает чужим
кодом.

## Product configuration остаётся непрозрачным

Документ — opaque JSON object. SDK сохраняет байты, которые вернул Core, без
remarshal и без нормализации, отклоняет duplicate JSON object keys, проверяет
JSON syntax, размер, generation, digest и schema version, и только после этого
вызывает service-owned `ConfigurationApplier`. Сервис сам валидирует документ по
своей версионированной JSON Schema и атомарно активирует его; при ошибке
предыдущая конфигурация остаётся полностью рабочей, отменённый context
прерывает apply без активации. SDK не интерпретирует ни одного product field,
не знает capabilities, providers, DB drivers и public routes. Product settings,
Manifest и schemas остаются в соответствующих service repos.

## Secrets

`SecretManager` выдаёт и погашает scoped grant-ы для operational secrets.
Во время вызова service-owned applier grant автоматически ограничен именно
обрабатываемым candidate generation; вне этого вызова — уже активным
поколением. Произвольное generation задать нельзя. Grant ограничен
instance/replica, поколением и purpose, и
погашается ровно один раз; второе погашение отклоняется локально, без второго
вызова к Core. SDK не интерпретирует secret references и не пишет secret bytes,
handles, references или purpose ни в один ответ, лог, error или metric.
Погашенное значение отдаётся владельцу с явным `Destroy`.

## Transport и credentials

Control plane — только mutual TLS: HTTPS-only, обязательный client
certificate, pinned single peer identity, TLS floor из contract и отказ от
HTTP redirect. Анонимный клиент, bearer token вместо сертификата и plaintext
control URL отклоняются на handshake или на этапе конструирования. Bearer-only
и plaintext downgrade-путей нет.

Это ограничение относится к текущему production HTTP contract v1. Механика
транспортной защиты принадлежит SDK, а не продуктовым сервисам. Отдельный
loopback-only plaintext profile для локальной разработки реализован как
явный opt-in SDK API `infrastructure.NewLoopbackHealthServer`; по умолчанию
listener не создаётся. SDK принимает только TCP bind с literal loopback IP и
сам публикует на отдельном listener только generic `GET /healthz`.
Основной SDK REST handler также всегда публикует универсальные `GET /healthz`
и `GET /metrics`; маршруты, общий health response и exposition создаёт SDK из
зафиксированного HTTP contract и SDK collector. Все product services используют
этот `HandlerSet` и не реализуют, не заменяют и не дублируют endpoints. Публичный
data-plane listener при необходимости подключает тот же SDK
`NewObservabilityHandler` с тем же collector.
Произвольный handler передать нельзя. `/ready` остаётся под mTLS, поскольку его
ответ содержит identity replica и generation; остальные lifecycle, metadata,
metrics, admin и artifact endpoints также доступны только через mTLS.
Config pull и secret grants всегда требуют mTLS и недоступны через plaintext
listener/client. TLS failure не включает plaintext fallback. mTLS обязателен
для чувствительных операций и production Core↔service REST. Точные значения
profile публикуются в generated versioned artifact
[`loopback-plaintext-profile.json`](infrastructure/assets/service-sdk/v2/loopback-plaintext-profile.json).

Credentials принадлежат оператору: SDK не встраивает CA, не генерирует
сертификаты и не подставляет заглушку. `LoadCredentials` валидирует материал
(непустой trust pool, соответствие роли, private key против public key,
минимальный размер ключа); `NewStaticCredentialsProvider` обслуживает один
набор, `NewFileCredentialsProvider` перечитывает файлы при изменении размера
или mtime, поэтому ротация на диске применяется без рестарта процесса.
Неудачная ротация sticky: та же ошибка повторяется, пока вход не изменится, и
ранее валидный материал остаётся в силе. `NewRevocation` индексирует CRL,
подписанные доверенным authority, и уважает окно ThisUpdate/NextUpdate;
недоступный, непарсящий, неподписанный или истёкший набор даёт ошибку, а не
`false`, поэтому handshake fail-closed. `RevocationFailClosed` — zero value и
единственная posture, которую описывает contract; `RevocationPermitAbsent`
существует как явный более слабый opt-in флаг и не используется по умолчанию.
Revocation и peer identity вычисляются один раз на handshake — единственной
точке, где предъявляется peer certificate; окно на уже принятое соединение
ограничивает contract idle deadline.

## Слои и границы

Ровно четыре production слоя: `domain/`, `application/`,
`infrastructure/`, `presentation/`. `domain/` содержит только `models/` и
`interfaces/`. Зависимости направлены внутрь: domain не импортирует другие
слои, application и infrastructure импортируют domain, presentation —
domain и application. `presentation` не импортирует `infrastructure`, поэтому
composition живёт вне четырёх слоёв — в собственном `main` сервиса.
`tests/layers.test.ts` падает на нарушении любого из этих правил, на
появлении пятого слоя или product-specific package.

SDK не зависит от Core, `wool`, конкретного web server, любого product service
или стороннего модуля: production-код импортирует только четыре слоя и
standard library. `wool` остаётся независимой библиотекой только
для generic service↔service взаимодействия и этот модуль её не импортирует.

## Что делает автор сервиса

Автор реализует только service-owned порты и собирает их в своём `main`:

- `interfaces.ConfigurationApplier` — валидация и атомарное применение
  документа.
- `interfaces.ServiceMetadata` — `Manifest` и версионированная settings JSON
  Schema, публикуемые байт-в-байт.
- `interfaces.Logger`, `interfaces.LifecycleObserver`, `interfaces.Clock` и
  observability, если автор хочет собственную телеметрию.
- `application.NewSecretManager` там, где сервису нужны operational secrets.

SDK даёт готовые адаптеры: `infrastructure.LoadHTTPContract`,
`NewLifecycle`, `NewCoreConfigurationSource`, `NewServiceClient`,
`NewCoreSecretBroker`, `NewMutualTLSClient`, `NewMutualTLSServer`,
`NewJSONLogger`, `NewObserverPrometheusCollector`, `NewRecorder`,
`NewLoggingObserver`, `NewRevocation` и `presentation.NewHandlerSet`.
`NewHandlerSet` валидирует contract и требует значение на каждый порт до того,
как что-либо слушается, отказываясь строить неконформную поверхность;
`Handler()` отдаёт multiplexer и никогда сам не слушает — transport и lifecycle
процесса принадлежат composition root. Author не пишет generic HTTP/mTLS boilerplate и
не может случайно добавить endpoint, которого нет в versioned contract.

Рабочий пример composition — `tests/fixtures/reload-runtime/main.go`. Он
test-only: его plaintext mirror и harness control listener никогда не
попадают в production packages.

## Observability

`NewObserverPrometheusCollector` рендерит exposition в media type, который
регистрирует contract, с детерминированными именами, help-строками и
bounded labels; `NewRecorder` считает ровно один bounded lifecycle event на
outcome, держит readiness и pull-failure сигналы в шаге с применённым
поколением и никогда не принимает document, secret value или grant handle как
label value. `NewJSONLogger` пишет newline-delimited JSON в operator-owned
sink, ограничивая поля, длины ключа и значения и подставляя redaction
placeholder до сериализации; config body, cookies, `Authorization`, private
key, DSN, grant handle и transport secrets не пишутся. `application.OutcomeOf`
даёт типизированный outcome из ошибки, а `OutcomeError` не сериализует
внутреннюю причину клиенту: наружу уходят только outcome и
зарегистрированный contract-ом code.

## Canonical module path

Canonical import path, утверждённый владельцем: `github.com/Liapoldus/service-sdk/v2`.
Все потребители должны использовать этот путь и Service API из regenerated
contracts. Старые imports, HTTP paths и API aliases не поддерживаются.
Нейтральные ID, application methods и исходные immutable generation bytes/digests
не меняются. SDK-only gate не заменяет проверки Core и service-потребителей;
не выпускать модуль до прохождения общих integration gates.

## Release integrity

Каждый `service-sdk-v2.*` release публикуется только release workflow после
`go test ./...`, `go vet ./...` и проверки module path. Workflow создаёт
детерминированный source artifact, SPDX SBOM, keyless Sigstore bundle для
artifact и SBOM и GitHub build-provenance attestation. Потребитель проверяет
checksum и bundle до добавления SDK в lockfile; наличие одного только Git tag
не является доказательством происхождения.

Нормативная архитектура и migration plan находятся в
[документации Core](/core/architecture/target).

## Проверка

`make check` запускает TypeScript conformance, `go build ./...` и
`go vet ./...`. TypeScript conformance поднимает реальные mTLS child processes,
поэтому для него нужны Go toolchain и Node. На 2026-10-04 текущий локальный
набор включает 14 файлов и 190 тестов; тот же suite прошёл в Ubuntu 24.04 ARM64
VM под OrbStack. Сквозная интеграция дополнительно проверяется в Core и
продуктовых services; локальный PASS не заменяет hosted CI на согласованных
опубликованных revisions и release provenance.
