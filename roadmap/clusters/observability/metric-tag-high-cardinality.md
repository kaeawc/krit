# MetricTagHighCardinality

**Cluster:** [observability](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

Tag keyed on `user_id` / `session_id` / `trace_id` — high-cardinality
label explodes metric storage.

## Triggers

```kotlin
registry.counter("events", "user_id", user.id).increment()
```

## Does not trigger

```kotlin
registry.counter("events", "tier", user.tier).increment()
```

## Dispatch

`call_expression` on metric constructors with high-cardinality tag
key literals.

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
- Related: [`span-attribute-with-high-cardinality.md`](span-attribute-with-high-cardinality.md)
