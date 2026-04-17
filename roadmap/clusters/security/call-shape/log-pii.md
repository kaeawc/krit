# LogPii

**Cluster:** [security/call-shape](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

Logger call (`Log.*` / `Timber.*` / `logger.*`) with a string-template
argument that interpolates a variable whose name matches
`/password|token|secret|apiKey|authHeader|ssn|pan|cvv|jwt/i`.

## Triggers

```kotlin
Log.d("Auth", "sending password=$password for user=$userId")
```

## Does not trigger

```kotlin
Log.d("Auth", "sending auth request for user=$userId")
```

## Dispatch

`call_expression` on logger methods; inspect string-template
arguments for interpolations whose identifier matches the PII regex.

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
- Related: [`print-stack-trace-in-release.md`](print-stack-trace-in-release.md),
  `roadmap/clusters/privacy/analytics-event-with-pii-param-name.md`
