# ComponentMissingModule

**Cluster:** [di-hygiene](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** inactive

## Catches

`@Component(modules = [...])` whose `modules` list does not include
a module referenced by a transitive `@Binds` / `@Provides` reachable
through the component's bindings.

## Triggers

```kotlin
@Component(modules = [AModule::class])
interface App {
    fun providesFoo(): Foo // Foo is bound in BModule, which is missing
}
```

## Does not trigger

```kotlin
@Component(modules = [AModule::class, BModule::class])
interface App { fun providesFoo(): Foo }
```

## Dispatch

Cross-file walk: resolve component's transitively needed bindings,
compare to `modules = [...]` declaration.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
- Related: `roadmap/clusters/di-graph/whole-graph-binding-completeness.md`
