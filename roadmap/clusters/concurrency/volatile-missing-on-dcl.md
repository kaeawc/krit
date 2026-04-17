# VolatileMissingOnDcl

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

Double-checked locking pattern where the field is not `@Volatile`.

## Triggers

```kotlin
object Singleton {
    private var instance: Foo? = null
    fun get(): Foo {
        if (instance == null) {
            synchronized(this) {
                if (instance == null) instance = Foo()
            }
        }
        return instance!!
    }
}
```

## Does not trigger

```kotlin
object Singleton {
    @Volatile private var instance: Foo? = null
    // ... same body
}
// or by lazy
object Singleton { val instance by lazy { Foo() } }
```

## Dispatch

`property_declaration` with a three-statement DCL pattern: nullable
`var`, outer null check, `synchronized` block, inner null check,
assignment.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
