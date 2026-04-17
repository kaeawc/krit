# FirebaseRemoteConfigDefaultsWithPii

**Cluster:** [privacy](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

Remote Config `setDefaults(...)` map containing keys matching PII
patterns.

## Triggers

```kotlin
Firebase.remoteConfig.setDefaultsAsync(mapOf(
    "user_email_template" to "%s@example.com",
))
```

## Does not trigger

```kotlin
Firebase.remoteConfig.setDefaultsAsync(mapOf(
    "welcome_message" to "Hello",
))
```

## Dispatch

`call_expression` on `setDefaults`/`setDefaultsAsync` inspecting
map keys.

## Links

- Parent: [`roadmap/60-privacy-data-handling-rules.md`](../../60-privacy-data-handling-rules.md)
