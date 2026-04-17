# AnalyticsEventWithPiiParamName

**Cluster:** [privacy](README.md) · **Status:** shipped · **Severity:** warning · **Default:** inactive

## Catches

Call to an analytics `event_method` (from the SDK registry) whose
argument bundle includes a key matching
`/email|phone|ssn|dob|address|lat|lng|location/i`.

## Triggers

```kotlin
firebaseAnalytics.logEvent("signup", bundleOf(
    "user_email" to email,
    "plan" to "free",
))
```

## Does not trigger

```kotlin
firebaseAnalytics.logEvent("signup", bundleOf(
    "plan" to "free",
))
```

## Dispatch

`call_expression` matching a registered analytics event method.
Inspect argument map / bundle keys for the PII regex.

## Links

- Parent: [`roadmap/60-privacy-data-handling-rules.md`](../../60-privacy-data-handling-rules.md)
- Related: `roadmap/clusters/security/call-shape/log-pii.md`
