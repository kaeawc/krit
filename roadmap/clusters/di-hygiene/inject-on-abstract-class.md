# InjectOnAbstractClass

**Cluster:** [di-hygiene](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`@Inject constructor(...)` on an `abstract class`. Dagger cannot
instantiate it.

## Triggers

```kotlin
abstract class BaseUseCase @Inject constructor(val dep: Dep)
```

## Does not trigger

```kotlin
class ConcreteUseCase @Inject constructor(val dep: Dep) : BaseUseCase(dep)
```

## Dispatch

`class_declaration` with `abstract` modifier and an `@Inject`
primary constructor.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
