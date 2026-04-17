# StringConcatForTranslation

**Cluster:** [i18n](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`stringResource(R.string.greeting) + " " + name` — concatenation of a
translatable resource with any non-literal. Forces English word order.

## Triggers

```kotlin
Text(stringResource(R.string.greeting) + " " + name)
```

## Does not trigger

```kotlin
// In strings.xml: <string name="greeting_with_name">Hello %1$s!</string>
Text(stringResource(R.string.greeting_with_name, name))
```

## Dispatch

Walk `+` binary expressions where one operand is a
`stringResource(...)` call and the other isn't a literal.

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
- Related: [`string-template-for-translation.md`](string-template-for-translation.md)
