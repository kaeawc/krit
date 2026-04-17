# SuppressedWarningWithoutJustification

**Cluster:** [licensing](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`@Suppress("UNCHECKED_CAST")` / any suppression with no KDoc
immediately above explaining why. Stricter variant of
`ForbiddenSuppress`.

## Triggers

```kotlin
@Suppress("UNCHECKED_CAST")
val m = map as Map<String, Int>
```

## Does not trigger

```kotlin
/** Safe: the map is produced by a factory that always returns String→Int. */
@Suppress("UNCHECKED_CAST")
val m = map as Map<String, Int>
```

## Dispatch

`annotation` on `@Suppress` with no preceding KDoc comment.

## Links

- Parent: [`roadmap/64-licensing-legal-rules.md`](../../64-licensing-legal-rules.md)
