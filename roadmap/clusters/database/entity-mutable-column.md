# EntityMutableColumn

**Cluster:** [database](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`@Entity data class User(val name: String, var lastLogin: Long)`
— `var` column in an entity prevents straightforward copy-on-write.

## Triggers

```kotlin
@Entity data class User(@PrimaryKey val id: Long, var name: String)
```

## Does not trigger

```kotlin
@Entity data class User(@PrimaryKey val id: Long, val name: String)
```

## Dispatch

`class_declaration` annotated `@Entity` with `var` class parameters.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
