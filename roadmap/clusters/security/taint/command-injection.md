# CommandInjection

**Cluster:** [security/taint](README.md) · **Status:** deferred

## Catches

Untrusted source reaching `Runtime.exec(...)`, `ProcessBuilder(...)`,
or `Shell.Command(...)`.

## Shape

```kotlin
val path = intent.getStringExtra("path")
Runtime.getRuntime().exec("ls $path")
```

## Why deferred

Tier-2 analog [`../call-shape/runtime-exec-unsafe-shape.md`](../call-shape/runtime-exec-unsafe-shape.md)
ships now. Taint version requires source→sink tracking.

## Links

- Parent: [`roadmap/51-security-rules-taint.md`](../../../51-security-rules-taint.md)
- Tier-2 analog: [`../call-shape/runtime-exec-unsafe-shape.md`](../call-shape/runtime-exec-unsafe-shape.md)
