# CollectionsSynchronizedListIteration

**Cluster:** [concurrency](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

`for (x in Collections.synchronizedList(list))` — iteration
requires external synchronization even on a synchronized list.

## Triggers

```kotlin
for (x in Collections.synchronizedList(list)) { work(x) }
```

## Does not trigger

```kotlin
synchronized(list) {
    for (x in list) { work(x) }
}
// or a copy-on-write container
```

## Dispatch

`for_statement` whose iterable expression is a call to
`Collections.synchronizedList` / `synchronizedSet` / `synchronizedMap`.

## Links

- Parent: [`roadmap/56-concurrency-coroutines-rules.md`](../../56-concurrency-coroutines-rules.md)
