# AnalyticsCallWithoutConsentGate

**Cluster:** [privacy](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

Analytics event call that is not inside an
`if (consentState.analyticsAllowed) { ... }` or similar guard.

## Triggers

```kotlin
firebaseAnalytics.logEvent("screen_view", Bundle.EMPTY)
```

## Does not trigger

```kotlin
if (consent.analyticsAllowed) {
    firebaseAnalytics.logEvent("screen_view", Bundle.EMPTY)
}
```

## Dispatch

Analytics-method call outside an enclosing function that contains
any `consent`/`gdpr`/`privacy`/`tracking` token — heuristic.

## Links

- Parent: [`roadmap/60-privacy-data-handling-rules.md`](../../60-privacy-data-handling-rules.md)
