# LocaleGetDefaultForFormatting

**Cluster:** [i18n](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** active

## Catches

`Locale.getDefault()` passed to a formatter used for persistence or
network IO — should be `Locale.ROOT` or `Locale.US`.

## Triggers

```kotlin
fun isoTimestamp(instant: Instant): String =
    DateTimeFormatter.ISO_INSTANT
        .withLocale(Locale.getDefault())
        .format(instant)
```

## Does not trigger

```kotlin
fun isoTimestamp(instant: Instant): String =
    DateTimeFormatter.ISO_INSTANT
        .withLocale(Locale.ROOT)
        .format(instant)
```

## Dispatch

`call_expression` argument inspection; gated on the enclosing call
being a persistence/network formatter (shared
`isPersistenceOrNetworkContext` helper — see parent doc).

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
