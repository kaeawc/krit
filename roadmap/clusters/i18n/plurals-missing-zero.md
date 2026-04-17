# PluralsMissingZero

**Cluster:** [i18n](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`<plurals>` in a `values-ar/` or `values-ru/` variant without a
`<item quantity="zero">`. CLDR specifies a zero form for these
languages.

## Triggers

```xml
<!-- values-ar/plurals.xml -->
<plurals name="item_count">
    <item quantity="one">عنصر واحد</item>
    <item quantity="other">%d عناصر</item>
</plurals>
```

## Does not trigger

```xml
<plurals name="item_count">
    <item quantity="zero">لا يوجد عناصر</item>
    <item quantity="one">عنصر واحد</item>
    <item quantity="two">عنصران</item>
    <item quantity="few">%d عناصر</item>
    <item quantity="many">%d عنصرًا</item>
    <item quantity="other">%d عنصر</item>
</plurals>
```

## Dispatch

Resource rule; gated on the parent directory matching a locale that
CLDR requires the zero form for.

## Links

- Parent: [`roadmap/53-i18n-l10n-rules.md`](../../53-i18n-l10n-rules.md)
