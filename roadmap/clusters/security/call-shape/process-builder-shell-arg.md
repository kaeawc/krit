# ProcessBuilderShellArg

**Cluster:** [security/call-shape](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`ProcessBuilder(listOf("sh", "-c", ...))` / `ProcessBuilder("bash", "-c", ...)`
where the last argument has interpolation.

## Triggers

```kotlin
ProcessBuilder("sh", "-c", "grep $pattern /var/log/app.log").start()
```

## Does not trigger

```kotlin
ProcessBuilder("grep", pattern, "/var/log/app.log").start()
```

## Dispatch

`call_expression` on `ProcessBuilder` with `"sh"` or `"bash"` as one
of the first args and a `"-c"` second arg.

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
- Related: [`runtime-exec-unsafe-shape.md`](runtime-exec-unsafe-shape.md)
