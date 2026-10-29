krit is a fast and accurate static analysis tool for Kotlin, Java, and Android projects.

It uses tree-sitter for fast source parsing, dispatches rules through a checked-in registry, and adds an optional JVM-backed krit-fir (K2 FIR) helper process for checks that need compiler-grade facts. Cached parse, resource, cross-file, and oracle data keep repeated runs fast.

---

Get started:

```bash
go install github.com/kaeawc/krit/cmd/krit@latest
krit .
```
