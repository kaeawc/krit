# TranslatableMarkupMismatch

**Cluster:** [i18n](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## Catches

The default string uses `<b>foo</b>` markup; the variant uses
`**foo**` or plain text (or vice versa).

## Triggers

```xml
<!-- values/strings.xml --> <string name="emphasis">This is &lt;b&gt;bold&lt;/b&gt;</string>
<!-- values-fr/strings.xml --> <string name="emphasis">Ceci est **gras**</string>
```

## Does not trigger

```xml
<!-- values/strings.xml --> <string name="emphasis">This is &lt;b&gt;bold&lt;/b&gt;</string>
<!-- values-fr/strings.xml --> <string name="emphasis">Ceci est &lt;b&gt;gras&lt;/b&gt;</string>
```

## Dispatch

Cross-variant string-resource rule.

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
