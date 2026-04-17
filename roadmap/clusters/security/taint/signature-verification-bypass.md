# SignatureVerificationBypass

**Cluster:** [security/taint](README.md) · **Status:** deferred

## Catches

Signature verification over a value whose input flowed from storage /
network without integrity checks earlier in the flow graph.

## Shape

```kotlin
val payload = prefs.getString("signed_payload", null) ?: return
val sig = prefs.getString("sig", null) ?: return
Signature.getInstance("SHA256withRSA").apply {
    initVerify(pub); update(payload.toByteArray())
}.verify(sig.toByteArray())
```

## Links

- Parent: [`roadmap/51-security-rules-taint.md`](../../../51-security-rules-taint.md)
