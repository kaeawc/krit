# StringRepeatedInContentDescription

**Cluster:** [accessibility](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

A `contentDescription` that duplicates the visible text of a sibling
node — TalkBack reads both, causing stutter.

## Triggers

```xml
<LinearLayout>
    <TextView android:text="@string/submit" />
    <ImageView android:contentDescription="@string/submit" />
</LinearLayout>
```

## Does not trigger

```xml
<LinearLayout>
    <TextView android:text="@string/submit" />
    <ImageView android:contentDescription="@string/submit_icon_desc" />
</LinearLayout>
```

## Dispatch

Layout XML rule; sibling walk across the same parent element. Same
shape as the existing `DuplicateIncludedIds`.

## Links

- Parent: [`roadmap/52-accessibility-rules.md`](../../52-accessibility-rules.md)
