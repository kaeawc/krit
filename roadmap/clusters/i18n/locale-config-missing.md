# LocaleConfigMissing

**Cluster:** [i18n](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

App manifest has `android:localeConfig` attribute but no
`locales_config.xml` resource in `res/xml/`.

## Triggers

```xml
<application android:localeConfig="@xml/locales_config" ... />
```

When `res/xml/locales_config.xml` is missing.

## Does not trigger

Both the attribute and the resource file exist.

## Dispatch

Manifest rule that cross-references `res/xml/` via the resource index.

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
- Related: [`locale-config-stale.md`](locale-config-stale.md)
