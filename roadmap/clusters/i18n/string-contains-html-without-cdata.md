# StringContainsHtmlWithoutCDATA

**Cluster:** [i18n](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`<string>` whose value contains `<` or `>` that isn't wrapped in
`<![CDATA[...]]>` or entity-escaped.

## Triggers

```xml
<string name="html_msg">Click <a href="...">here</a></string>
```

## Does not trigger

```xml
<string name="html_msg"><![CDATA[Click <a href="...">here</a>]]></string>
```

## Dispatch

Resource rule on `<string>` text.

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
