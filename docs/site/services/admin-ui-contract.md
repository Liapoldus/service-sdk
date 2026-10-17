# Service Admin UI contract

Сервис публикует versioned declarative Admin Surface; клиент управления не
вшивает product UI, а Core не принимает service-owned HTTP handler. Полная жизненная
модель, fixed internal API, cache, security и ownership находятся на
[Service Admin Pages](/services/admin-pages).

Поддерживаемые базовые fields: `string`, `number`, `boolean`, `select`,
`multiselect`, `secret`, `file`, `directory`, `duration`, `size`, `code`,
`keyValue`, `array`, `object`. `secret` — reference/write-only field, его
значение никогда не возвращается UI.

Общие technical endpoint/JSON shapes для declarative Admin Surface принадлежат
отдельному Service SDK contract; конкретное содержимое page/actions остаётся
service-owned. Не размещать Admin Surface в `wool`, который отвечает
только за generic service-to-service communication.
Для action с бинарным artifact Core передаёт metadata и один ограниченный
поток через отдельный mTLS Service SDK REST endpoint. SDK владеет framing,
ограничением входящих байтов, cancellation/backpressure и receipt envelope;
сервис владеет schema metadata, форматом архива, проверкой содержимого и
durable operation. Core не импортирует peer-only `wool` и не
буферизует весь artifact. SDK endpoint реализован и проверен на Core→SDK→service
child-process пути: передача artifact, receipt и status action долговечной
operation, доступность результата и повторный запуск service.
Ограничения и формат endpoint принадлежат versioned SDK contract. Это
расширение Core API описано в
[API boundaries](/architecture/api-boundaries); endpoint не является
неофициальным direct-service URL.
