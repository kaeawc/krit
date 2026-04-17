# WebViewAllowContentAccess

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`settings.allowContentAccess = true` or `.setAllowContentAccess(true)`
on a `WebSettings`. Allows content:// provider access from the WebView.

## Example — triggers

```kotlin
webView.settings.allowContentAccess = true
```

## Example — does not trigger

```kotlin
// default value is true but explicit disable is also clean
webView.settings.allowContentAccess = false
```

## Implementation notes

- Same dispatch + receiver helper as
  [`webview-allow-file-access.md`](webview-allow-file-access.md).

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`webview-allow-file-access.md`](webview-allow-file-access.md)
