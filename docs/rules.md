# Rules

Krit ships 400+ rules spanning Kotlin style, complexity, null safety, performance, and Android manifests / resources / icons / Gradle. Many are detekt-compatible (same names and config keys) or Android Lint–compatible. Many are auto-fixable.

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

## Rule types (internals)

- **DispatchRule** — receives specific AST node types. Most rules use this for precision and perf.
- **LineRule** — scans file lines in a single pass. Used for patterns that don't need AST structure.
- **AggregateRule** — collects nodes during the AST walk and finalizes after. Used for whole-file metrics.

See `internal/rules/` for implementations and `tests/fixtures/` for positive/negative examples.
