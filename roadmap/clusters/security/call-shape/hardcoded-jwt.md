# HardcodedJwt

**Cluster:** [security/call-shape](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

String literal matching the three-part base64 JWT shape
`/^eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/`.

## Triggers

```kotlin
val token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.abc123"
```

## Does not trigger

```kotlin
val token = prefs.getString("jwt", null)
```

## Dispatch

Regex match over string-literal nodes.

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
- Related: [`hardcoded-bearer-token.md`](hardcoded-bearer-token.md)
