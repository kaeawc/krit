# LayoutEditTextMissingImportance

**Cluster:** [accessibility](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`<EditText>` without `android:importantForAutofill` on API 26+.

## Triggers

```xml
<EditText android:layout_width="match_parent" android:layout_height="wrap_content" />
```

## Does not trigger

```xml
<EditText android:layout_width="match_parent"
          android:layout_height="wrap_content"
          android:importantForAutofill="yes" />
```

## Dispatch

Layout XML rule.

## Links

- Parent: [`roadmap/52-accessibility-rules.md`](../../52-accessibility-rules.md)
