# RoomLoadsAllWhereFirstUsed

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`dao.getAll().first()` — loads the entire table for a single
element. Should be a direct `@Query` with `LIMIT 1`.

## Triggers

```kotlin
val first = dao.getAll().first()
```

## Does not trigger

```kotlin
val first = dao.getFirst() // @Query("SELECT * FROM users LIMIT 1")
```

## Dispatch

`call_expression` chain match: `getAll().first()` / `.single()` /
`[0]`.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
