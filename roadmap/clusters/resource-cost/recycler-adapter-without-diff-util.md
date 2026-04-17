# RecyclerAdapterWithoutDiffUtil

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

Class extending `RecyclerView.Adapter` that overrides
`notifyDataSetChanged()` without any `DiffUtil`/`ListAdapter`
usage in the same file.

## Triggers

```kotlin
class UserAdapter : RecyclerView.Adapter<VH>() {
    fun setData(new: List<User>) {
        items = new
        notifyDataSetChanged()
    }
}
```

## Does not trigger

```kotlin
class UserAdapter : ListAdapter<User, VH>(UserDiffCallback) { ... }
```

## Dispatch

`class_declaration` extending `RecyclerView.Adapter` with
`notifyDataSetChanged` in body and no `DiffUtil` import.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
