# HardcodedSecretKey

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`SecretKeySpec` constructed from a literal `byteArrayOf(...)` or from
a literal string. Keys belong in the keystore / secret manager.

## Example — triggers

```kotlin
val key = SecretKeySpec("p@ssw0rd12345678".toByteArray(), "AES")
```

## Example — does not trigger

```kotlin
val key = keyStore.getKey("alias", null) as SecretKey
```

## Implementation notes

- Dispatch: `call_expression` targeting `SecretKeySpec` with a literal
  first argument.
- Shares `isLiteralByteArray` helper with `static-iv`.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`static-iv.md`](static-iv.md),
  [`weak-key-size.md`](weak-key-size.md)
