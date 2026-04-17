# BufferedReadWithoutBuffer

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`FileInputStream(...).read(ByteArray(N))` where `N < 8192` — should
be wrapped in `BufferedInputStream`.

## Triggers

```kotlin
val bytes = ByteArray(512)
FileInputStream(path).read(bytes)
```

## Does not trigger

```kotlin
FileInputStream(path).buffered().use { it.readBytes() }
```

## Dispatch

`call_expression` on `.read(...)` with a small buffer size.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
