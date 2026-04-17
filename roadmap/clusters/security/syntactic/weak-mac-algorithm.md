# WeakMacAlgorithm

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`Mac.getInstance("HmacMD5" | "HmacSHA1")` — HMAC with broken hash.

## Example — triggers

```kotlin
import javax.crypto.Mac

val mac = Mac.getInstance("HmacSHA1")
mac.init(key)
```

## Example — does not trigger

```kotlin
val mac = Mac.getInstance("HmacSHA256")
```

## Implementation notes

- Dispatch: `call_expression` matching `Mac.getInstance` with a weak-algorithm string literal.
- Infra reuse: same helper as `weak-message-digest`.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`weak-message-digest.md`](weak-message-digest.md)
