# ConcurrentModificationIteration

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`for (x in list) { list.remove(x) }` — ConcurrentModificationException.

## Triggers

```kotlin
for (x in items) { if (x.stale) items.remove(x) }
```

## Does not trigger

```kotlin
items.removeAll { it.stale }
val iter = items.iterator()
while (iter.hasNext()) {
    if (iter.next().stale) iter.remove()
}
```

## Dispatch

`for_statement` whose body contains a `.remove(...)`/`.add(...)`
call on the same receiver as the iterable.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
