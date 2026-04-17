# OpenRedirect

**Cluster:** [security/taint](README.md) · **Status:** deferred

## Catches

Untrusted source (Intent extra, URI query parameter) reaching
`WebView.loadUrl(...)`, `startActivity(Intent(ACTION_VIEW, ...))`,
or an HTTP request builder's URL setter.

## Shape

```kotlin
val redirect = intent.getStringExtra("redirect")
webView.loadUrl(redirect)
```

## Links

- Parent: [`roadmap/51-security-rules-taint.md`](../../../51-security-rules-taint.md)
