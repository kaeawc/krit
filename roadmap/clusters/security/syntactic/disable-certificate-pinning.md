# DisableCertificatePinning

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`CertificatePinner.Builder()` chain ending in `.build()` with no
`.add(...)` calls in between — an effectively empty pinner.

## Example — triggers

```kotlin
val pinner = CertificatePinner.Builder().build()
```

## Example — does not trigger

```kotlin
val pinner = CertificatePinner.Builder()
    .add("api.example.com", "sha256/AAAA...")
    .build()
```

## Implementation notes

- Dispatch: `call_expression` chain root at `CertificatePinner.Builder()`.
- Walk the chain; if `.build()` follows with no `.add(...)` in
  between, flag.
- Reuses the call-chain walker pattern from
  `internal/rules/performance.go`.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`okhttp-disable-ssl-validation.md`](okhttp-disable-ssl-validation.md)
