# Kotlin Custom Rules SDK — Implementation Prompt

This document is an implementer-facing prompt. It captures the design
explored on branch `claude/explore-rule-systems-EhfE6` for letting Kotlin
developers author custom Krit rules in idiomatic Kotlin, surface findings
in JetBrains IDEs, and avoid forking or recompiling the Go binary. It is
intended to be handed to an engineer (or agent) tasked with landing the
work in tractable phases.

## 1. Goal

Allow downstream teams to author custom rules in Kotlin (with optional
Kotlin Analysis API access) and surface findings everywhere Krit already
surfaces them — CLI, LSP, MCP, SARIF, baselines — plus optionally in
Android Studio with zero per-developer setup.

Non-goals: rewriting Krit in Kotlin, replacing the Go analyzer core,
moving built-in rules off tree-sitter.

## 2. Scope

In scope:

- Public Maven-Central SDK that defines the Kotlin rule contract.
- A JVM Kotlin "rules sidecar" that extends the existing oracle daemon
  in `internal/oracle/`.
- Gradle plugin wiring (`krit-gradle-plugin`) so a project can declare
  custom-rule modules.
- Discovery and dispatch integration with Krit's Go pipeline.
- Documentation, versioning, and trust model.
- Optional: FIR compiler-plugin relay that republishes Krit findings as
  IDE diagnostics.

Out of scope (for this prompt):

- A JetBrains Marketplace plugin. Revisit only after LSP + FIR relay
  ship and a concrete UX requirement appears that LSP cannot satisfy.
- Kotlin/Native plugins. Discarded: Analysis API is JVM-only, K/N gives
  no compiler-integration advantage, and contributor familiarity is
  better served by the JVM sidecar.
- WASM rule sandbox. Tracked separately under `docs/external-rules.md`.

## 3. Architecture

```
+-----------------+       +-----------------------+
|  Krit (Go core) | <-->  |  Oracle + Rules JVM   |
|  tree-sitter,   |  RPC  |  daemon (resident):   |
|  dispatcher,    |       |    - Analysis API     |
|  output, fix    |       |    - Plugin classload |
+-----------------+       |    - @KritRule scan   |
        |                 +-----------------------+
        |                            ^
        |                            |
        v                            |
+-----------------+        +-------------------+
|  LSP / MCP /    |        |  Project Gradle:  |
|  CLI / SARIF    |        |  customRules(...) |
+-----------------+        +-------------------+
        |
        v (optional)
+---------------------------+
|  FIR plugin relay         |
|  (reads Krit findings,    |
|   reports diagnostics)    |
+---------------------------+
```

The Go core remains the source of truth. The JVM daemon already exists
for Analysis-API oracle facts (`internal/oracle/`); custom rules
classload into the same JVM and reuse its parse cache.

## 4. Published artifacts

All under coordinate group `dev.krit` on Maven Central.

| Artifact                       | Purpose                                              | Stability   |
|--------------------------------|------------------------------------------------------|-------------|
| `krit-rule-api`                | `@KritRule`, `KritRule`, `RuleContext`, `Finding`, `Fix` | **Stable** — semver |
| `krit-analysis`                | Analysis-API helpers, matchers, common patterns      | Stable      |
| `krit-test`                    | `KritRuleTester` for unit testing custom rules       | Stable      |
| `krit-gradle-plugin`           | Existing; gains `krit { customRules(...) }` DSL      | Stable      |
| `krit-fir-plugin` *(optional)* | FIR compiler plugin that relays findings to the IDE  | Experimental |

Splitting `api` from `analysis` keeps lexical-only rules off
`kotlin-compiler-embeddable`. Plugin authors depend `compileOnly` on
both — the daemon already has them on its classpath, and `compileOnly`
prevents classloader conflicts.

## 5. Rule authoring contract

```kotlin
@KritRule(
  id = "MyCo/AvoidLegacyHttp",
  category = "correctness",
  severity = Severity.WARNING,
  languages = [Language.KOTLIN],
  needs = [Capability.NeedsResolver],
  maturity = Maturity.EXPERIMENTAL,
)
class AvoidLegacyHttp : KritRule {
  override fun check(file: KritFile, ctx: RuleContext): List<Finding> {
    // file exposes a Krit-flavored façade over PSI; raw PSI is available
    // via file.ktFile for power users.
  }
}
```

Decision: ship a **thin Krit-flavored façade** (`KritFile`, `KritCall`,
`KritReceiver`) over PSI, with `ktFile`/`psiElement` escape hatches.
Façade isolates rule authors from `kotlin-compiler-embeddable` PSI
churn between Kotlin versions; escape hatches keep power users
unblocked.

Discovery uses `ServiceLoader` with a KSP processor in `krit-rule-api`
that generates `META-INF/services/dev.krit.api.KritRule` from
`@KritRule` annotations. No reflection scan at runtime.

## 6. Where rules live in a consumer repo

Example: a typical Android multi-module repo.

```
android-build/
├── settings.gradle.kts
├── gradle/libs.versions.toml
├── build-logic/
│   └── krit-rules/
│       ├── build.gradle.kts
│       └── src/
│           ├── main/kotlin/com/kaeawc/krit/
│           │   ├── AvoidLegacyHttp.kt
│           │   └── EnforceDispatchersIo.kt
│           └── test/kotlin/com/kaeawc/krit/
│               └── AvoidLegacyHttpTest.kt
└── app/, core/, ...
```

`build-logic/krit-rules/build.gradle.kts`:

```kotlin
plugins {
  alias(libs.plugins.kotlin.jvm)
  alias(libs.plugins.ksp)
}

dependencies {
  compileOnly(libs.krit.rule.api)
  compileOnly(libs.krit.analysis)
  ksp(libs.krit.rule.api.ksp)
  testImplementation(libs.krit.test)
}
```

Root `build.gradle.kts`:

```kotlin
krit {
  customRules(project(":build-logic:krit-rules"))
}
```

The Gradle plugin builds the jar, hands its path to Krit at invocation
time. No `~/.krit/plugins/` shuffling.

## 7. Wire protocol

Extend the existing oracle JSON-RPC with two verbs:

- `listPlugins(jars: string[]) -> [{ ruleId, languages, needs, severity,
  maturity, sdkVersion, ... }]`
  Classloads jars, scans `META-INF/services/dev.krit.api.KritRule`,
  reports descriptors back to the Go dispatcher.
- `analyzeFile(path, source, ruleIds[]) -> { findings: [...] }`
  Parses the file once (reusing the parse cache), runs all requested
  custom rules against the PSI tree, returns findings.

Per-file batching, not per-node. The daemon already keeps files
resident; this verb piggybacks on `WorkspaceState`'s parse cache.

## 8. Dispatch flow

1. Krit Go pipeline walks files. Built-in rules run on tree-sitter AST
   exactly as today.
2. For any file matching at least one enabled custom-rule capability,
   the dispatcher calls `analyzeFile` on the daemon with the relevant
   rule IDs.
3. Findings merge into the standard pipeline: suppression
   (`@Suppress("MyCo/AvoidLegacyHttp")`), baselines, maturity gating,
   output, autofix.

Plugin rule IDs are first-class everywhere `--list-rules`, JSON, SARIF,
Checkstyle, LSP, MCP already show built-ins.

## 9. Optional: FIR plugin relay

`krit-fir-plugin` runs inside `kotlinc` and Android Studio K2 mode. It
does **not** re-analyze. It either:

- reads Krit's cached findings (`.krit/snapshots/.../findings.json`)
  for the file being compiled, or
- queries the Krit daemon over a local socket per-compiled-file.

Findings map to FIR `KtDiagnosticReporter` calls with the right source
ranges. Quick-fixes route to Krit's existing autofix engine. This gives
Kotlin devs in-IDE squiggles with zero LSP setup, without forking rule
logic across two analyzers.

Hard rule: **the FIR plugin never re-implements rules.** If that
becomes tempting, write a design doc first.

## 10. Phased delivery

Each phase is independently shippable and reviewable. A phase only
begins after the prior phase's evaluation gates pass.

### Phase 0 — Spike (1 PR)

- Stand up a tiny `dev.krit:krit-rule-api` skeleton (interfaces only).
- Classload a single hardcoded plugin jar in the oracle daemon.
- Run one trivial rule from Go, return one finding.
- No Gradle wiring, no Maven publish yet.

Exit gate: end-to-end finding from a Kotlin-authored rule appears in
`krit scan` JSON output.

### Phase 1 — SDK + daemon plumbing

- Finalize `krit-rule-api` shape (`KritRule`, `RuleContext`, façade
  types, `Finding`, `Fix`, `Capability`, `Maturity`, `Severity`).
- KSP processor for `META-INF/services` generation.
- Daemon verbs `listPlugins` / `analyzeFile`.
- Go dispatcher integration including suppression, baselines, maturity.
- `krit-test` harness with `KritRuleTester`.
- Publish to Maven Central staging.

Exit gates:

- 10 sample rules of varying capability tiers pass tests via
  `KritRuleTester` and via `krit scan`.
- Cold-daemon-start adds <500ms vs. baseline on a 1-file scan.
- Warm `analyzeFile` adds <50ms per file when no custom rule needs
  Analysis API.
- ABI smoke test passes: load a plugin compiled against API v0.x.0 on
  daemon vN+1 and assert clean error rather than crash.

### Phase 2 — Gradle wiring

- `krit { customRules(project(...)) }` DSL.
- Wire jar paths into Krit's CLI invocation transparently.
- Doc updates in `docs/external-rules.md` linking to this plan.
- Sample project under `playground/` exercising a custom rule.

Exit gates:

- `./gradlew :app:kritCheck` on a sample multi-module repo runs both
  built-in and custom rules.
- Custom rules participate in baselines and suppression in CI.

### Phase 3 — FIR relay (optional, gated)

- `krit-fir-plugin` Gradle plugin reading the project's snapshot
  findings directory.
- Map findings to `KtDiagnosticReporter` with correct ranges.
- Quick-fix routing to Krit's fixer for `FixCosmetic` and
  `FixIdiomatic` rules only (defer `FixSemantic`).

Exit gates:

- Live squiggles in Android Studio K2 mode without LSP installed.
- No measurable slowdown to `kotlinc` cold compile.
- Compatible with at least the two most recent Kotlin minor versions.

### Phase 4 — Marketplace plugin (deferred)

Do **not** start. Re-evaluate after Phase 3 only if a concrete UX
requirement (Inspect-Code batch, rich settings panel) shows up that
LSP cannot satisfy.

## 11. Evaluation criteria for landing

Every PR in Phases 1–3 must clear all of these to merge.

### Correctness

- All built-in rule tests still pass (`go test ./... -count=1`).
- `make integration` passes including the new sample project.
- Custom rule findings are byte-identical between fresh and
  warm-cache daemon runs (determinism is non-negotiable per `CLAUDE.md`).

### Performance budgets

| Scenario                              | Budget                              |
|---------------------------------------|-------------------------------------|
| Cold daemon start (no plugins)        | No regression vs. baseline          |
| Cold daemon start (1 plugin jar)      | <500ms additional                   |
| Warm `analyzeFile`, no Analysis API   | <50ms / file p95                    |
| Warm `analyzeFile`, with Analysis API | <200ms / file p95                   |
| `krit scan` whole-project, 100 plugin rules off | within 2% of no-plugins baseline |
| `krit scan` whole-project, 10 plugin rules on   | within 25% of no-plugins baseline |

Benchmarks live under `internal/cli/serve/analyze_project_bench_test.go`
and a new `internal/oracle/plugin_bench_test.go`.

### ABI / versioning

- `krit-rule-api` follows semver. Minor bumps preserve binary
  compatibility for plugin jars.
- Daemon reads `Krit-SDK-Version` from each plugin jar manifest and
  refuses incompatible loads with a clear, user-actionable error.
- A `compat-matrix` table ships in `docs/external-rules.md` and is
  updated in the same PR that ships each Krit release.
- Regression test loads three pinned plugin jars (oldest supported,
  midpoint, latest) on each release branch.

### Trust model

- Plugin jars run arbitrary JVM code. Default policy: load from
  `<project>/build-logic/**` and `<project>/.krit/plugins/**` only.
  Global `~/.krit/plugins/` requires `--allow-global-plugins`.
- Daemon logs SHA-256 of every loaded jar to `.krit/plugins-loaded.json`
  for audit.
- Plugins may not spawn subprocesses or open network sockets without an
  explicit capability declaration (`Capability.NeedsNetwork` does not
  exist in Phase 1; rejected at load if encountered).

### Test coverage

- Each new public API has unit tests in `krit-test`.
- Each new daemon verb has a Go-side integration test under
  `internal/cli/serve/`.
- Sample project under `playground/` has a smoke test in CI.
- Determinism test: run a 100-file project twice, assert identical
  findings ordering and content.

### Documentation

- `docs/external-rules.md` updated to mark in-process registration as
  one of two supported paths.
- `docs/plans/kotlin-custom-rules-sdk.md` (this file) updated as the
  plan evolves; archived once Phase 3 ships.
- A "Writing your first custom rule" guide under `docs/`.
- KDoc on every public symbol in `krit-rule-api`.

### Backwards compatibility

- No breaking changes to `pkg/extension`; the Go in-process path
  remains supported and tested.
- No breaking changes to existing daemon verbs.
- `--list-rules`, JSON, SARIF schemas: additive only; new fields are
  optional and consumers tolerate their absence.

### Operational

- `make integration` is the gate (per `CLAUDE.md`).
- `golangci-lint run ./...` clean.
- For JVM side: a `:krit-rule-api:check` Gradle task runs lint
  (`detekt` or `ktlint`) and tests; CI fails on regressions.
- Telemetry: daemon emits per-rule timing for plugin rules through the
  existing perf snapshot pipeline so users can profile their custom
  rule cost.

## 12. Risks and mitigations

| Risk                                                | Mitigation                                                                 |
|-----------------------------------------------------|----------------------------------------------------------------------------|
| PSI shape churn between Kotlin versions             | Façade types; raw-PSI escape hatch only on opt-in                          |
| Daemon JVM OOM under many plugin rules              | Per-jar classloader; budgeted heap; explicit unload verb                   |
| First-run cold start regresses scan latency         | Defer `listPlugins` to first file that needs it; cache descriptor manifest |
| Plugin rule false positives leak to CI              | `Maturity.Experimental` default; require `--experimental` to enable        |
| Plugin authors fork rule logic into FIR plugin      | FIR plugin must remain a relay; PR template asks for justification         |
| ABI breakage strands consumers                      | Pinned compat matrix; explicit error on mismatch; pre-merge regression jar |
| Two rule systems drift in semantics                 | One source of truth (Krit Go core); FIR/LSP/MCP all surface the same data  |

## 13. Open questions to resolve before Phase 1

These are blocking decisions for the engineer landing Phase 1. Do not
guess; raise in the PR or a discussion.

1. **Façade scope.** Which PSI types get a `Krit*` wrapper in v0.1?
   Minimum: `KritFile`, `KritCall`, `KritReceiver`, `KritImport`. Anything
   else?
2. **Capability surface.** Does Phase 1 expose `NeedsResolver` only, or
   also `NeedsCrossFile`, `NeedsModuleIndex`? Each one expands the RPC
   shape.
3. **Autofix shape.** `Fix` in Kotlin returns text edits or PSI
   transformations? Text edits are simpler and ktfmt-friendly.
4. **Findings transport.** JSON over stdio is the default. Should
   `analyzeFile` return findings inline, or write to a results file and
   return a path? Inline is simpler; consider streaming if files get
   pathological.
5. **Where does the Kotlin sample project live?** New module under
   `playground/`, or a separate repo? Tests on a separate repo are
   harder to keep green.
6. **Maven publishing.** Sonatype OSSRH or Central Portal? Decide once
   so the release script doesn't churn.

## 14. Acceptance summary

Phase 1 is "done" when:

- A Kotlin developer can write a `@KritRule` in a Gradle module,
  declare it via `krit { customRules(...) }`, and see findings in
  `krit scan` JSON output.
- All Section 11 evaluation criteria pass on CI.
- `docs/external-rules.md` documents the new path with a working
  example.
- One real custom rule replaces one currently-built-in rule in a
  staged feature flag, proving production parity.

Phase 3 is "done" when the same Kotlin developer sees that finding as a
live diagnostic in Android Studio K2 mode without installing an LSP or
marketplace plugin.
