# UpperLowerInvariantMisuse

**Cluster:** [i18n](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`.uppercase()` / `.lowercase()` without an argument on a user-facing
string (display name, email domain). Use `.uppercase(Locale.ROOT)`
for case-insensitive comparison, or avoid for display.

## Triggers

```kotlin
val normalized = userName.uppercase()
```

## Does not trigger

```kotlin
val normalized = userName.uppercase(Locale.ROOT)
```

## Dispatch

`call_expression` on `uppercase`/`lowercase` with 0 args. The
existing `ImplicitDefaultLocale` rule catches the older `.toLowerCase()`
/ `.toUpperCase()` — this is the 1.5+ equivalent.

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
- Related: existing `ImplicitDefaultLocale`
