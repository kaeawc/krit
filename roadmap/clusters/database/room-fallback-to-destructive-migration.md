# RoomFallbackToDestructiveMigration

**Cluster:** [database](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

`Room.databaseBuilder(...).fallbackToDestructiveMigration()` outside
a debug module — silent data loss on version bump.

## Triggers

```kotlin
Room.databaseBuilder(context, AppDb::class.java, "app.db")
    .fallbackToDestructiveMigration()
    .build()
```

## Does not trigger

Same call inside `src/debug/` or wrapped in `if (BuildConfig.DEBUG)`.

## Dispatch

`call_expression` chain match; skip when the enclosing file lives
in a debug source set.

## Links

- Parent: [`roadmap/57-database-room-rules.md`](../../57-database-room-rules.md)
