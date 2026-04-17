# DeepLinkMissingAutoVerify

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

Manifest `<intent-filter>` with a `<data>` element containing
`android:scheme="https"` but no `android:autoVerify="true"` on the
enclosing intent-filter. Without auto-verify, the user gets a
disambiguator every time a link is opened.

## Example — triggers

```xml
<intent-filter>
    <action android:name="android.intent.action.VIEW" />
    <category android:name="android.intent.category.DEFAULT" />
    <category android:name="android.intent.category.BROWSABLE" />
    <data android:scheme="https" android:host="app.example.com" />
</intent-filter>
```

## Example — does not trigger

```xml
<intent-filter android:autoVerify="true">
    <action android:name="android.intent.action.VIEW" />
    <category android:name="android.intent.category.DEFAULT" />
    <category android:name="android.intent.category.BROWSABLE" />
    <data android:scheme="https" android:host="app.example.com" />
</intent-filter>
```

## Implementation notes

- Manifest rule; reuses `internal/rules/android_manifest_security.go`.
- Walks `<application>` → `<activity>` → `<intent-filter>` and
  checks for `data/@scheme="https"` co-occurring with a missing
  `android:autoVerify` on the filter element.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
