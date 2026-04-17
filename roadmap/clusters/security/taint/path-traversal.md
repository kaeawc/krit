# PathTraversal

**Cluster:** [security/taint](README.md) · **Status:** deferred

## Catches

Untrusted source reaching `File(parent, child)`, `FileInputStream(...)`,
`openFileOutput(...)`, `File(...)`.

## Shape

```kotlin
val name = request.queryParameter("file")
FileInputStream(File(downloads, name)).use { ... }
```

## Why deferred

Tier-2 analog [`../call-shape/file-from-untrusted-path.md`](../call-shape/file-from-untrusted-path.md)
ships now with a function-name heuristic. Taint version generalises
beyond named-function gating.

## Links

- Parent: [`roadmap/51-security-rules-taint.md`](../../../51-security-rules-taint.md)
- Tier-2 analog: [`../call-shape/file-from-untrusted-path.md`](../call-shape/file-from-untrusted-path.md)
