# StaticIv

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`IvParameterSpec` constructed from a literal byte array or from a
literal string `.toByteArray()`. Static IVs break AES-CBC and AES-GCM
confidentiality guarantees.

## Example — triggers

```kotlin
val iv = IvParameterSpec(byteArrayOf(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0))
val cipher = Cipher.getInstance("AES/CBC/PKCS5Padding")
cipher.init(Cipher.ENCRYPT_MODE, key, iv)
```

## Example — does not trigger

```kotlin
val iv = ByteArray(16).also { SecureRandom().nextBytes(it) }
val cipher = Cipher.getInstance("AES/GCM/NoPadding")
cipher.init(Cipher.ENCRYPT_MODE, key, GCMParameterSpec(128, iv))
```

## Implementation notes

- Dispatch: `call_expression` where callee resolves to `IvParameterSpec`
  and the first argument is a literal `byteArrayOf(...)` or a literal
  string followed by `.toByteArray()`.
- Shape helper: reuse `isLiteralByteArray(arg)` from the planned
  security shape helpers.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`hardcoded-secret-key.md`](hardcoded-secret-key.md)
