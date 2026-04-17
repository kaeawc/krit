# MetroGraphFactoryMissingAbstract

**Cluster:** [di-hygiene](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`@GraphFactory` on a concrete class — must be abstract or interface.

## Triggers

```kotlin
@GraphFactory
class AppGraphFactory { fun create(): AppGraph = TODO() }
```

## Does not trigger

```kotlin
@GraphFactory
interface AppGraphFactory { fun create(): AppGraph }
```

## Dispatch

`class_declaration` annotated `@GraphFactory` that is neither
abstract nor interface.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
