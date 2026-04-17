# WeakKeySize

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`KeyPairGenerator.initialize(N)` where `N < 2048` for RSA, or
`KeyGenerator.init(N)` where `N < 128` for AES.

## Example — triggers

```kotlin
val kpg = KeyPairGenerator.getInstance("RSA")
kpg.initialize(1024)
```

## Example — does not trigger

```kotlin
val kpg = KeyPairGenerator.getInstance("RSA")
kpg.initialize(2048)
```

## Implementation notes

- Dispatch: `call_expression` on `.initialize(...)` / `.init(...)`
  where the receiver resolves to `KeyPairGenerator` / `KeyGenerator`.
- Reads the integer literal; if missing, bail. No type inference
  strictly required, but the oracle sharpens the receiver check.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`hardcoded-secret-key.md`](hardcoded-secret-key.md)
