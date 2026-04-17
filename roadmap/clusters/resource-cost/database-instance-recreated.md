# DatabaseInstanceRecreated

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`Room.databaseBuilder(...)` inside a function that isn't an
`@Provides`, `@Module`, or companion-object initializer — should
be a singleton.

## Triggers

```kotlin
fun loadUsers(): List<User> {
    val db = Room.databaseBuilder(context, AppDb::class.java, "app.db").build()
    return db.userDao().all()
}
```

## Does not trigger

```kotlin
@Provides @Singleton
fun provideDb(@ApplicationContext ctx: Context): AppDb =
    Room.databaseBuilder(ctx, AppDb::class.java, "app.db").build()
```

## Dispatch

`call_expression` on `Room.databaseBuilder` inside a regular
function body.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
