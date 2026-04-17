# StartActivityWithUntrustedIntent

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`context.startActivity(intent)` where `intent` was built via
`Intent.parseUri(...)` earlier in the same function. Intent parsing
from untrusted strings enables intent-redirection attacks.

## Example — triggers

```kotlin
fun launch(uri: String) {
    val intent = Intent.parseUri(uri, 0)
    startActivity(intent)
}
```

## Example — does not trigger

```kotlin
fun launch(uri: String) {
    val inner = Intent.parseUri(uri, 0)
    inner.setPackage(packageName)            // explicit target
    inner.component = ComponentName(...)     // explicit component
    startActivity(inner)
}
```

## Implementation notes

- Dispatch: `call_expression` on `startActivity`.
- Walk back across sibling statements in the same function body for
  `Intent.parseUri(...)` assigned to the argument name; skip if a
  `setPackage` / `component = ...` guard precedes the launch.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`implicit-pending-intent.md`](implicit-pending-intent.md),
  `roadmap/clusters/security/taint/intent-redirection.md`
