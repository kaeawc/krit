# PrngFromSystemTime

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`Random(System.currentTimeMillis())` / `Random(Date().time)` used
inside a file that imports `javax.crypto` or `java.security`. The
existing `SecureRandom` rule covers naked `Random()`; this covers
the seeded variant.

## Example — triggers

```kotlin
import javax.crypto.Cipher

val rng = Random(System.currentTimeMillis())
val key = ByteArray(32).also { rng.nextBytes(it) }
```

## Example — does not trigger

```kotlin
val rng = SecureRandom()
val key = ByteArray(32).also { rng.nextBytes(it) }
```

## Implementation notes

- Dispatch: `call_expression` for `Random(...)` with a `currentTimeMillis` / `Date().time` arg.
- Gated on the file importing a crypto package — reuses the
  `fileImportsProto` / `fileImportsKsp` helper pattern.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: existing `SecureRandom` in `internal/rules/android_security.go`
