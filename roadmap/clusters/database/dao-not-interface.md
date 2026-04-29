# DaoNotInterface

**Cluster:** [database](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`@Dao abstract class FooDao { ... }` — Room prefers interfaces.

## Triggers

```kotlin
@Dao abstract class UserDao { ... }
```

## Does not trigger

```kotlin
@Dao interface UserDao { ... }
```

## Dispatch

`class_declaration` annotated `@Dao` that isn't an interface.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
