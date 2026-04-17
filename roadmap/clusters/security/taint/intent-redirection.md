# IntentRedirection

**Cluster:** [security/taint](README.md) · **Status:** deferred

## Catches

Untrusted extra containing an inner `Intent` reaching `startActivity(...)`.

## Shape

```kotlin
val inner = intent.getParcelableExtra<Intent>("next")
startActivity(inner)
```

## Links

- Parent: [`roadmap/51-security-rules-taint.md`](../../../51-security-rules-taint.md)
- Related: [`../syntactic/start-activity-with-untrusted-intent.md`](../syntactic/start-activity-with-untrusted-intent.md)
