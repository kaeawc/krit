# FIR checkers cluster

Delivery vehicle for Kotlin FIR-based checkers inside krit. The
goal is to give krit a second rule-execution path — alongside
tree-sitter + oracle — that runs inside a JVM with full access to
the K2 frontend's FIR tree, symbol resolver, type inference, and
diagnostic reporter. FIR-aware rules can catch things that are
either impossible or heuristic at the tree-sitter layer: smart-cast
nullability, suspend-context propagation, inline-function body
analysis, generic type bounds, exhaustive-when on sealed hierarchies
resolved through compiled dependencies, and the long tail of rules
that currently live as approximations.

This cluster is **tool-mode**, not a rule set. It does not add new
rule-category concepts (those live in `roadmap/clusters/<category>/`).
It adds the *machinery* that lets a rule be written as a FIR checker
instead of a Go walker — after which any rule cluster can opt into
it one rule at a time.

## Motivation

The 2026-04-14/15 oracle optimization push (see
[`roadmap/20-kotlin-type-oracle.md`](../../20-kotlin-type-oracle.md))
made the `krit-types` JVM subprocess fast enough to be in the warm
loop on dev machines — kotlin/kotlin warm-0 is 5.67 s, Signal
warm-0 is 0.48 s. Cache, daemon, per-rule filter, poison markers,
cross-invocation reuse are all in place. That infrastructure was
built to return *types*, but it's equally capable of returning
*findings*. This cluster turns that observation into a concrete
delivery mechanism.

It also avoids re-implementing type inference at the rule layer.
Every time a tree-sitter rule has to ask "is this call suspend" or
"does this receiver resolve to a Flow" or "is this variable
smart-cast to non-null", it's re-deriving a fact the Kotlin
compiler already computed. FIR checkers let rules consume that
computation directly.

## Reference implementation: Metro

[ZacSweers/metro](https://github.com/ZacSweers/metro) ships a
production FIR-based compiler plugin (DI framework, not a linter,
but the plugin shape is the same). It is the single best source in
`~/github/` for the patterns we'll copy. The key files:

- [`compiler/.../fir/MetroFirExtensionRegistrar.kt`](../../../../../github/metro/compiler/src/main/kotlin/dev/zacsweers/metro/compiler/fir/MetroFirExtensionRegistrar.kt)
  — `FirExtensionRegistrar` subclass; shows how a plugin declares
  its checker extension alongside code-generation extensions.
- [`compiler/.../fir/MetroFirCheckers.kt`](../../../../../github/metro/compiler/src/main/kotlin/dev/zacsweers/metro/compiler/fir/MetroFirCheckers.kt)
  — `FirAdditionalCheckersExtension` with `DeclarationCheckers` +
  `ExpressionCheckers`; one `Set<FirClassChecker>` /
  `Set<FirFunctionCallChecker>` / etc per extension point.
- [`compiler/.../fir/checkers/InjectConstructorChecker.kt`](../../../../../github/metro/compiler/src/main/kotlin/dev/zacsweers/metro/compiler/fir/checkers/InjectConstructorChecker.kt)
  — A real checker. Stateless `object`, `FirClassChecker(MppCheckerKind.Common)`
  base, context parameters carry `CheckerContext` + `DiagnosticReporter`,
  reports via `reporter.reportOn(source, MetroDiagnostics.X)`.
- [`compiler/.../fir/MetroDiagnostics.kt`](../../../../../github/metro/compiler/src/main/kotlin/dev/zacsweers/metro/compiler/fir/MetroDiagnostics.kt)
  — Central diagnostic factory file. Every diagnostic is a
  `KtDiagnosticFactoryN` constant registered in an object.
- [`compiler-compat/`](../../../../../github/metro/compiler-compat/)
  — Version-shim module. One subproject per supported Kotlin
  version (`k2220/`, `k230/`, `k2320/`, `k240_beta1/`,
  `k240_dev_2124/`). Each subproject implements a `CompatContext`
  interface loaded via `ServiceLoader`. Shaded into the final
  plugin JAR because Kotlin native requires embedded deps
  ([KT-53477](https://youtrack.jetbrains.com/issue/KT-53477)).
- [`compiler-tests/`](../../../../../github/metro/compiler-tests/)
  — Uses JetBrains' compiler-test framework (`.kt` files under
  `src/test/data/diagnostic/` with golden `<!DIAGNOSTIC_NAME!>...<!>`
  annotations and `// RENDER_DIAGNOSTICS_FULL_TEXT` directives).
  `./gradlew :compiler-tests:generateTests` regenerates test
  methods from data files.

Metro is carrying the "experimental compiler plugin API" tax in
real life — they have five `compiler-compat` subprojects as of
2026-04 and add one per new Kotlin version. That tax is the single
most important thing to budget for in this cluster.

## Phase structure

Two tracks, B before C. Track B ships a usable FIR-checker runner
that is entirely owned by krit (krit as the CLI driver, no kotlinc
plugin). Track C takes the same checker source and *also* publishes
it as a kotlinc compiler plugin for users who want IDE + compile-time
integration.

### Track B — krit orchestrates a JVM runner

Krit is the user-facing front end. FIR checkers run inside a JVM
subprocess that krit launches and daemonizes, mirroring the shape
of `tools/krit-types/` and `internal/oracle/`. No Gradle plugin,
no kotlinc plugin JAR, no IDE integration. The user runs `krit
check` and gets findings from both the tree-sitter path and the
FIR path in one report.

| Step | Concept | Rough effort |
|---|---|---|
| B.1 | [JVM runner (`tools/krit-fir/`)](jvm-runner.md) — Kotlin subprocess, embedded kotlinc via Analysis API + `FirExtensionRegistrar`, stdio protocol | 1–2 weeks |
| B.2 | [Go integration](go-integration.md) — daemon reuse, content-hash cache for findings, `internal/firchecks/` client, per-rule filter | 1 week |
| B.3 | [Internal checker API](checker-api.md) — `KritChecker` interface, diagnostic declarations, test harness | 1 week |
| B.4 | [Pilot rule migration](pilot-rules.md) — 3–5 rules ported from Go to FIR with parity oracle tests | 1 week |

**Definition of done for Track B:** `krit check --fir` on
Signal-Android runs both passes, merges findings, and a pilot
FIR rule ships in the default catalog behind a flag. Cold and
warm timings are within 2× of the tree-sitter-only path.

### Track C — dual packaging as a kotlinc plugin

Once Track B is shipping and at least one pilot rule lives under
`tools/krit-fir/rules/`, package the same checker classes as a
standalone kotlinc compiler plugin. Users get IDE diagnostics +
compile-time errors via Gradle, with no krit CLI involvement.
Krit's CLI path continues to work — both distribution targets
share the same rule source.

| Step | Concept | Rough effort |
|---|---|---|
| C.1 | [Kotlinc plugin packaging](plugin-packaging.md) — shaded plugin JAR, `FirExtensionRegistrar` registration, `compiler-compat/` shim following Metro's layout, Gradle plugin portal publish, IDE integration via Kotlin External FIR Support | 2–3 weeks |

**Definition of done for Track C:** `id("dev.krit.fir") version "X"`
in a user's `build.gradle.kts` produces the same diagnostics as
`krit check --fir`, the plugin JAR is published to the Gradle
plugin portal, and it passes on at least two Kotlin versions
(current stable + current dev).

## Non-goals

- **FIR code generation.** We're not a DI framework. No
  `FirDeclarationGenerationExtension`, no `FirSupertypeGenerationExtension`,
  no IR lowering. Read-only checkers only.
- **Replacing the tree-sitter path.** The tree-sitter + oracle
  rule engine remains the default. FIR checkers are additive. A
  rule should only migrate to FIR if tree-sitter + oracle
  demonstrably cannot answer its question well.
- **Custom K2 IR transforms.** Out of scope. This is about
  diagnostics.
- **Competing with detekt's existing compiler plugin.** Detekt is
  explicitly [migrating away from its compiler plugin](https://github.com/detekt/detekt-compiler-plugin)
  toward the Analysis API. We're walking into the same swamp. The
  bet is that krit's Go-side orchestration (cache, daemon, fixes,
  SARIF, LSP, MCP) is the right home for FIR checker output
  regardless of whether the checkers run via `FirExtensionRegistrar`
  or the Analysis API — and Track C gives us the kotlinc-plugin
  distribution as a second, optional channel.

## Stability caveat

The FIR plugin API is
[**experimental**](https://github.com/JetBrains/kotlin/blob/master/docs/fir/fir-plugins.md).
No KEEP for stabilization exists as of Kotlin 2.3.0. Every
supported Kotlin version costs a compat subproject. Metro is
carrying ~5 and counting. Budget one week per supported version
per year of maintenance, forever. This is the main reason Track
B ships first — if the API taxes become unsustainable, Track B
still works as long as `krit-fir` can bundle its own kotlinc
(we control the version) and Track C is the part we'd pause.

## Interactions with existing roadmap items

- **Item 20 (Kotlin type oracle)** — `krit-fir` reuses the
  `internal/oracle/` daemon layer (PID routing, `Release`,
  timeout/watchdog, cross-invocation reuse). No new Go
  infrastructure for daemon lifecycle.
- **Item 21 (Daemon startup optimization)** — AppCDS / persistent
  daemon / CRaC apply equally to `krit-fir`. Same JVM flags, same
  startup budget.
- **Item 27 (Performance algorithms)** — the per-rule `OracleFilter`
  infrastructure is directly reusable for deciding which files a
  given FIR checker needs to see.
- **Item 15 (Fix quality)** — FIR checkers can emit `Fix` payloads
  in the same format as Go rules. Item 15's byte-mode infrastructure
  handles the application layer transparently.
- **Item 10 (Detekt coverage gaps)** — the "needs type inference"
  rules in the gap are the natural pilot targets for Track B.4.
- **Item 68 (flat-tree migration)** — orthogonal. FIR checkers
  don't touch the FlatNode path at all.
