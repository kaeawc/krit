# LayoutMinTouchTargetInButtonRow

**Cluster:** [accessibility](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`<Button>` in a `LinearLayout` with `android:layout_height="wrap_content"`
and no explicit `android:minHeight` ≥ 48dp.

## Triggers

```xml
<LinearLayout android:orientation="horizontal">
    <Button android:layout_width="wrap_content"
            android:layout_height="wrap_content"
            android:text="@string/ok" />
</LinearLayout>
```

## Does not trigger

```xml
<Button android:layout_width="wrap_content"
        android:layout_height="wrap_content"
        android:minHeight="48dp"
        android:text="@string/ok" />
```

## Dispatch

Layout XML rule.

## Links

- Parent: [`roadmap/52-accessibility-rules.md`](../../52-accessibility-rules.md)
