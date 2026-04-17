# HardcodedSlackWebhook

**Cluster:** [security/call-shape](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

String literal starting with `https://hooks.slack.com/services/T`.

## Triggers

```kotlin
const val ALERT_WEBHOOK = "https://hooks.slack.com/services/<team>/<channel>/<secret-token>"
```

## Does not trigger

```kotlin
val webhook = System.getenv("ALERT_WEBHOOK")
```

## Dispatch

String-literal prefix match.

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
