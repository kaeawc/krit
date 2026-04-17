# TestInheritanceDepth

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`@Test` class whose inheritance chain is > 2 deep.

## Triggers

```kotlin
abstract class BaseTest { ... }
abstract class MiddleTest : BaseTest() { ... }
class UserTest : MiddleTest() { @Test fun load() { ... } }
```

## Does not trigger

At most one intermediate base class.

## Dispatch

Test class supertype walk via the cross-file index.

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
