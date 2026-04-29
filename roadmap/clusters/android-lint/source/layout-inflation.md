# LayoutInflation

**Cluster:** [android-lint](../README.md) · **Sub-cluster:** source · **Status:** shipped ·
**Severity:** warning · **Default:** active

## What it catches

`LayoutInflater.inflate(resource, null)` or `inflate(resource, null, false)` calls where the caller clearly has access to a non-null parent `ViewGroup`. Passing `null` as the parent causes the inflated view to lose the layout parameters defined in XML (e.g., `layout_width`, `layout_gravity`), leading to views that are sized or positioned incorrectly at runtime.

## Example — triggers

```kotlin
class ItemAdapter : RecyclerView.Adapter<ItemAdapter.ViewHolder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): ViewHolder {
        // parent is available but null is passed — layout params are dropped
        val view = LayoutInflater.from(parent.context)
            .inflate(R.layout.item_row, null)
        return ViewHolder(view)
    }
}
```

## Example — does not trigger

```kotlin
class ItemAdapter : RecyclerView.Adapter<ItemAdapter.ViewHolder>() {
    override fun onCreateViewHolder(parent: ViewGroup, viewType: Int): ViewHolder {
        val view = LayoutInflater.from(parent.context)
            .inflate(R.layout.item_row, parent, false)
        return ViewHolder(view)
    }
}
```

## Implementation notes

- Dispatch: `call_expression`
- Infra reuse: `internal/rules/android_source.go`
- Effort: Medium — requires tracking the enclosing function's parameter list to determine whether a non-null `ViewGroup` is in scope, then correlating it with the `inflate()` call's second argument
- Related: `LayoutInflationDetector` (AOSP), `ViewConstructorRule`

## Links

- Parent overview: [`../README.md`](../README.md)
