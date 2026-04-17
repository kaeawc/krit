# DebugToastInProduction

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`Toast.makeText(..., "DEBUG: ...", ...)` or `.show()` with a message
starting with `"debug"` / `"test"` / `"wip"` (case-insensitive).

## Triggers

```kotlin
Toast.makeText(context, "DEBUG: user clicked", Toast.LENGTH_SHORT).show()
```

## Does not trigger

```kotlin
Toast.makeText(context, stringResource(R.string.saved), Toast.LENGTH_SHORT).show()
```

## Dispatch

`call_expression` on `Toast.makeText` whose message literal matches
the debug prefix pattern.

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
