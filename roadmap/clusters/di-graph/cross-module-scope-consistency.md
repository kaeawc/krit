# CrossModuleScopeConsistency

**Cluster:** [di-graph](README.md) · **Status:** shipped · **Severity:** warning · **Default:** inactive

## Catches

A `@Singleton` type whose transitive dependency graph includes a
type scoped at a narrower lifecycle (e.g. `@ActivityScoped`).

## Triggers

```kotlin
@Singleton
class UserRepository @Inject constructor(
    private val router: Router, // @ActivityScoped
)
```

## Does not trigger

All transitive dependencies match or widen the scope.

## Dispatch

Binding graph walk + scope annotation propagation.

## Links

- Parent: [`../README.md`](../README.md)
- Related: [`whole-graph-binding-completeness.md`](whole-graph-binding-completeness.md)
