# WebViewDebuggingEnabled

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`WebView.setWebContentsDebuggingEnabled(true)` that is not guarded by
`BuildConfig.DEBUG` / `ApplicationInfo.FLAG_DEBUGGABLE` in the same
enclosing function.

## Example — triggers

```kotlin
class AppApplication : Application() {
    override fun onCreate() {
        super.onCreate()
        WebView.setWebContentsDebuggingEnabled(true)
    }
}
```

## Example — does not trigger

```kotlin
if (BuildConfig.DEBUG) {
    WebView.setWebContentsDebuggingEnabled(true)
}
```

## Implementation notes

- Dispatch: `call_expression` on `setWebContentsDebuggingEnabled(true)`.
- Walk ancestor `if_expression` and `when_expression` nodes for a
  `BuildConfig.DEBUG` condition; reuses the existing
  `BuildConfigDebugInverted` style helpers.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: `roadmap/clusters/release-engineering/build-config-debug-inverted.md`
