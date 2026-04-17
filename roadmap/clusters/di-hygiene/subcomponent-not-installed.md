# SubcomponentNotInstalled

**Cluster:** [di-hygiene](README.md) · **Status:** planned · **Severity:** warning · **Default:** inactive

## Catches

`@Subcomponent` declared but never returned from a parent component's
builder — orphan subcomponent.

## Triggers

```kotlin
@Subcomponent
interface UserSubcomponent {
    @Subcomponent.Factory interface Factory { fun create(): UserSubcomponent }
}

// No parent component exposes Factory
```

## Does not trigger

```kotlin
@Component(modules = [...])
interface AppComponent {
    fun userSub(): UserSubcomponent.Factory
}
```

## Dispatch

Cross-file reference: every `@Subcomponent` must be referenced by
some `@Component` builder method.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
