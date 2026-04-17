# Observability cluster

Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)

Uses the same registry infra as
[`../privacy/`](../privacy/), instanced for observability libraries.

## Logging hygiene

- [`logger-interpolated-message.md`](logger-interpolated-message.md)
- [`logger-string-concat.md`](logger-string-concat.md)
- [`logger-without-logger-field.md`](logger-without-logger-field.md)
- [`log-level-guard-missing.md`](log-level-guard-missing.md)
- [`unstructured-error-log.md`](unstructured-error-log.md)
- [`log-without-correlation-id.md`](log-without-correlation-id.md)

## Tracing / context

- [`span-start-without-finish.md`](span-start-without-finish.md)
- [`mdc-across-coroutine-boundary.md`](mdc-across-coroutine-boundary.md)
- [`with-context-without-tracing-context.md`](with-context-without-tracing-context.md)
- [`span-attribute-with-high-cardinality.md`](span-attribute-with-high-cardinality.md)
- [`trace-id-logged-as-plain-message.md`](trace-id-logged-as-plain-message.md)

## Metrics naming

- [`metric-name-missing-unit.md`](metric-name-missing-unit.md)
- [`metric-counter-not-monotonic.md`](metric-counter-not-monotonic.md)
- [`metric-timer-outside-block.md`](metric-timer-outside-block.md)
- [`metric-tag-high-cardinality.md`](metric-tag-high-cardinality.md)

## Structured fields

- [`mdc-put-no-remove.md`](mdc-put-no-remove.md)
- [`structured-log-key-mixed-case.md`](structured-log-key-mixed-case.md)
- [`nullable-structured-field.md`](nullable-structured-field.md)
