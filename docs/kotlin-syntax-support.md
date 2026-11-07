# Kotlin syntax support in the structural parser

Krit's structural analysis parses Kotlin with the tree-sitter Kotlin grammar
vendored by `github.com/smacker/go-tree-sitter` (pinned in `go.mod`, 2024).
Upgrading the embedded Kotlin compiler for the FIR helper (2.4.21)
doesn't change that grammar, so newer syntax can parse with recovery nodes.

When a construct doesn't parse, tree-sitter wraps the unknown tokens in an
ERROR or MISSING node. Krit drops only findings anchored inside those spans;
the rest of the file, including the enclosing declaration, is still analyzed.
FIR (`--fir` and the type oracle) uses the real compiler and isn't
affected.

| Construct (Kotlin version) | Structural parse | Effect |
|---|---|---|
| Context receivers `context(Logger)` (1.6.20, deprecated) | clean | none |
| Named context parameters `context(logger: Logger)`, `context(_: Logger)`, on functions, properties and class members (2.2) | clean after a pre-parse rewrite (see below) | the declaration covers the `context(...)` clause; named parameters are `parameter` children of a `context_parameters` node, which the type resolver puts in scope. Unnamed `context(Logger)` entries have no nodes |
| Context function types `context(Logger) () -> Unit` (2.2) | ERROR spans | the property is indexed with a truncated type |
| Explicit backing fields `val x: List<Int>` + `field = mutableListOf()` (2.4) | `field = …` recovers with a MISSING `val` | krit re-types that node as `explicit_backing_field`, so it isn't reported as a property named `field`. A top-level `field = …` still parses as an assignment statement, and the single-line typed form `val x: T field: F = …` recovers as one property (its declared type is read from the ERROR node). `ExplicitBackingFieldCandidate` suggests the rewrite and only autofixes class members for that reason |
| `@all:` meta-target (2.4) | `:Annotation` is an ERROR span | the annotation is seen as `@all`; the real annotation isn't visible to structural rules |
| `@param:`, `@field:` and other use-site targets | clean | none |
| `when` guards `is Int if x > 0 ->` (2.2) | clean after a rewrite: `if` reads as a second condition | rules see the guard as an extra `when_condition` of the entry |
| Multi-dollar interpolation `$$"…"` (2.2) | clean after a rewrite that neutralizes literal `$` bytes | none |
| Nested type aliases (2.3) | clean | none |

`internal/scanner/kotlin24_syntax_test.go` pins each row: the exact error
spans and the declarations that must still be indexed. When the grammar is
upgraded and a construct starts parsing cleanly, update that test and this
table.

## Pre-parse rewrites

`ParseKotlinFlat` (`internal/scanner/kotlin_rewrite*.go`) rewrites tokens the
vendored grammar cannot parse before it parses. Every rewrite keeps the byte
length, only touches code (a lexer skips strings, comments and backtick names),
and rows and columns are recomputed from the original source, so finding
offsets are unchanged. Node text is always read from the original source.
Where a rewrite hides tokens, `repairTree` restores nodes afterwards.

| Construct | Rewrite | Tree difference |
|---|---|---|
| `fun interface` | `fun` is blanked, the declaration is widened back over it | no `fun` token node |
| `a..<b`, `0b101` | `..<` parses as `..`, `0b` as `0x` | `0b101` is a `hex_literal` |
| `context(...)` clause | blanked, declaration widened | named parameters are rebuilt as a `context_parameters` child |
| `is X if cond ->` | `if` becomes `,` | guard is a second `when_condition` |
| `$$"…"` | `$$` prefix blanked, literal `$` bytes neutralized | none |
| `(val a, val b) = x`, `[val a, val b]`, `val [a, b]`, `[a, b] ->` | rewritten to `val (a, b)` / `(a, b)` | a `multi_variable_declaration` |
| `: Iface by expr { body }` on classes and objects | `by expr` is blanked, then rebuilt as `explicit_delegation` | none |
| `: @Ann Base` supertypes | annotation blanked | annotation has no nodes |
| `f(x).y = v`, `a()[i] = v`, `(x as T).p = v` | target wrapped in parentheses, removed again | none |
| line-leading `as`, `where` | line break replaced by a space | none |
| line-leading `+` or `-` outside parentheses and brackets | `;` ends the previous line, as Kotlin does (`a` / `+ b` is two statements) | none; not applied after `return`, `break`, `continue`, an `if`/`while`/`for` head or a line ending in an operator keyword |
| line-leading `in`, `!in`, `else ->`, `.5f`, `!x ->` | `;` ends the previous line | none; node ends are trimmed before the `;` |
| trailing comma in `<A, B,>`, `[i,]`, `(A, B,) -> R` | comma blanked | none |
| `Type?::m`, `Type<A>::m`, `A.B.() -> R`, `T?.(X) -> R` | `?` / type arguments blanked, qualifier dots read as `_` | type arguments and nullability not in the tree |
| `{ (a, b): T -> }` | type blanked | type not in the tree |
| identifiers that spell modifiers (`open.x()`, `{ annotation -> }`) | first letter upper-cased | none |
| non-ASCII letters in unquoted identifiers | read as `a` | none |

## Rule behavior on Kotlin 2.x constructs

`internal/rules/testdata/kotlin2x_compat/` holds synthetic sources, one per construct, with the findings each
rule must report marked inline (`TestKotlin2xCompatFixtures`). `tests/parity/oracle_kotlin2x_smartcast_test.go`
holds the oracle to the K2 compiler on smart casts the structural view cannot follow.

| Construct | Rules | Behavior |
|---|---|---|
| Explicit backing fields | `UnusedPrivateProperty`, `StateFlowMutableLeak`, naming rules | the `field = …` initializer is not a property; a public `StateFlow` backed by a private `MutableStateFlow` is not a leak |
| `when` guards | `NoElseInWhenSealed`, `NonExhaustiveWhen`, `ElseCaseInsteadOfExhaustiveWhen`, `UseIfInsteadOfWhen` | a guarded branch covers nothing, so it neither completes an exhaustive `when` nor makes an `else` redundant; an entry with a guard has two conditions, which `UseIfInsteadOfWhen` skips like a comma list |
| Context parameters | `UnusedParameter`, `LongParameterList`, `UnusedImport` | context parameters are supplied implicitly, so an unused name is not reported; they are not value parameters, so `LongParameterList` does not count them (detekt's PSI has no node for them either); their types count as import uses |
| `data object`, `value class` | `ClassNaming`, `UseDataClass`, `DataClassShouldBeImmutable` | handled like classes and objects |
| Non-local `break`/`continue` | `UnconditionalJumpStatementInLoop`, `LoopWithTooManyJumpStatements` | a jump inside a lambda is not a direct statement of the loop body; `LoopWithTooManyJumpStatements` counts it |
| `$$"…"` strings | `UnusedPrivateProperty`, `UnusedImport`, `StringShouldBeRawString` | only `$$` interpolates, so `$name` is text and `$${name}` is a use |
| Name-based destructuring `(val a, val b) = x` | `NoNameShadowing`, `LambdaParameterNaming`, `DestructuringDeclarationWithTooManyEntries` | positional entries in declarations and loops can shadow; name-based and lambda entries are not reported; destructured lambda parameters are naming-checked |
| K2 smart casts | `UnnecessarySafeCall`, `UnnecessaryNotNullOperator`, `NullableToStringCall`, `CastNullableToNonNullableType` | the verdict is the compiler's (oracle diagnostics and smart-cast facts), so boolean flags, `||` exits, contracts and stable properties are followed |
| `kotlin.concurrent.atomics` | `MutableStateInObject` | `AtomicInt`, `AtomicReference` and the atomic arrays count as thread-safe like their `java.util.concurrent.atomic` counterparts |
| Enum `entries` | `ElseCaseInsteadOfExhaustiveWhen`, `EnumNaming` | same results as `values()` |

## Known limits

- Keywords inside a rewrite's blanked text (annotations on supertypes,
  context clauses, `by` argument names other than the delegate expression) are
  invisible to structural rules.
- A line comment between a line and a leading `as` or `in` line stops the join;
  that statement keeps recovery nodes.
- Soft keyword names in positions other than those listed, assignments whose
  target contains a lambda in the middle of a longer chain, `.interpolator`
  style assignments on a line that starts with `.`, and `@Annotation` runs
  that follow unusual shapes can still produce ERROR nodes. Findings anchored
  inside an ERROR/MISSING span are dropped by `FilterColumnsByErrorRegions`.
- `internal/scanner/misparse_corpus_test.go` measures a source tree
  (`KRIT_MISPARSE_CORPUS=<root>`, `KRIT_MISPARSE_LIST=<file>`).

## Parity with FIR

The Kotlin 2.4 constructs that the grammar cannot parse (named context
parameters, explicit backing fields, `@all:`) are compared against the
compilers in `tests/parity`, under explicit language modes (the Kotlin settings
from `docs/kotlin-compiler-settings.md`) rather than the embedded compiler's
defaults:

- `oracle_kotlin24_parity_test.go` runs fixtures for named context parameters
  (declarations, use sites, shadowing), explicit backing fields against the
  exposed property type, and `@all:` / default annotation-use-site targets
  through the krit-fir oracle. It asserts the property and function
  annotations, the types at declaration and use sites, and that the facts stay
  the same when the analysis is repeated (cold daemon, served from the cache,
  warm daemon).
- `fir_kotlin24_parity_test.go` and `tests/fixtures/{positive,negative}/kotlin24/`
  hold krit-fir to the Go rules on Kotlin 2.4 sources under explicit language
  modes, and prove that the language mode reaches the checker (2.3 without the
  opt-in flags rejects the three features). `TestFirFixtureParity` picks the
  `kotlin24` fixtures up as well.

The oracle reports generic return types with their arguments
(`kotlin.collections.List<kotlin.Int>`), annotation classes with kind
`annotation`, and expression facts for resolved calls and the type of plain
references. Context parameters are not value parameters (`params`
holds only the declared ones), the exposed type of a property with an explicit
backing field is the declared one, and a property annotation's target follows
the language mode. A constructor `val` annotation with no use-site target lands
on the property under the 2.4 default (and `-Xannotation-default-target=param-property`)
but only on the parameter under language 2.3 and below (or
`first-only`), so the structural view of such an annotation never says where it
applies. `@all:` applies to the property in every mode where the feature is
enabled; below 2.4 that needs `-Xannotation-target-all` (applied by krit since
#785), otherwise the file does not compile.
