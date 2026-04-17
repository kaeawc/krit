# TempFileWorldReadable

**Cluster:** [security/call-shape](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`createTempFile(...)` followed by `setReadable(true, false)` or
`setWritable(true, false)` in the same block.

## Triggers

```kotlin
val t = File.createTempFile("secret", ".txt")
t.setReadable(true, false) // readable by all, not just owner
```

## Does not trigger

```kotlin
val t = File.createTempFile("secret", ".txt")
t.setReadable(true, true)
```

## Dispatch

Walk the siblings of a `createTempFile` assignment looking for
`setReadable/setWritable(true, false)` calls on the same variable.

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
