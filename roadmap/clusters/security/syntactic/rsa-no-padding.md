# RsaNoPadding

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`Cipher.getInstance("RSA/ECB/NoPadding" | "RSA/NONE/NoPadding")` —
textbook RSA is malleable and leaks plaintext structure. Should be
`OAEPPadding` or `PKCS1Padding`.

## Example — triggers

```kotlin
val cipher = Cipher.getInstance("RSA/ECB/NoPadding")
```

## Example — does not trigger

```kotlin
val cipher = Cipher.getInstance("RSA/ECB/OAEPWithSHA-256AndMGF1Padding")
```

## Implementation notes

- Dispatch: `call_expression` matching `Cipher.getInstance` with a
  string literal that contains `NoPadding` and starts with `RSA/`.
- Sibling of existing `GetInstance` rule (ECB/DES detection).

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: existing `GetInstance` rule in `internal/rules/android_security.go`
