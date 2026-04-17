# HardcodedLogTag

**Cluster:** [release-engineering](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`Log.d("MainActivity", ...)` where the tag matches the enclosing
class simple name — should be hoisted to a companion `TAG` constant.

## Triggers

```kotlin
class MainActivity : Activity() {
    fun load() { Log.d("MainActivity", "loading") }
}
```

## Does not trigger

```kotlin
class MainActivity : Activity() {
    companion object { private const val TAG = "MainActivity" }
    fun load() { Log.d(TAG, "loading") }
}
```

## Dispatch

`call_expression` on `Log.<level>` with a string-literal tag
matching the enclosing class name.

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
