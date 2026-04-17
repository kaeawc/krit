# ProviderInsteadOfLazy

**Cluster:** [di-hygiene](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

Constructor parameter typed `Provider<Foo>` whose body calls
`.get()` exactly once. `Lazy<Foo>` matches the intent and is cheaper.

## Triggers

```kotlin
class Presenter @Inject constructor(
    private val api: Provider<Api>,
) {
    fun load() = api.get().fetch()
}
```

## Does not trigger

```kotlin
class Presenter @Inject constructor(
    private val api: Lazy<Api>,
) {
    fun load() = api.get().fetch()
}
```

## Dispatch

`property_declaration` / `class_parameter` typed `Provider<...>`;
count call-sites of `.get()` across the class body.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
- Related: [`lazy-instead-of-direct.md`](lazy-instead-of-direct.md)
