# ComposeUnstableParameter

**Cluster:** [compose](README.md) · **Status:** shipped (2026-04-14) · **Severity:** warning · **Default:** active

## Catches

`@Composable fun Foo(items: List<Bar>)` where the parameter is a
`List`/`Map`/`Set` without `@Immutable` or an `ImmutableList` /
`PersistentList` from kotlinx.collections.immutable.

## Triggers

```kotlin
@Composable
fun UserList(users: List<User>) { /* ... */ }
```

## Does not trigger

```kotlin
@Composable
fun UserList(users: ImmutableList<User>) { /* ... */ }

@Immutable data class Users(val users: List<User>)
@Composable fun UserList(users: Users) { /* ... */ }
```

## Dispatch

`function_declaration` annotated `@Composable` whose parameters
include `List<T>` / `Map<K, V>` / `Set<T>` without a stability hint.

## Links

- Parent: [`roadmap/54-compose-correctness-rules.md`](../../54-compose-correctness-rules.md)
