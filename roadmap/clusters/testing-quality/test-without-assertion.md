# TestWithoutAssertion

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`@Test fun foo()` whose body contains no call to a known assertion
function.

## Triggers

```kotlin
@Test fun loads() { repository.load() }
```

## Does not trigger

```kotlin
@Test fun loads() {
    val result = repository.load()
    assertThat(result).isNotNull()
}
```

## Dispatch

`function_declaration` annotated `@Test`; walk body for any call
matching the assertion allowlist (`assert*`, `verify`, `shouldBe`,
`fail`, `assertThat`, `expectThat`, `confirmVerified`).

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
