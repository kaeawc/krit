# PluralsBuiltWithIfElse

**Cluster:** [i18n](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** active

## Catches

`if (count == 1) getString(R.string.one) else getString(R.string.many)`
— use `getQuantityString` / `pluralStringResource`.

## Triggers

```kotlin
val text = if (count == 1) "1 item" else "$count items"
```

## Does not trigger

```kotlin
val text = resources.getQuantityString(R.plurals.item_count, count, count)
```

## Dispatch

`if_expression` whose condition compares an Int to `1` and whose
branches both produce strings.

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
- Related: [`plurals-missing-zero.md`](plurals-missing-zero.md)
