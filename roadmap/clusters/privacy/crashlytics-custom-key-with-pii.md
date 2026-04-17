# CrashlyticsCustomKeyWithPii

**Cluster:** [privacy](README.md) · **Status:** shipped · **Severity:** warning · **Default:** inactive

## Catches

`FirebaseCrashlytics.setCustomKey("email", ...)` — crash reports
shouldn't carry PII.

## Triggers

```kotlin
FirebaseCrashlytics.getInstance().setCustomKey("email", user.email)
```

## Does not trigger

```kotlin
FirebaseCrashlytics.getInstance().setCustomKey("tier", "premium")
```

## Dispatch

`call_expression` on `setCustomKey` from the Crashlytics registry
entry.

## Links

- Parent: [`roadmap/60-privacy-data-handling-rules.md`](../../60-privacy-data-handling-rules.md)
