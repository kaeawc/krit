# RoomRelationWithoutIndex

**Cluster:** [database](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`@Relation(parentColumn=..., entityColumn=...)` referencing a
column that isn't in the entity's `indices = [...]`.

## Triggers

```kotlin
@Entity data class Post(@PrimaryKey val id: Long, val userId: Long)

data class UserWithPosts(
    @Embedded val user: User,
    @Relation(parentColumn = "id", entityColumn = "userId")
    val posts: List<Post>,
)
// Post has no Index("userId")
```

## Does not trigger

```kotlin
@Entity(indices = [Index("userId")])
data class Post(...)
```

## Dispatch

Cross-file: resolve `@Relation` entity to its `@Entity` declaration,
check `indices` list.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
