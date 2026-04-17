# WebViewMixedContentAllowAll

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`settings.mixedContentMode = MIXED_CONTENT_ALWAYS_ALLOW` or
`.setMixedContentMode(MIXED_CONTENT_ALWAYS_ALLOW)`.

## Example — triggers

```kotlin
webView.settings.mixedContentMode = WebSettings.MIXED_CONTENT_ALWAYS_ALLOW
```

## Example — does not trigger

```kotlin
webView.settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
```

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`webview-allow-file-access.md`](webview-allow-file-access.md)
