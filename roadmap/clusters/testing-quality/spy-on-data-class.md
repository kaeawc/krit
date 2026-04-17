# SpyOnDataClass

**Cluster:** [testing-quality](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`spyk(Foo())` / `spy(Foo())` where `Foo` is a `data class` — data
classes compare by value, spy semantics rarely work as intended.

## Triggers

```kotlin
val user = spyk(User("alice"))
```

## Does not trigger

```kotlin
val user = User("alice")
```

## Dispatch

`call_expression` on `spy`/`spyk` whose argument constructs a
`data class`.

## Links

- Parent: [`roadmap/59-testing-quality-rules.md`](../../59-testing-quality-rules.md)
