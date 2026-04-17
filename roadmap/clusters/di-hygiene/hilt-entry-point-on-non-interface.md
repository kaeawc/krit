# HiltEntryPointOnNonInterface

**Cluster:** [di-hygiene](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`@EntryPoint` annotation on a class or object — must be on an
interface.

## Triggers

```kotlin
@EntryPoint
@InstallIn(SingletonComponent::class)
class LegacyEntryPoint { ... }
```

## Does not trigger

```kotlin
@EntryPoint
@InstallIn(SingletonComponent::class)
interface LegacyEntryPoint { fun api(): Api }
```

## Dispatch

`class_declaration` annotated `@EntryPoint` whose kind isn't interface.

## Links

- Parent: [`roadmap/55-di-hygiene-rules.md`](../../55-di-hygiene-rules.md)
