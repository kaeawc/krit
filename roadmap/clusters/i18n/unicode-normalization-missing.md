# UnicodeNormalizationMissing

**Cluster:** [i18n](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`string.contains(userInput)` inside a function named `search*` /
`find*` — unicode-equivalent characters will not match.

## Triggers

```kotlin
fun searchUsers(query: String): List<User> =
    users.filter { it.name.contains(query, ignoreCase = true) }
```

## Does not trigger

```kotlin
fun searchUsers(query: String): List<User> {
    val normalized = Normalizer.normalize(query, Normalizer.Form.NFC)
    return users.filter {
        Normalizer.normalize(it.name, Normalizer.Form.NFC)
            .contains(normalized, ignoreCase = true)
    }
}
```

## Dispatch

`call_expression` on `.contains(...)` inside a function with a
name-pattern gate.

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
