# RoomSuspendQueryInTransaction

**Cluster:** [database](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`suspend fun` with `@Query` inside a `@Transaction` function — Room
already wraps suspending queries; double-wrapping breaks cancellation.

## Triggers

```kotlin
@Transaction
suspend fun load(id: Long): UserWithPosts {
    val user = getUser(id) // @Query suspend
    return UserWithPosts(user, getPosts(id))
}
```

## Does not trigger

```kotlin
@Transaction
fun loadBlocking(id: Long): UserWithPosts = /* blocking DAO ops */
```

## Dispatch

Suspend DAO function with both `@Transaction` and calls into other
suspend `@Query` functions on the same DAO.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
