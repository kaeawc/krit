# StringTrailingWhitespace

**Cluster:** [i18n](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`<string>` resource value with trailing whitespace that isn't
`translatable="false"` — significant in some locales and in
concatenated strings.

## Triggers

```xml
<string name="label">Label </string>
```

## Does not trigger

```xml
<string name="label">Label</string>
<string name="label_with_space" translatable="false">Label </string>
```

## Dispatch

Resource rule; regex on raw text content.

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
