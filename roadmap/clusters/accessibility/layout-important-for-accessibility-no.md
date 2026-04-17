# LayoutImportantForAccessibilityNo

**Cluster:** [accessibility](README.md) · **Status:** shipped · **Severity:** warning · **Default:** inactive

## Catches

`android:importantForAccessibility="no"` on a view that also has
`android:clickable="true"` or `android:focusable="true"` — hides a
user-interactive element from assistive tech.

## Triggers

```xml
<Button android:clickable="true" android:importantForAccessibility="no" />
```

## Does not trigger

```xml
<ImageView android:importantForAccessibility="no" />
```

## Dispatch

Layout XML rule.

## Links

- Parent: [`roadmap/52-accessibility-rules.md`](../../52-accessibility-rules.md)
