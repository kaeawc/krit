# LocaleDefaultForCurrency

**Cluster:** [i18n](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`Currency.getInstance(Locale.getDefault())` inside a class whose
name contains `Price`, `Money`, or `Amount`. Currency should come
from order/transaction data, not the user's locale.

## Triggers

```kotlin
class PriceFormatter {
    private val currency = Currency.getInstance(Locale.getDefault())
}
```

## Does not trigger

```kotlin
class PriceFormatter(private val currencyCode: String) {
    private val currency = Currency.getInstance(currencyCode)
}
```

## Dispatch

`call_expression` inside a class declaration with a name-pattern gate.

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
