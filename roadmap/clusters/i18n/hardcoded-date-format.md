# HardcodedDateFormat

**Cluster:** [i18n](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`SimpleDateFormat("pattern")` / `DateTimeFormatter.ofPattern("pattern")`
without a `Locale` argument.

## Triggers

```kotlin
val fmt = SimpleDateFormat("yyyy-MM-dd")
```

## Does not trigger

```kotlin
val fmt = SimpleDateFormat("yyyy-MM-dd", Locale.ROOT)
val fmt2 = DateTimeFormatter.ofPattern("yyyy-MM-dd", Locale.US)
```

## Dispatch

`call_expression` on `SimpleDateFormat(...)` / `DateTimeFormatter.ofPattern(...)`
with a single string argument.

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
- Related: [`locale-get-default-for-formatting.md`](locale-get-default-for-formatting.md)
