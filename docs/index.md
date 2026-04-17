krit is a fast and accurate Kotlin static analysis tool.

It walks each Kotlin file's AST once, dispatches to rules via symbol-indexed routing, skips files using heuristics in the expensive type-analysis phase, and caches type information under content-addressable dep-closure fingerprints that survive across runs in a long-lived JVM daemon with AppCDS class-data-sharing. The goal is to use performance focused architecture to create the fastest possible feedback cycle.

---

Get started:

```bash
curl -fsSL https://raw.githubusercontent.com/kaeawc/krit/main/scripts/install.sh | bash
```
