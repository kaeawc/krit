# RecyclerViewInLazyColumn

**Cluster:** [resource-cost](README.md) · **Status:** shipped · **Severity:** warning · **Default:** active

## Catches

XML layout: `RecyclerView` nested inside a scrollable parent
without `android:nestedScrollingEnabled="false"`.

## Triggers

```xml
<ScrollView>
    <androidx.recyclerview.widget.RecyclerView ... />
</ScrollView>
```

## Does not trigger

```xml
<androidx.core.widget.NestedScrollView>
    <androidx.recyclerview.widget.RecyclerView
        android:nestedScrollingEnabled="false" ... />
</androidx.core.widget.NestedScrollView>
```

## Dispatch

Layout XML rule; parent/child walk.

## Links

- Parent: [`roadmap/62-resource-cost-rules.md`](../../62-resource-cost-rules.md)
