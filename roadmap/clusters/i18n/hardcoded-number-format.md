# HardcodedNumberFormat

**Cluster:** [i18n](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** active

## Catches

`DecimalFormat("pattern")` / `NumberFormat.getInstance()` without a
`Locale` argument.

## Triggers

```kotlin
val fmt = DecimalFormat("#,###.##")
```

## Does not trigger

```kotlin
val fmt = DecimalFormat("#,###.##", DecimalFormatSymbols(Locale.ROOT))
val n = NumberFormat.getInstance(Locale.US)
```

## Dispatch

`call_expression` on `DecimalFormat`/`NumberFormat.getInstance` with
no `Locale` argument.

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
