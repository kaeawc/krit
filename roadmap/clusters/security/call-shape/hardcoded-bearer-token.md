# HardcodedBearerToken

**Cluster:** [security/call-shape](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`"Bearer <literal>"` or `"Bearer ${literal}"` where the literal is
≥ 16 characters and not a placeholder.

## Triggers

```kotlin
request.header("Authorization", "Bearer sk_live_abcdef0123456789")
```

## Does not trigger

```kotlin
request.header("Authorization", "Bearer ${BuildConfig.API_TOKEN}")
```

## Dispatch

String-template scanner inspecting string-literal nodes that begin
with `Bearer `.

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
- Related: [`hardcoded-jwt.md`](hardcoded-jwt.md),
  [`hardcoded-slack-webhook.md`](hardcoded-slack-webhook.md)
