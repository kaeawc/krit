# ContentProviderQueryWithSelectionInterpolation

**Cluster:** [security/call-shape](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`ContentResolver.query(uri, projection, selection, ...)` where
`selection` is an interpolated string — selectionArgs is the safe
path.

## Triggers

```kotlin
resolver.query(uri, null, "name = '$name'", null, null)
```

## Does not trigger

```kotlin
resolver.query(uri, null, "name = ?", arrayOf(name), null)
```

## Dispatch

`call_expression` on `ContentResolver.query` with the shared shape
helper applied to the third argument.

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
- Related: [`sql-injection-raw-query.md`](sql-injection-raw-query.md)
