# RunTestWithThreadSleep

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`Thread.sleep(...)` inside a `runTest { }` — blocks the virtual
time scheduler.

## Triggers

```kotlin
@Test fun works() = runTest { Thread.sleep(100) }
```

## Does not trigger

```kotlin
@Test fun works() = runTest { advanceTimeBy(100) }
```

## Dispatch

`call_expression` on `Thread.sleep` inside a `runTest` lambda.

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
