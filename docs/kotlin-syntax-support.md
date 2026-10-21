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
| Named context parameters on class members (2.2) | one such member: the class body stays intact, with an error-flagged `property_declaration` for the `context(...)` list in front of the function. Two or more in one class: the class keeps no body and every member is left at the top level of the file | in the second case class-scoped rules cannot attribute the members to their class (FIR and KAA still see the full class) |
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

## Parity with FIR and KAA

The Kotlin 2.4 constructs that the grammar cannot parse (named context
parameters, explicit backing fields, `@all:`) are compared against the
compilers in `tests/parity`, under explicit language modes (the Kotlin settings
from `docs/kotlin-compiler-settings.md`) rather than the embedded compiler's
defaults:

- `oracle_kotlin24_parity_test.go` runs fixtures for named context parameters
  (declarations, use sites, shadowing), explicit backing fields against the
  exposed property type, and `@all:` / default annotation-use-site targets
  through both oracle backends. It asserts the property and function
  annotations, the types at declaration and use sites, and that the facts stay
  the same when the analysis is repeated (cold daemon, served from the cache,
  warm daemon). `TestOracleBackendParity` includes the same fixtures.
- `fir_kotlin24_parity_test.go` and `tests/fixtures/{positive,negative}/kotlin24/`
  hold krit-fir to the Go rules on Kotlin 2.4 sources under explicit language
  modes, and prove that the language mode reaches the checker (2.3 without the
  opt-in flags rejects the three features). `TestFirFixtureParity` picks the
  `kotlin24` fixtures up as well.

Intentional differences between the backends, which the tests tolerate:

| | KAA | FIR |
|---|---|---|
| Generic return type | head only: `kotlin.collections.List` | with arguments: `kotlin.collections.List<kotlin.Int>` |
| Annotation class kind | `class` | `annotation` |
| Expression facts | resolved calls only | resolved calls and the type of plain references |

Both backends agree on: context parameters are not value parameters (`params`
holds only the declared ones), the exposed type of a property with an explicit
backing field is the declared one, and a property annotation's target follows
the language mode. A constructor `val` annotation with no use-site target lands
on the property under the 2.4 default (and `-Xannotation-default-target=param-property`)
but only on the parameter under language 2.3 and below (or
`first-only`), so the structural view of such an annotation never says where it
applies. `@all:` applies to the property in every mode where the feature is
enabled; below 2.4 that needs `-Xannotation-target-all` (applied by krit since
#785), otherwise the file does not compile.
