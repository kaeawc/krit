# WebViewFileAccessFromFileUrls

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`settings.allowFileAccessFromFileURLs = true` — pages from `file://`
can read other `file://` documents.

## Example — triggers

```kotlin
webView.settings.allowFileAccessFromFileURLs = true
```

## Example — does not trigger

```kotlin
webView.settings.allowFileAccessFromFileURLs = false
```

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`webview-universal-access-from-file-urls.md`](webview-universal-access-from-file-urls.md)
