# ImageLoaderNoMemoryCache

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`Glide.with(...).load(...).into(...)` / `Coil.load(...)` without
`.diskCachePolicy` / `.memoryCachePolicy` in a list/Recycler
context.

## Triggers

```kotlin
fun bind(holder: VH, item: Item) {
    Glide.with(holder.view).load(item.url).into(holder.image)
}
```

## Does not trigger

```kotlin
Glide.with(holder.view)
    .load(item.url)
    .diskCacheStrategy(DiskCacheStrategy.RESOURCE)
    .into(holder.image)
```

## Dispatch

`call_expression` chain match; enclosing-function-is-hot-path
check.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
