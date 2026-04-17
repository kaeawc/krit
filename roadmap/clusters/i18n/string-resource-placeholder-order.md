# StringResourcePlaceholderOrder

**Cluster:** [i18n](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

Default string uses `%1$s %2$s`; variant reorders them without
positional syntax (losing the mapping).

## Triggers

```xml
<!-- values/strings.xml --> <string name="greet">%1$s, %2$s</string>
<!-- values-fr/strings.xml --> <string name="greet">%2$s, %1$s</string>
```

Flagged if the variant drops positional syntax, e.g. `%s, %s`.

## Does not trigger

Both variants consistently use positional or non-positional syntax.

## Dispatch

Cross-variant resource rule; reuses the existing
`StringFormatMatches` / `StringFormatInvalid` walker.

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
- Related: existing `StringFormatMatches`
