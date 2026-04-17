# LazyInsteadOfDirect

**Cluster:** [di-hygiene](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

Constructor parameter typed `Lazy<Foo>` whose `.get()` is called
unconditionally at class init time — direct injection is cheaper.

## Triggers

```kotlin
class Presenter @Inject constructor(private val api: Lazy<Api>) {
    private val loaded = api.get().initial()
}
```

## Does not trigger

```kotlin
class Presenter @Inject constructor(private val api: Api) {
    private val loaded = api.initial()
}
```

## Dispatch

`class_parameter` typed `Lazy<T>` whose `.get()` is called in an
initializer / `init` block.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
- Related: [`provider-instead-of-lazy.md`](provider-instead-of-lazy.md)
