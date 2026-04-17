# StringTemplateForTranslation

**Cluster:** [i18n](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`"${stringResource(R.string.label)}: $value"` — same problem as
`string-concat-for-translation`, interpolation flavour.

## Triggers

```kotlin
Text("${stringResource(R.string.label)}: $value")
```

## Does not trigger

```kotlin
Text(stringResource(R.string.label_fmt, value))
```

## Dispatch

String-template node containing a `stringResource(...)` interpolation.

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
- Related: [`string-concat-for-translation.md`](string-concat-for-translation.md)
