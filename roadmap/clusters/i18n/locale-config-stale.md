# LocaleConfigStale

**Cluster:** [i18n](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`locales_config.xml` entries don't match the set of `values-XX/`
folders present in the project.

## Triggers

```xml
<!-- locales_config.xml lists en, fr, de -->
<!-- project has values/, values-fr/, values-de/, values-es/ -->
```

## Does not trigger

The config and the set of variant folders match exactly.

## Dispatch

Resource rule that enumerates `res/values-*/` folders and compares
with `locales_config.xml` contents.

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
- Related: [`locale-config-missing.md`](locale-config-missing.md)
