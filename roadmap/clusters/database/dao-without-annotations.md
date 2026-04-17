# DaoWithoutAnnotations

**Cluster:** [database](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`@Dao` class with a function that has no
`@Query`/`@Insert`/`@Update`/`@Delete`/`@Transaction`.

## Triggers

```kotlin
@Dao interface UserDao {
    @Query("SELECT * FROM users") fun all(): List<User>
    fun helper(): Int = 0 // unannotated
}
```

## Does not trigger

DAO functions all annotated, or helper moved to companion object.

## Dispatch

Functions inside a `@Dao` without the expected annotation set.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
