# OkHttpDisableSslValidation

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`OkHttpClient.Builder()` chain that calls `.hostnameVerifier(...)`
with a lambda that returns `true`, or `.sslSocketFactory(...)` passing
a trust-all `X509TrustManager`.

## Example — triggers

```kotlin
val client = OkHttpClient.Builder()
    .hostnameVerifier { _, _ -> true }
    .sslSocketFactory(unsafeSocketFactory, unsafeTrustManager)
    .build()
```

## Example — does not trigger

```kotlin
val client = OkHttpClient.Builder().build()
```

## Implementation notes

- Dispatch: `call_expression` chain containing `.hostnameVerifier(...)` or `.sslSocketFactory(...)`.
- Cooperates with [`allow-all-hostname-verifier.md`](allow-all-hostname-verifier.md) and [`insecure-trust-manager.md`](insecure-trust-manager.md) — those detect the class implementations, this detects the call-site wiring.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
