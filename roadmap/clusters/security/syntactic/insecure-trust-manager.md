# InsecureTrustManager

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

Class implementing `X509TrustManager` whose `checkServerTrusted` /
`checkClientTrusted` body is empty or a single bare `return`.

## Example — triggers

```kotlin
class TrustAll : X509TrustManager {
    override fun checkClientTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    override fun checkServerTrusted(chain: Array<X509Certificate>?, authType: String?) {}
    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}
```

## Example — does not trigger

```kotlin
class AppTrustManager(
    private val delegate: X509TrustManager,
) : X509TrustManager by delegate
```

## Implementation notes

- Dispatch: `class_declaration` with supertype `X509TrustManager` or
  `TrustManager`. Walk body for the two override methods; check each
  for an empty or single-`return` body.
- Shared helper with `allow-all-hostname-verifier` for detecting
  "empty override" shape.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`allow-all-hostname-verifier.md`](allow-all-hostname-verifier.md),
  [`okhttp-disable-ssl-validation.md`](okhttp-disable-ssl-validation.md)
