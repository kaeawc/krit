# AnvilMergeComponentEmptyScope

**Cluster:** [di-hygiene](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`@MergeComponent(SomeScope::class)` whose scope is not declared by
any `@ContributesTo` / `@ContributesBinding` in the project — the
merged component is empty.

## Triggers

```kotlin
@MergeComponent(UnusedScope::class)
interface AppComponent
// No @ContributesTo(UnusedScope::class) exists
```

## Does not trigger

```kotlin
@MergeComponent(AppScope::class)
interface AppComponent
```

## Dispatch

Cross-file scope reference: ensure the scope literal has at least
one contribution elsewhere in the project.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
