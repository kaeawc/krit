# Rules

Krit ships rules spanning Kotlin and Java source, style, complexity, null safety,
performance, Android manifests, resources, icons, and Gradle. Many are
detekt-compatible (same names and config keys) or Android Lint-compatible. Many
are auto-fixable.

## LogConditional Semantics

The opt-in Go and FIR checkers report only `android.util.Log.v/d/i/println` when the message
performs work; `w/e/wtf` are silent. FIR follows Android Lint 32.4.1 message semantics. Go uses a
deliberately narrower structural rule: after unwrapping parentheses, the message itself must be a
`call_expression`, or be a string template or `+` concatenation with a direct entry/operand that
unwraps to a `call_expression`. For example, `"x" + compute()`, `"${user.name()}"`, and
`obj.toString()` report. A call nested below another expression does not count: property reads such
as `getPackageInfo(...).versionName`, Elvis/conditional expressions, and `${compute().length}` are
silent in Go. It does not resolve message bindings, property declarations, or constness.

FIR follows Android Lint 32.4.1's resolved `LogDetector` message semantics: non-constant
concatenations/templates, calls, Kotlin qualified property accesses, and other expressions
perform work. Literals, simple references, and constant concatenations/templates are silent.
For example, with a non-const `val name`, `"load $name"` reports in FIR but is silent in Go;
resolved const references in templates/concatenations stay silent in FIR.
`println` uses argument index 2; the other methods use index 1. Go also checks positional values
inside named argument wrappers, but Kotlin rejects named arguments to the real Java `Log` methods;
the receiver-shape compile test records that boundary rather than using invalid code as FIR proof.

The nearest `if` decides guarding, regardless of flag spelling; only literal `true` does not guard.
A `when` entry guards if any of its case values is not literal `true`; `else` does not guard.
Enclosing source calls, classes/objects, member/top-level functions and properties, constructors,
accessors, and initializers stop the search. Local functions, standalone lambdas/anonymous functions,
declarations local to init blocks and secondary constructors, loops, and try/catch continue it.
A log used as a receiver, such as `if (flag) Log.d("T", compute()).also { }`, is guarded:
the selector call is not an enclosing call. A call taking that expression as an argument still
stops the search. Thus `if (flag) { run { Log.d("T", compute()) } }` reports,
but `if (flag) { val callback = { Log.d("T", compute()) } }` does not.

All Go verdicts are pinned by `// go-lines:` headers in the `LogConditional*.kt` goldens. Remaining
Go differences are documented inline: alias/static/star-import resolution and file-wide shadowing
by a nested `Log` class can cause missed log calls; call-free non-constant templates/concatenations,
qualified Kotlin properties and Java getters, Elvis expressions, not-null assertions, and
conditional expressions that FIR treats as work are silent in Go. Calls nested in those latter
expressions are also silent unless they are the message itself or a direct template/concatenation
entry. FIR treats Java field reads such as `getPackageInfo(...).versionName` as no work; Go also
stays silent because the outer message is not a call. FIR/Lint can fold literal local `val`s in
concatenations, but not templates; Go does not resolve constness in either case.
The historical corpus row in `docs/fir-validation.md` predates this semantic change and is not a fresh qualification.

Follow-ups, not implemented here: Java-source coverage and the scanner's `/benchmark/` test-path
substring classification. The existing Go-provided test-file skip remains unchanged in FIR.

## Categories

| Category           | What it catches |
|--------------------|-----------------|
| Style              | Magic numbers, wildcard imports, unused code, redundant syntax |
| Naming             | Class, function, variable, enum, package naming |
| Complexity         | Cyclomatic/cognitive complexity, long methods, nested depth |
| Potential Bugs     | Unsafe casts, null safety, unreachable code, swallowed exceptions |
| Empty Blocks       | Empty catch/if/else/for/while/when/try/finally/init |
| Exceptions         | Too-generic catches, printStackTrace, return from finally |
| Coroutines         | Hardcoded dispatchers, redundant suspend, GlobalScope |
| Performance        | Array primitives, spread operator, unnecessary instantiation |
| Dead Code          | Cross-file unused public API |
| Libraries          | Library-specific patterns |
| Android Manifest   | Security, permissions, exported components, backup config |
| Android Resources  | Missing translations, unused resources, density checks |
| Android Usability  | Accessibility, typography, button styles, text fields |
| Android Icons      | Launcher icon density, WebP conversion, adaptive icons |
| Android Gradle     | Dependency management, SDK versions, build configuration |
| Android Source     | Android-specific Kotlin/Java code patterns |

`krit --list-rules` prints the full list with IDs and default thresholds.
`krit --list-rules --format json` prints it as JSON (`{"rules": [...]}`, one
object per rule with `id`, `category`, `severity`, `defaultActive`,
`fixable`, `fixLevel`, `precision`, `maturity`, `description`, and
`docsUrl`) for editor
integrations and scripts; `--cwe` and `--maturity` filter it the same way.

## Rule reference

Every rule has a page with its options, defaults, detekt/Android Lint
mapping, and examples: see the [rule reference](rules/categories.md). Pages
live at `https://kaeawc.github.io/krit/rules/<category>/<rule-id-kebab>/`
(for example `rules/style/magic-number/`) and are generated by
`go run ./cmd/krit-docgen` (run it before `mkdocs build`; the docs workflow
does). The same URL is the `docsUrl` field of `krit --list-rules --format json`
and the SARIF `helpUri`.

## Compose-aware defaults

Krit recognizes common Compose patterns without extra config:

- `@Composable` functions skip PascalCase naming checks
- `@Preview` functions skip magic number checks
- Color hex literals and `.dp` / `.sp` values are recognized
- Ignore-by-annotation lists (`ignoreAnnotated`) take short names (`Composable`, not the FQN)

## Auto-fix safety levels

| Level     | What it changes | Example |
|-----------|-----------------|---------|
| Cosmetic  | Whitespace, formatting, redundant keywords | Remove `public`, fix trailing whitespace, reorder modifiers |
| Idiomatic | Equivalent Kotlin conventions | `check(x != null)` → `checkNotNull(x)`, `.filter{}.first()` → `.first{}` |
| Semantic  | Changes that can affect edge-case behavior | Remove unused code, `var` → `val`, `as` → `as?` |

`krit --fix .` applies cosmetic + idiomatic. `krit --fix --fix-level=semantic .` applies everything. Use `--dry-run` to preview.

## Suggested fixes

A rule that cannot reduce to a single safe rewrite may instead emit
**suggested fixes** — an ordered, user-selectable list. Autofix and
suggested fixes are mutually exclusive per rule, and the order is
rule-recommended. See [Suggested fixes](suggested-fixes.md) for the
full contract, an authoring example, JSON shape, and integration
guidance.

## Rule types (internals)

- **Node-dispatched rules** — register `NodeTypes` and receive matching flat AST nodes.
- **Line-pass rules** — declare `NeedsLinePass` and scan file lines in a single pass.
- **Project-scope rules** — declare capabilities such as cross-file, module,
  manifest, resource, Gradle, resolver, or oracle data and run after the needed
  indexes are available.
- **JVM-backed rules** — use source inference first where possible and can opt
  into FIR helper facts when source analysis is not enough.

See `internal/rules/` for implementations and `tests/fixtures/` for positive/negative examples.
