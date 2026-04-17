# ViewTag

**Cluster:** [android-lint](../README.md) · **Sub-cluster:** source · **Status:** planned ·
**Severity:** warning · **Default:** active

## What it catches

`View.setTag(Object)` calls where the value passed is a framework-provided or application-level object that holds a reference back to a `View` (e.g., a `ViewHolder` that stores view references). On Android API < 14 this creates a memory leak because the tag is stored in a static map keyed by the view; the GC cannot collect the view or anything the tag references even after the activity is destroyed.

## Example — triggers

```kotlin
class LegacyListAdapter : BaseAdapter() {
    override fun getView(position: Int, convertView: View?, parent: ViewGroup): View {
        val view = convertView ?: layoutInflater.inflate(R.layout.item, parent, false)
        val holder = ViewHolder(
            title = view.findViewById(R.id.title),
            subtitle = view.findViewById(R.id.subtitle)
        )
        view.setTag(holder) // holder holds view references — memory leak risk
        return view
    }

    data class ViewHolder(val title: TextView, val subtitle: TextView)
}
```

## Example — does not trigger

```kotlin
class ModernListAdapter : RecyclerView.Adapter<ModernListAdapter.ViewHolder>() {
    // RecyclerView.ViewHolder manages view references safely — no setTag needed
    class ViewHolder(view: View) : RecyclerView.ViewHolder(view) {
        val title: TextView = view.findViewById(R.id.title)
        val subtitle: TextView = view.findViewById(R.id.subtitle)
    }
}

// setTag with a simple scalar is also safe
fun markSelected(view: View, isSelected: Boolean) {
    view.setTag(R.id.tag_selected, isSelected) // two-arg overload with resource key is fine
}
```

## Implementation notes

- Dispatch: `call_expression`
- Infra reuse: `internal/rules/android_source.go`
- Effort: Small — match single-argument `setTag(obj)` calls on a `View` receiver; flag when the argument's static type is a user-defined class or any type that contains `View` fields (heuristic: class name contains "Holder" or "Tag")
- Related: `ViewTagDetector` (AOSP)

## Links

- Parent overview: [`../README.md`](../README.md)
