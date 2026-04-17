# XmlExternalEntity

**Cluster:** [security/syntactic](README.md) · **Status:** planned · **Severity:** warning · **Default:** active

## What it catches

`DocumentBuilderFactory.newInstance()` / `SAXParserFactory.newInstance()`
/ `XMLInputFactory.newInstance()` without a subsequent
`.setFeature("http://apache.org/xml/features/disallow-doctype-decl", true)`
in the same block.

## Example — triggers

```kotlin
val factory = DocumentBuilderFactory.newInstance()
val builder = factory.newDocumentBuilder()
val doc = builder.parse(input)
```

## Example — does not trigger

```kotlin
val factory = DocumentBuilderFactory.newInstance().apply {
    setFeature("http://apache.org/xml/features/disallow-doctype-decl", true)
    isXIncludeAware = false
    isExpandEntityReferences = false
}
```

## Implementation notes

- Dispatch: `call_expression` on `newInstance()` whose receiver is
  an XML factory class.
- Walk the sibling statements in the same block looking for the
  `setFeature` guard. Similar pattern to
  `isEarlyReturnGuarded` in `potentialbugs_nullsafety.go`.

## Links

- Parent: [`roadmap/49-security-rules-syntactic.md`](../../../49-security-rules-syntactic.md)
