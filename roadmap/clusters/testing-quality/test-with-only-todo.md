# TestWithOnlyTodo

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`@Test fun foo() { TODO() }` or `@Test fun foo() { fail("unimplemented") }`
— passing stub that should be `@Ignore`d.

## Triggers

```kotlin
@Test fun loads() { TODO() }
```

## Does not trigger

```kotlin
@Test @Ignore fun loads() { TODO() }
```

## Dispatch

Test function whose body is exactly one `TODO()` or
`fail(...)` call.

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
