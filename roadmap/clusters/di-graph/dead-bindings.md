# DeadBindings

**Cluster:** [di-graph](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

A `@Provides`/`@Binds` function whose return type is never injected
anywhere reachable from any component.

## Triggers

```kotlin
@Provides fun provideFoo(): Foo = FooImpl()
// No class has `@Inject constructor(foo: Foo)` or a `foo(): Foo` component exposure
```

## Does not trigger

Binding is reachable from at least one component.

## Dispatch

Whole-graph walk + reachability analysis. Analogous to
`UnusedPrivateFunction` but over the DI binding graph.

## Links

- Parent: [`../README.md`](../README.md)
