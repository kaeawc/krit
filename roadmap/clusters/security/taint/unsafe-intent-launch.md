# UnsafeIntentLaunch

**Cluster:** [security/taint](README.md) · **Status:** deferred

## Catches

`Intent.parseUri(...)` from a SharedPreferences / deep-link / extra
source reaching `startActivity(...)`.

## Shape

```kotlin
val raw = prefs.getString("last_deep_link", null) ?: return
startActivity(Intent.parseUri(raw, 0))
```

## Links

- Parent: [`roadmap/51-security-rules-taint.md`](../../../51-security-rules-taint.md)
