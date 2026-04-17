# RecyclerAdapterStableIdsDefault

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`RecyclerView.Adapter` that doesn't override `setHasStableIds(true)`
and doesn't use `ListAdapter`.

## Triggers

```kotlin
class UserAdapter : RecyclerView.Adapter<VH>() { ... }
```

## Does not trigger

```kotlin
class UserAdapter : RecyclerView.Adapter<VH>() {
    init { setHasStableIds(true) }
    override fun getItemId(position: Int): Long = items[position].id
}
```

## Dispatch

`class_declaration` extending `RecyclerView.Adapter` with no
`setHasStableIds` call.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
