# WholeGraphBindingCompleteness

**Cluster:** [di-graph](README.md) · **Status:** planned · **Severity:** warning · **Default:** inactive

## Catches

A Dagger / Hilt / Metro component whose transitively needed bindings
cannot all be resolved through its declared modules.

## Triggers

```kotlin
@Component(modules = [ApiModule::class])
interface AppComponent {
    fun cache(): UserCache // UserCache needs DiskDao, not bound anywhere reachable
}
```

## Does not trigger

The transitive closure of required types is covered by the module
list.

## Dispatch

Component walk: starting from a component's exposed functions,
resolve each return type, follow `@Inject` constructors and
`@Provides`/`@Binds` until closure. Missing links emit findings.

This is a static reproduction of Dagger's kapt/ksp completeness
check without running codegen.

## Links

- Parent: [`../README.md`](../README.md)
- Related: `roadmap/clusters/di-hygiene/component-missing-module.md`
