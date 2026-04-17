# ForeignKeyWithoutOnDelete

**Cluster:** [database](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** active

## Catches

`@ForeignKey(...)` without `onDelete = CASCADE/RESTRICT/SET_NULL`.
Default is NO_ACTION, which usually means stale rows.

## Triggers

```kotlin
@Entity(foreignKeys = [ForeignKey(
    entity = Team::class,
    parentColumns = ["id"],
    childColumns = ["teamId"],
)])
data class User(...)
```

## Does not trigger

```kotlin
ForeignKey(
    entity = Team::class,
    parentColumns = ["id"],
    childColumns = ["teamId"],
    onDelete = ForeignKey.CASCADE,
)
```

## Dispatch

`annotation` / `call_expression` for `ForeignKey` constructor without
the named arg `onDelete`.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
