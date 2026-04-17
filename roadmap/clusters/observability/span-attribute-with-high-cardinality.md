# SpanAttributeWithHighCardinality

**Cluster:** [observability](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`span.setAttribute("user_id", userId)` — high-cardinality attribute
values explode trace indexes.

## Triggers

```kotlin
span.setAttribute("user_id", user.id.toString())
```

## Does not trigger

```kotlin
span.setAttribute("user_tier", user.tier.name)
```

## Dispatch

`call_expression` on `setAttribute` / `setAttributes` with a key
in the high-cardinality list (`user_id`, `session_id`, `trace_id`).

## Links

- Parent: [`roadmap/61-observability-rules.md`](../../61-observability-rules.md)
