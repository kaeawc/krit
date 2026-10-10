# Kotlin syntax support in the structural parser

Krit's structural analysis parses Kotlin with the tree-sitter Kotlin grammar
vendored by `github.com/smacker/go-tree-sitter` (pinned in `go.mod`, 2024).
Upgrading the embedded Kotlin compiler for the FIR and KAA helpers (2.4.20)
doesn't change that grammar, so newer syntax can parse with recovery nodes.

When a construct doesn't parse, tree-sitter wraps the unknown tokens in an
ERROR or MISSING node. Krit drops only findings anchored inside those spans;
the rest of the file, including the enclosing declaration, is still analyzed.
FIR and KAA (`--fir`, the type oracle) use the real compiler and aren't
affected.

| Construct (Kotlin version) | Structural parse | Effect |
|---|---|---|
| Context receivers `context(Logger)` (1.6.20, deprecated) | clean | none |
| Named context parameters `context(logger: Logger)`, `context(_: Logger)`, on functions and properties (2.2) | `: Type` is an ERROR span | the function/property is indexed; the context parameter's type isn't recorded as a reference |
| Context function types `context(Logger) () -> Unit` (2.2) | ERROR spans | the property is indexed with a truncated type |
| Explicit backing fields `val x: List<Int>` + `field = mutableListOf()` (2.4) | `field = …` recovers with a MISSING `val` | krit re-types that node as `explicit_backing_field`, so it isn't reported as a property named `field` |
| `@all:` meta-target (2.4) | `:Annotation` is an ERROR span | the annotation is seen as `@all`; the real annotation isn't visible to structural rules |
| `@param:`, `@field:` and other use-site targets | clean | none |
| `when` guards `is Int if x > 0 ->` (2.2) | the guard is an ERROR span | the `when` entry is indexed without its guard |
| Multi-dollar interpolation `$$"…"` (2.2) | the whole declaration is ERROR | findings inside that declaration are dropped |
| Nested type aliases (2.3) | clean | none |

`internal/scanner/kotlin24_syntax_test.go` pins each row: the exact error
spans and the declarations that must still be indexed. When the grammar is
upgraded and a construct starts parsing cleanly, update that test and this
table.
