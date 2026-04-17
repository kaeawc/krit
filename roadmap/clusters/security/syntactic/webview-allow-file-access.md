# WebViewAllowFileAccess

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`settings.allowFileAccess = true` or `.setAllowFileAccess(true)` on a
`WebSettings` receiver. WebView file URL access is disabled by default
since API 30; explicit enablement is almost always a mistake.

## Example — triggers

```kotlin
webView.settings.allowFileAccess = true
```

## Example — does not trigger

```kotlin
webView.settings.allowFileAccess = false
```

## Implementation notes

- Dispatch: assignment or `setAllowFileAccess` call where the receiver
  resolves (via typeinfer) or textually suffixes with `WebSettings`.
- Shares `isWebSettingsReceiver(node)` helper with the other WebView
  rules in this sub-cluster.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`webview-allow-content-access.md`](webview-allow-content-access.md),
  [`webview-universal-access-from-file-urls.md`](webview-universal-access-from-file-urls.md)
