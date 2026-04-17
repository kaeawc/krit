# HardcodedGcpServiceAccount

**Cluster:** [security/call-shape](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

String literal containing `"type": "service_account"` or starting
with `-----BEGIN PRIVATE KEY-----` in non-`.pem` source files.

## Triggers

```kotlin
val serviceAccount = """
    {"type": "service_account", "project_id": "my-proj", ...}
""".trimIndent()
```

## Does not trigger

```kotlin
val serviceAccount = File("service-account.json").readText()
```

## Dispatch

String-literal scan with both markers. Skips `.pem` / `.json`
resource files via path check.

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
