# StringSpanInContentDescription

**Cluster:** [accessibility](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`<string>` resource referenced from a `contentDescription` that
contains HTML markup (`<b>`, `<i>`, `<br>`). TalkBack reads raw tags.

## Triggers

```xml
<string name="image_desc">A &lt;b&gt;bold&lt;/b&gt; cat</string>
```

Used as `android:contentDescription="@string/image_desc"`.

## Does not trigger

```xml
<string name="image_desc">A cat</string>
```

## Dispatch

Resource rule; cross-references strings used from `contentDescription`
attributes elsewhere in the layout set.

## Links

- Parent: [`roadmap/52-accessibility-rules.md`](../../52-accessibility-rules.md)
