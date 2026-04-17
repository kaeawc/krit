# AllowAllHostnameVerifier

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

Class implementing `HostnameVerifier` whose `verify(...)` body is a
single `return true` or `= true`.

## Example — triggers

```kotlin
class AllowAll : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean = true
}
```

## Example — does not trigger

```kotlin
class StrictHostVerifier : HostnameVerifier {
    override fun verify(hostname: String, session: SSLSession): Boolean =
        hostname == session.peerHost && defaultVerifier.verify(hostname, session)
}
```

## Implementation notes

- Dispatch: `class_declaration` whose supertype text contains
  `HostnameVerifier`.
- Walk `class_body` for `function_declaration` named `verify`; check
  its body shape.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
- Related: [`insecure-trust-manager.md`](insecure-trust-manager.md),
  [`okhttp-disable-ssl-validation.md`](okhttp-disable-ssl-validation.md)
