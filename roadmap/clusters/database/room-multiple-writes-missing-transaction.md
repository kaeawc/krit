# RoomMultipleWritesMissingTransaction

**Cluster:** [database](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

DAO function body with 2+ `@Insert`/`@Update`/`@Delete` calls where
the function itself is not `@Transaction`.

## Triggers

```kotlin
suspend fun save(user: User, prefs: Prefs) {
    insertUser(user)
    insertPrefs(prefs)
}
```

## Does not trigger

```kotlin
@Transaction
suspend fun save(user: User, prefs: Prefs) {
    insertUser(user)
    insertPrefs(prefs)
}
```

## Dispatch

`function_declaration` inside a `@Dao` whose body contains multiple
`@Insert`/`@Update`/`@Delete` call-sites without `@Transaction` on
the enclosing function.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
