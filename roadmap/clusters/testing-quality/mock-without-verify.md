# MockWithoutVerify

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** info · **Default:** active

## Catches

`mockk<Foo>()` / `mock(Foo::class.java)` assigned to a local that
is never referenced on the LHS of a `verify` or `every` call.

## Triggers

```kotlin
@Test fun load() {
    val api = mockk<Api>()
    val repo = Repo(api)
    repo.load()
}
```

## Does not trigger

```kotlin
@Test fun load() {
    val api = mockk<Api> { every { get() } returns data }
    val repo = Repo(api)
    repo.load()
    verify { api.get() }
}
```

## Dispatch

`property_declaration` whose RHS is `mockk<>()` / `mock(...)`; walk
enclosing function for `verify(name)` / `every { name. }`.

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
