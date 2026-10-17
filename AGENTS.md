# Service SDK agent instructions

This directory is a standalone Go module for generic Core↔service REST lifecycle.
Its canonical module path is `github.com/Liapoldus/service-sdk/v2` under the
approved breaking nomenclature migration. Consumers must migrate to the same
Service API and regenerated contracts; SDK-only checks do not establish consumer
integration. Its
remote is `https://github.com/Liapoldus/service-sdk.git`. Do not publish a
release until every active consumer and integration gate passes.

This repository owns the Service SDK documentation and diagrams under `docs/`
plus the package README. The unified VitePress site imports a pinned source
revision; edit the owner source, never a generated copy in the site aggregator.
The README is canonical at `/service-sdk`; shared service guides live under
`docs/site/services/` at `/services`. Runtime entities are Services; only editor
extensions are StudioPlugins. Control plane: Studio → CLI → Core → Service SDK
REST. Data plane: independent services → Wool (`github.com/Liapoldus/wool/v3`).
Do not add old imports, routes, commands or API aliases. Preserve opaque IDs,
application methods and exact immutable generation bytes/digests. `TODO.md`
retains dated historical reports with their original terminology.

## Architecture

- Keep exactly four production layers: `domain/`, `application/`,
  `infrastructure/`, and `presentation/`.
- `domain/` contains only `models/` and `interfaces/`. Keep business policy in
  `application/`; network, TLS, HTTP, and code-owned contract constants belong
  in `infrastructure/`; HTTP handlers belong in `presentation/`.
- Dependencies point inward: domain imports no other layer; application imports
  domain; infrastructure imports domain; presentation imports domain and
  application. Do not add a fifth layer or product-specific packages.
- `presentation/` must not import `infrastructure/`. Composition — contract
  loading, SDK client/server setup, listeners, process lifecycle and shutdown —
  belongs in the service's own `main` or another package outside these four
  layers; `tests/fixtures/reload-runtime/main.go` is the worked example. The
  service passes security configuration/credentials to SDK APIs; the SDK owns
  TLS/mTLS listeners, clients, certificate verification and handshake. A
  product service must not construct its own TLS stack for Core↔service REST.
- The SDK owns generic Core↔service lifecycle only. It must not depend on Core,
  `wool`, a specific web server, or any product service. `wool` owns generic
  service↔service communication only.
- Every production product service composes its Core-facing REST surface with
  `presentation.NewHandlerSet`. The SDK contract fixes the universal `GET
  /healthz` and `GET /metrics` routes and response formats; service code must not
  define, replace, rename or duplicate either handler. A product with a public
  data-plane listener mounts `presentation.NewObservabilityHandler` from the
  same SDK contract and collector instead of implementing another copy.
- Product configuration remains an opaque JSON object to the SDK. Preserve the
  exact bytes returned by Core, verify generation/schema-version/digest, reject
  duplicate JSON object keys, and call the service-owned validator/applier before
  acknowledging `Reload`. Never interpret product fields.
- `Rollback` is a Core Management API operation; do not add a service-side
  rollback endpoint. Services pull exact generations only after Core invokes
  `Reload`.
- Versioned HTTP v2 and lifecycle/poll/loopback constants are owned by typed
  constructors in `infrastructure/contract_definitions.go`. Loaders return fresh
  maps and slices, never parse static JSON. Change definitions first, then run
  `make contracts`; `make contracts-check` checks deterministic public JSON
  artifacts under `infrastructure/assets/service-sdk/`. Do not edit those four
  generated documents as a second source of truth. The two v2 JSON schemas are
  code-owned declarations in `infrastructure/schemas.go`; runtime exports and
  public artifacts are generated from them without runtime embed/file loaders.
- Require per-replica mTLS on the production Core↔service REST contract. The SDK
  owns all TLS/mTLS and plaintext transport mechanics; services select a
  supported SDK security profile and never implement transport security
  themselves. The versioned, opt-in loopback plaintext development profile is
  separate from the production v1 HTTP contract, disabled by default, and
  exposes only the generic `GET /healthz` response on its own
  literal-loopback TCP listener. It accepts no caller-provided handler; every
  other route, including `/ready`, is unavailable there. Exact config pull and
  secret-grant endpoints/clients always require mTLS and must never be reachable
  through a plaintext listener. Plaintext must not bind remotely or activate as
  fallback after a TLS failure. Do not add plaintext or bearer-only downgrade
  to the production contract. Do not place application settings or secrets in
  environment variables, process arguments, logs, errors, metrics, or ACKs.
- In v2 the SDK owns generic replica self-registration/lease, versioned
  peer-directory and endpoint resolution, but never service process/container
  lifecycle or peer Call/Stream execution. Core and protocol are not SDK
  dependencies. Installation and planned upgrades belong to the operator,
  outside this module. Release digests are authenticated replica claims
  for compatibility checks, not proof of artifact provenance.

## Studio configuration metadata

The approved workspace configuration plan is [`../studio/plan.md`](../studio/plan.md).
The SDK owns a bounded, versioned, declarative configuration-UI descriptor
alongside the service-owned JSON Schema and manifest. The SDK validates and
canonicalizes descriptor metadata, but never renders UI, interprets product
settings, executes widget code, or depends on Studio/Core/CLI/Wool. JSON Schema
remains the validation authority; UI hints cannot loosen its constraints.
Metadata flows through Service SDK REST to Core and then through typed CLI to
Studio. A resource may reference a service-owned artifact action only through
typed `pageId`/`actionId` metadata; consumers must verify the live Core-backed
Admin Surface still declares an artifact action before invoking it. This pointer
is neither authorization nor an endpoint. Project-backed resources cannot
declare a service-admin publication action. Do not add a direct Studio or CLI
Service SDK client. Add descriptor conformance and canonicalization tests before
changing the public contract.

## Tests and checks

- Keep TypeScript conformance/child-process tests under `tests/`; native Go
  unit tests are approved beside typed contracts and production code. Tests are
  not a fifth production layer.
- Add/update a failing native Go or TypeScript test before implementing behavior,
  then keep the completed slice green. Contract tests must cover published JSON
  parity, caller ownership of mutable values, validation, and schema exports.
- Run `make check` before handing off a completed SDK slice.
- The child-process fixture is test-only. Its plaintext listener and any
  insecure TLS options must never enter production packages.
- When an owner has approved a target contract, remove superseded lifecycle
  behavior as part of the coordinated replacement. Do not ship permanent
  compatibility aliases, old-endpoint fallbacks, or two concurrent lifecycle
  models. Keep consumer migration, tests, and the new SDK contract aligned in
  the final integrated change; never hand off a state that still depends on the
  retired protocol lifecycle.
