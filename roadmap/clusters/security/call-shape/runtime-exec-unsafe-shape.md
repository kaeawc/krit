# RuntimeExecUnsafeShape

**Cluster:** [security/call-shape](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`Runtime.getRuntime().exec(...)` with a single-string argument that
contains interpolation or concatenation.

## Triggers

```kotlin
Runtime.getRuntime().exec("ls -la $userPath")
```

## Does not trigger

```kotlin
Runtime.getRuntime().exec(arrayOf("ls", "-la", userPath))
```

## Dispatch

`call_expression` on `Runtime.exec(...)` with a string argument.
Shape helper reused.

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
- Related: [`process-builder-shell-arg.md`](process-builder-shell-arg.md),
  `roadmap/clusters/security/taint/command-injection.md`
