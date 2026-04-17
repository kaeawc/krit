# EncryptWithoutAuthentication

**Cluster:** [security/taint](README.md) · **Status:** deferred

## Catches

AES/CBC or AES/ECB (already caught by tier-1) whose ciphertext
reaches a user-visible sink (network, content provider, intent extra)
without passing through an HMAC / MAC pass on the way out.

## Shape

```kotlin
val ciphertext = cbcCipher.doFinal(plaintext) // no MAC
publish(Intent("bcast").putExtra("payload", ciphertext))
```

## Links

- Parent: [`roadmap/51-security-rules-taint.md`](../../../51-security-rules-taint.md)
- Related: [`../syntactic/static-iv.md`](../syntactic/static-iv.md)
