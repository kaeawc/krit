# StringNotSelectable

**Cluster:** [accessibility](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`<TextView android:selectable="false">` on content that contains
URLs or phone numbers — breaks copy/paste for assistive tech users.

## Triggers

```xml
<TextView android:text="call 555-1234 for support"
          android:textIsSelectable="false" />
```

## Does not trigger

```xml
<TextView android:text="call 555-1234 for support"
          android:textIsSelectable="true" />
```

## Dispatch

Layout XML rule.

## Links

- Parent: [`roadmap/52-accessibility-rules.md`](../../52-accessibility-rules.md)
