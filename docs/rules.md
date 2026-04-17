# Rules

Krit ships **472 rules** — 230 detekt-compatible (227 core + 3 library), 181 Android Lint–compatible, plus extras for resources, icons, and Gradle. **142** are auto-fixable.

## Categories

| Category           | Rules | Auto-fix | What it catches |
|--------------------|------:|---------:|-----------------|
| Style              |    45 |       22 | Magic numbers, wildcard imports, unused code, redundant syntax |
| Naming             |    18 |        4 | Class, function, variable, enum, package naming |
| Complexity         |    12 |        0 | Cyclomatic/cognitive complexity, long methods, nested depth |
| Potential Bugs     |    28 |        8 | Unsafe casts, null safety, unreachable code, swallowed exceptions |
| Empty Blocks       |    14 |        6 | Empty catch/if/else/for/while/when/try/finally/init |
| Exceptions         |    10 |        3 | Too-generic catches, printStackTrace, return from finally |
| Coroutines         |     8 |        4 | Hardcoded dispatchers, redundant suspend, GlobalScope |
| Performance        |     6 |        2 | Array primitives, spread operator, unnecessary instantiation |
| Dead Code          |     6 |        6 | Cross-file unused public API |
| Libraries          |     3 |        1 | Library-specific patterns |
| Android Manifest   |    40 |       12 | Security, permissions, exported components, backup config |
| Android Resources  |    35 |       15 | Missing translations, unused resources, density checks |
| Android Usability  |    30 |       10 | Accessibility, typography, button styles, text fields |
| Android Icons      |    25 |       18 | Launcher icon density, WebP conversion, adaptive icons |
| Android Gradle     |    20 |        8 | Dependency management, SDK versions, build configuration |
| Android Source     |    31 |       12 | Android-specific Kotlin/Java code patterns |
| **Total**          | **472** | **142** | |

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
