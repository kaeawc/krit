# ZipSlipUnchecked

**Cluster:** [security/call-shape](README.md) · **Status:** planned · **Severity:** info · **Default:** inactive

## Catches

`ZipInputStream` / `ZipFile` iteration that calls `File(parent, entry.name)`
without a subsequent `canonicalPath.startsWith(...)` check on the same
variable.

## Triggers

```kotlin
ZipInputStream(stream).use { zis ->
    var entry = zis.nextEntry
    while (entry != null) {
        val out = File(destDir, entry.name)
        out.parentFile?.mkdirs()
        out.outputStream().use { zis.copyTo(it) }
        entry = zis.nextEntry
    }
}
```

## Does not trigger

```kotlin
val destCanonical = destDir.canonicalPath
ZipInputStream(stream).use { zis ->
    var entry = zis.nextEntry
    while (entry != null) {
        val out = File(destDir, entry.name)
        require(out.canonicalPath.startsWith(destCanonical + File.separator))
        // ... write
        entry = zis.nextEntry
    }
}
```

## Dispatch

`while_statement` / `for_statement` whose body contains a
`File(parent, entry.name)` constructor without a guarding
`canonicalPath.startsWith(...)` on the same output variable in the
same block.

## Links

- Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)
- Related: [`file-from-untrusted-path.md`](file-from-untrusted-path.md)
