# WeakMessageDigest

**Cluster:** [security/syntactic](README.md) · **Status:** planned ·
**Severity:** warning · **Default:** active

## What it catches

`MessageDigest.getInstance("MD5" | "SHA-1" | "MD2")` used for
security-relevant hashing. MD5 and SHA-1 are collision-broken; MD2 is
pre-broken. Legitimate non-security uses (file integrity for a local
cache, checksumming for a client-server etag) should move to a
narrower API or suppress the rule.

## Example — triggers

```kotlin
import java.security.MessageDigest

fun fingerprint(password: ByteArray): ByteArray {
    val md = MessageDigest.getInstance("MD5")
    return md.digest(password)
}
```

## Example — does not trigger

```kotlin
import java.security.MessageDigest

fun fingerprint(password: ByteArray): ByteArray {
    val md = MessageDigest.getInstance("SHA-256")
    return md.digest(password)
}
```

## Implementation notes

- Dispatch: `call_expression` where callee text matches
  `MessageDigest.getInstance` and the single argument is a string
  literal in the weak set.
- Infra reuse: `hasAnnotationNamed`, `scanner.NodeText`.
- The same pattern as the existing `GetInstance` rule for cipher
  algorithms; see `internal/rules/android_security.go`.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`weak-mac-algorithm.md`](weak-mac-algorithm.md),
  [`rsa-no-padding.md`](rsa-no-padding.md)
