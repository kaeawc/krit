# LogInjection

**Cluster:** [security/taint](README.md) · **Status:** deferred

## Catches

Untrusted source reaching a logger format string containing `\n` or
other control characters.

## Shape

```kotlin
val user = request.header("X-User")
logger.info("user=$user logged in") // newline in $user forges a log line
```

## Links

- Parent: [`roadmap/51-security-rules-taint.md`](../../../51-security-rules-taint.md)
- Related: [`../call-shape/log-pii.md`](../call-shape/log-pii.md)
