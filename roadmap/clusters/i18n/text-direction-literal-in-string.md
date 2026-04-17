# TextDirectionLiteralInString

**Cluster:** [i18n](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`string.startsWith("\u200E")` / contains BIDI control characters
outside a dedicated rtl-handling helper.

## Triggers

```kotlin
val fixed = "\u200E" + userName
```

## Does not trigger

```kotlin
// Inside a dedicated bidi helper
fun wrapLtr(s: String) = BidiFormatter.getInstance().unicodeWrap(s)
```

## Dispatch

String-literal scan for BIDI control chars (`\u200E`, `\u200F`,
`\u202A`..`\u202E`, `\u2066`..`\u2069`).

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
