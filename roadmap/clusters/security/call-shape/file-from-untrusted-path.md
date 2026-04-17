# FileFromUntrustedPath

**Cluster:** [security/call-shape](README.md) · **Status:** shipped · **Severity:** info · **Default:** inactive

## Catches

`File(parent, child)` where `child` is a literal containing `..` or a
non-literal expression inside a function whose name contains
`upload`/`extract`/`unzip`/`download`.

## Triggers

```kotlin
fun extractEntry(zipDir: File, entryName: String) {
    val out = File(zipDir, entryName)
    out.writeBytes(data)
}
```

## Does not trigger

```kotlin
fun extractEntry(zipDir: File, entryName: String) {
    val out = File(zipDir, entryName)
    require(out.canonicalPath.startsWith(zipDir.canonicalPath + File.separator))
    out.writeBytes(data)
}
```

## Dispatch

`call_expression` on `File(...)` with two args, gated on the enclosing
function name pattern.

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
- Related: [`zip-slip-unchecked.md`](zip-slip-unchecked.md)
