# RoomExportSchemaDisabled

**Cluster:** [database](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`@Database(exportSchema = false)` in a production module — loses
schema history.

## Triggers

```kotlin
@Database(entities = [User::class], version = 3, exportSchema = false)
abstract class AppDb : RoomDatabase()
```

## Does not trigger

```kotlin
@Database(entities = [User::class], version = 3)
abstract class AppDb : RoomDatabase()
```

## Dispatch

`class_declaration` annotated `@Database` with `exportSchema = false`.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
