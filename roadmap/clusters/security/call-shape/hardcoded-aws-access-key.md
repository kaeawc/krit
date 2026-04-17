# HardcodedAwsAccessKey

**Cluster:** [security/call-shape](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** active

## Catches

String literal matching `^AKIA[0-9A-Z]{16}$` — AWS IAM access key id.

## Triggers

```kotlin
const val AWS_KEY = "AKIAIOSFODNN7EXAMPLE"
```

## Does not trigger

```kotlin
val awsKey = System.getenv("AWS_ACCESS_KEY_ID")
```

## Dispatch

String-literal scan with a precise regex. High-precision detector,
so ships active by default.

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
- Related: [`hardcoded-jwt.md`](hardcoded-jwt.md),
  [`hardcoded-bearer-token.md`](hardcoded-bearer-token.md)
