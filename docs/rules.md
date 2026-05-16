# Rules

Krit ships rules spanning Kotlin and Java source, style, complexity, null safety,
performance, Android manifests, resources, icons, and Gradle. Many are
detekt-compatible (same names and config keys) or Android Lint-compatible. Many
are auto-fixable.

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
  into Kotlin Analysis API/FIR helper facts when source analysis is not enough.

See `internal/rules/` for implementations and `tests/fixtures/` for positive/negative examples.
