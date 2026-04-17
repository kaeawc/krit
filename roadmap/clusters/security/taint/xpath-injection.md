# XpathInjection

**Cluster:** [security/taint](README.md) · **Status:** deferred

## Catches

Untrusted source reaching `XPath.compile(...)` or `evaluate(...)`.

## Shape

```kotlin
val name = request.queryParameter("name")
val xp = XPathFactory.newInstance().newXPath()
xp.evaluate("//user[@name='$name']", doc)
```

## Links

- Parent: [`roadmap/51-security-rules-taint.md`](../../../51-security-rules-taint.md)
