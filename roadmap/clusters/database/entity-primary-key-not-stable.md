# EntityPrimaryKeyNotStable

**Cluster:** [database](README.md) · **Status:** in_progress · **Severity:** warning · **Default:** active

## Catches

`@PrimaryKey var id: Long = 0` with `autoGenerate = false` — mutable
PK breaks `equals`/`hashCode` contract.

## Triggers

```kotlin
@Entity data class User(@PrimaryKey var id: Long = 0, val name: String)
```

## Does not trigger

```kotlin
@Entity data class User(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val name: String,
)
```

## Dispatch

`property_declaration` inside `@Entity` annotated `@PrimaryKey`
with `var` modifier.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
