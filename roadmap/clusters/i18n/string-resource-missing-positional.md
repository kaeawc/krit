# StringResourceMissingPositional

**Cluster:** [i18n](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

A variant string contains multiple `%s` — positional syntax
(`%1$s %2$s`) is required for languages that reorder.

## Triggers

```xml
<string name="greet">%s meets %s</string>
```

## Does not trigger

```xml
<string name="greet">%1$s meets %2$s</string>
```

## Dispatch

Resource rule on `<string>` values.

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
