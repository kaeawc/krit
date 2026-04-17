# Security — syntactic misuse (tier 1)

Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)

Ship-now rules. Each positive case is a single `call_expression`,
`class_declaration`, or property assignment. AST dispatch only; no
data flow, no taint.

## Crypto

- [`weak-message-digest.md`](weak-message-digest.md)
- [`weak-mac-algorithm.md`](weak-mac-algorithm.md)
- [`static-iv.md`](static-iv.md)
- [`hardcoded-secret-key.md`](hardcoded-secret-key.md)
- [`rsa-no-padding.md`](rsa-no-padding.md)
- [`weak-key-size.md`](weak-key-size.md)
- [`allow-all-hostname-verifier.md`](allow-all-hostname-verifier.md)
- [`insecure-trust-manager.md`](insecure-trust-manager.md)
- [`prng-from-system-time.md`](prng-from-system-time.md)

## WebView

- [`webview-allow-file-access.md`](webview-allow-file-access.md)
- [`webview-allow-content-access.md`](webview-allow-content-access.md)
- [`webview-universal-access-from-file-urls.md`](webview-universal-access-from-file-urls.md)
- [`webview-file-access-from-file-urls.md`](webview-file-access-from-file-urls.md)
- [`webview-mixed-content-allow-all.md`](webview-mixed-content-allow-all.md)
- [`webview-debugging-enabled.md`](webview-debugging-enabled.md)

## Network / TLS

- [`hardcoded-http-url.md`](hardcoded-http-url.md)
- [`disable-certificate-pinning.md`](disable-certificate-pinning.md)
- [`okhttp-disable-ssl-validation.md`](okhttp-disable-ssl-validation.md)
- [`network-security-config-debug-overrides.md`](network-security-config-debug-overrides.md)

## Deserialization / reflection

- [`java-object-input-stream.md`](java-object-input-stream.md)
- [`gson-polymorphic-from-json.md`](gson-polymorphic-from-json.md)
- [`jackson-default-typing.md`](jackson-default-typing.md)
- [`xml-external-entity.md`](xml-external-entity.md)

## Android intent / manifest

- [`implicit-pending-intent.md`](implicit-pending-intent.md)
- [`start-activity-with-untrusted-intent.md`](start-activity-with-untrusted-intent.md)
- [`broadcast-receiver-exported-flag-missing.md`](broadcast-receiver-exported-flag-missing.md)
- [`deep-link-missing-auto-verify.md`](deep-link-missing-auto-verify.md)
