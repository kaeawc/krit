# AnalyticsUserIdFromPii

**Cluster:** [privacy](README.md) · **Status:** shipped · **Severity:** warning · **Default:** inactive

## Catches

Call to a `user_id_methods` registry entry whose argument is a
property named `email` / `phoneNumber` / `username`. User IDs
should be opaque.

## Triggers

```kotlin
firebaseAnalytics.setUserId(user.email)
```

## Does not trigger

```kotlin
firebaseAnalytics.setUserId(user.anonymousId)
```

## Dispatch

`call_expression` on the user-id setter; inspect argument identifier.

## Links

- Parent: [`roadmap/60-privacy-data-handling-rules.md`](../../60-privacy-data-handling-rules.md)
