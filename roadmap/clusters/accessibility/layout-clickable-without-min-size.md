# LayoutClickableWithoutMinSize

**Cluster:** [accessibility](README.md) · **Status:** shipped · **Severity:** warning · **Default:** inactive

## Catches

`android:clickable="true"` on a view whose
`android:layout_height`/`layout_width` is `< 48dp`.

## Triggers

```xml
<ImageView
    android:layout_width="32dp"
    android:layout_height="32dp"
    android:clickable="true" />
```

## Does not trigger

```xml
<ImageView
    android:layout_width="48dp"
    android:layout_height="48dp"
    android:clickable="true" />
```

## Dispatch

XML layout rule; reuses the resource parsing pipeline in
`internal/rules/android_resource_*.go`.

## Links

- Parent: [`roadmap/52-accessibility-rules.md`](../../52-accessibility-rules.md)
