# ClipboardOnSensitiveInputType

**Cluster:** [privacy](README.md) · **Status:** shipped · **Severity:** warning · **Default:** inactive

## Catches

`clipboardManager.setPrimaryClip(...)` whose content flows from an
`EditText` whose XML `inputType` is a password variant.

## Triggers

```xml
<EditText android:id="@+id/pwd" android:inputType="textPassword" />
```
```kotlin
clipboardManager.setPrimaryClip(ClipData.newPlainText("", pwd.text))
```

## Does not trigger

Same code but the source `EditText` isn't a password field.

## Dispatch

Cross-reference XML `inputType` of an id used in a Kotlin
`setPrimaryClip` call via the resource index.

## Links

- Parent: [`roadmap/60-privacy-data-handling-rules.md`](../../60-privacy-data-handling-rules.md)
