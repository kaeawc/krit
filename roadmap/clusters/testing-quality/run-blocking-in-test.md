# RunBlockingInTest

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** info · **Default:** active

## Catches

`runBlocking { ... }` in a `@Test` function in a module with
`kotlinx-coroutines-test` on the classpath — should be `runTest`.

## Triggers

```kotlin
@Test fun works() = runBlocking { service.load() }
```

## Does not trigger

```kotlin
@Test fun works() = runTest { service.load() }
```

## Dispatch

`call_expression` on `runBlocking` inside a `@Test` function;
requires BuildGraph lookup for the test classpath.

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
