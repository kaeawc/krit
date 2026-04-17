# SharedMutableStateInObject

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`@Test` class contains a `companion object` with a `var`, or a
file-level `object` with `var` shared across tests.

## Triggers

```kotlin
class MyTest {
    companion object { var counter = 0 }
    @Test fun a() { counter++ }
    @Test fun b() { assertEquals(0, counter) } // order-dependent
}
```

## Does not trigger

State inside a `@BeforeEach`-initialised instance field.

## Dispatch

Test-file scan for `object`/`companion object` with `var`.

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
- Related: `roadmap/clusters/concurrency/mutable-state-in-object.md`
