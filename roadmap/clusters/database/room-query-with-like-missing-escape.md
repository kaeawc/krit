# RoomQueryWithLikeMissingEscape

**Cluster:** [database](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`@Query("... WHERE name LIKE :q")` with no `%` embedding or escape
directives — suggests the binding is used as exact-match.

## Triggers

```kotlin
@Query("SELECT * FROM users WHERE name LIKE :q")
fun search(q: String): List<User>
```

## Does not trigger

```kotlin
@Query("SELECT * FROM users WHERE name LIKE '%' || :q || '%' ESCAPE '\\'")
fun search(q: String): List<User>
```

## Dispatch

`@Query` whose SQL contains `LIKE :<name>` without embedding.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
