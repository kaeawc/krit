# LayoutAutofillHintMismatch

**Cluster:** [accessibility](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`<EditText inputType="textEmailAddress">` without
`android:autofillHints="emailAddress"`.

## Triggers

```xml
<EditText android:inputType="textEmailAddress" />
```

## Does not trigger

```xml
<EditText android:inputType="textEmailAddress"
          android:autofillHints="emailAddress" />
```

## Dispatch

Layout XML rule with a small input-type → autofill-hint map.

## Links

- Parent: [`roadmap/52-accessibility-rules.md`](../../52-accessibility-rules.md)
