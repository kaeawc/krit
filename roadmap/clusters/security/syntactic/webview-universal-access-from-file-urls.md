# WebViewUniversalAccessFromFileUrls

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`settings.allowUniversalAccessFromFileURLs = true` — lets pages loaded
from `file://` make cross-origin XHR to any domain.

## Example — triggers

```kotlin
webView.settings.allowUniversalAccessFromFileURLs = true
```

## Example — does not trigger

```kotlin
webView.settings.allowUniversalAccessFromFileURLs = false
```

## Implementation notes

- Same receiver helper as the other WebView rules.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`webview-file-access-from-file-urls.md`](webview-file-access-from-file-urls.md)
