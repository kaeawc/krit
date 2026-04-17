# PublicToInternalLeakyAbstraction

**Cluster:** [architecture](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

A public class that wraps a single internal class one-to-one —
leaky abstraction.

## Triggers

```kotlin
class UserService(private val impl: InternalUserService) {
    fun get(id: Long) = impl.get(id)
    fun save(u: User) = impl.save(u)
}
```

## Does not trigger

Public class adds real behavior, multiple collaborators, or a
genuinely different API shape.

## Dispatch

Heuristic: public class with a single private constructor param
whose methods only delegate.

## Links

- Parent: [`../README.md`](../README.md)
