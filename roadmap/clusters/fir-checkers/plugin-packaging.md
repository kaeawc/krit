# C.1 — Kotlinc plugin packaging

**Cluster:** [fir-checkers](README.md) · **Status:** planned · **Track:** C · **Severity:** n/a (tool mode)

## Catches

Once Track B is shipping and at least one pilot rule lives under
`tools/krit-fir/src/main/kotlin/dev/krit/fir/rules/`, the same
checker classes can be repackaged as a standalone kotlinc
compiler plugin. Users add a Gradle plugin to their Kotlin build,
and FIR diagnostics appear in their normal compile output plus
the IntelliJ editor — no krit CLI involved. This is the second
distribution channel for the **same checker source code**.

The entire point of committing to Option A in
[jvm-runner.md](jvm-runner.md) was to make this concept a
packaging exercise instead of a rewrite.

## Shape

Three Gradle modules plus a Gradle plugin module:

```
tools/krit-fir/                      ← checker classes (already exists from Track B)
  src/main/kotlin/dev/krit/fir/
    rules/                           ← the FirClassChecker / FirFunctionCallChecker objects

tools/krit-fir-plugin/               ← new — kotlinc plugin jar
  src/main/kotlin/dev/krit/fir/plugin/
    KritFirCompilerPluginRegistrar.kt   ← CompilerPluginRegistrar
    KritFirExtensionRegistrar.kt        ← installs FirAdditionalCheckersExtension
    KritFirCommandLineProcessor.kt      ← CLI options (severity, rule enable/disable)
  resources/META-INF/services/
    org.jetbrains.kotlin.compiler.plugin.CompilerPluginRegistrar

tools/krit-fir-compat/               ← new — per-version shim module (see below)
  k2220/src/main/kotlin/...
  k230/src/main/kotlin/...
  k2320/src/main/kotlin/...
  k240/src/main/kotlin/...

tools/krit-fir-gradle-plugin/        ← new — Gradle plugin that wires the jar into builds
  src/main/kotlin/dev/krit/fir/gradle/
    KritFirGradlePlugin.kt
    KritFirExtension.kt
```

The checker classes under `tools/krit-fir/` are the **single
source of truth**. Track B's JVM runner consumes them via
embedded kotlinc. `tools/krit-fir-plugin/` consumes the exact
same classes and registers them with the real kotlinc at compile
time. Both packaging targets import `tools/krit-fir` as a regular
compile dependency.

## The compat shim (direct copy of Metro's layout)

Metro's `compiler-compat/` is the template. See
[`~/github/metro/compiler-compat/README.md`](../../../../../github/metro/compiler-compat/README.md).
The model:

1. **A single `CompatContext` interface** in the common source set
   defines every API surface that differs across Kotlin versions —
   e.g., `FirExpressionEvaluator.evaluate` signature shifts, new
   parameters on `FirAdditionalCheckersExtension`, diagnostic
   factory DSL changes.
2. **One subproject per supported Kotlin version** (`k2220/`,
   `k230/`, `k2320/`, `k240/`) implements `CompatContext` against
   that version's exact kotlinc API.
3. **`ServiceLoader`** picks the right implementation at runtime
   based on the kotlinc version the plugin JAR was loaded into.
4. **Shaded into the plugin JAR** via the Gradle shadow plugin,
   because Kotlin native requires embedded deps
   ([KT-53477](https://youtrack.jetbrains.com/issue/KT-53477)).

Metro's `compiler-compat/` has five subprojects as of 2026-04 and
adds one per Kotlin dev build their IDE plugin needs to support.
The maintenance cost is **~1 week per Kotlin version per year**.
Budget this. If a Kotlin version release breaks the plugin API in
a way the shim can't absorb cleanly, that version is simply not
supported for one release until someone writes the new subproject.

Metro ships several scripts in `compiler-compat/` that we should
copy wholesale:

- [`extract-kotlin-compiler-txt.sh`](../../../../../github/metro/compiler-compat/extract-kotlin-compiler-txt.sh)
  — reads the bundled kotlinc version out of an Android Studio /
  IntelliJ install
- [`resolve-ij-kotlin-version.sh`](../../../../../github/metro/compiler-compat/resolve-ij-kotlin-version.sh)
  — traces an `-ij`-suffixed dev build back to the Kotlin master
  commit it branched from (uses git ancestry in `JetBrains/kotlin`,
  binary searches dev tags)
- [`fetch-all-ide-kotlin-versions.py`](../../../../../github/metro/compiler-compat/fetch-all-ide-kotlin-versions.py)
  — enumerates recent IDE releases and emits a
  `BUILT_IN_COMPILER_VERSION_ALIASES` map

These scripts are how Metro keeps its IDE-version → kotlinc-version
mapping up to date without having to check every release manually.
They're the main reason Metro's compat story is livable instead of
constant firefighting.

## Gradle plugin

Users install via:

```kotlin
plugins {
  id("dev.krit.fir") version "0.1.0"
}

kritFir {
  enabledRules.set(listOf("FlowCollectInOnCreate", "ComposeRememberWithoutKey"))
  severity("FlowCollectInOnCreate", Severity.Error)
}
```

The Gradle plugin:

1. Adds `tools/krit-fir-plugin.jar` to the `kotlinCompilerPluginClasspath`
   configuration on every Kotlin compilation task
2. Passes rule enable/disable + severity as plugin CLI options
   via `KritFirCommandLineProcessor`
3. Supports JVM, Android, KMP targets (Metro's gradle plugin
   shows how — it iterates over `project.kotlinExtension.targets`)

Publish to the Gradle plugin portal as `dev.krit.fir`. Follow the
same release process as `krit-gradle-plugin`
([item 01](../../01-gradle-plugin.md)).

## IDE integration

Two paths:

**Path 1 — Kotlin External FIR Support plugin (automatic).** The
community [Kotlin External FIR Support plugin](https://plugins.jetbrains.com/plugin/26480-kotlin-external-fir-support)
loads compiler plugins from the user's Gradle project into the
IDE's K2 analysis session. No krit-specific IDE plugin needed.
Users install one third-party plugin, and all krit-fir
diagnostics light up in the editor with red/yellow squigglies.
This is how Metro gets IDE integration for free.

**Path 2 — First-party IDE plugin (deferred).** A proper krit
IntelliJ plugin could ship alongside `tools/krit-fir-plugin` and
add things the External FIR Support plugin doesn't — quick-fix
action previews, per-rule suppression annotations, inline
severity controls. This is a **significant** undertaking (IDE
plugin API, packaging, marketplace submission) and is **not in
scope** for Track C.1. Listed here only so future-us remembers
it as an option.

## CLI options

The compiler plugin accepts these options, parsed by
`KritFirCommandLineProcessor`:

| Option | Default | Description |
|---|---|---|
| `enabledRules` | all | Comma-separated rule names to enable |
| `severityOverrides` | (none) | `Rule1=error,Rule2=info` |
| `outputFormat` | compiler | `compiler` (native diagnostics) or `json` (krit-compatible report file) |
| `reportFile` | (none) | Path to write the JSON report if `outputFormat=json` |

The `outputFormat=json` mode is the bridge: a user can run
kotlinc with the plugin and have it emit a krit JSON report
file that `krit check` (or CI tooling) can consume. That's the
workflow for teams that want FIR checkers on CI but can't or
won't add krit's CLI to their build.

## Dual-distribution promise

The same rule file in `tools/krit-fir/src/main/kotlin/dev/krit/fir/rules/FlowCollectInOnCreate.kt`
is consumed by:

1. `tools/krit-fir/` (Track B) — embedded kotlinc in a subprocess
   spawned by the Go CLI
2. `tools/krit-fir-plugin/` (Track C) — kotlinc plugin JAR loaded
   by a user's Gradle build or IDE

**Rule authors write one file. Both distribution targets pick it
up.** This is the whole point of the cluster, and it's the
justification for committing to Option A in B.1 despite the
compat tax.

## Definition of done

- `tools/krit-fir-plugin/` builds a shaded JAR that registers
  as a kotlinc plugin against at least two Kotlin versions
  (current stable + current dev) via the compat shim
- `tools/krit-fir-gradle-plugin/` publishes to the Gradle plugin
  portal under `dev.krit.fir`
- A sample Android project with `id("dev.krit.fir")` applied
  shows pilot-rule diagnostics in both `./gradlew compileDebugKotlin`
  output and the IntelliJ editor (via Kotlin External FIR Support)
- `outputFormat=json` emits a krit-compatible report file that
  `krit check --fir-report <path>` can merge into its output
- CI runs the plugin against a matrix of Kotlin versions; when a
  new Kotlin version is released, a new compat subproject is
  required before bumping the matrix

## Non-goals (for this concept)

- First-party IntelliJ plugin (see Path 2 above — deferred)
- ServiceLoader-based third-party rule extension — reserved for
  a follow-on concept once the plugin is in public use
- Quick-fix application inside the IDE — tree-sitter-based krit
  fixes still require `krit fix` via the CLI; IDE integration for
  fixes is a larger design problem
- Deleting the Go implementations of promoted pilot rules —
  still deferred even in Track C, until the FIR version has
  shipped as the default for at least one release cycle (see
  [pilot-rules.md](pilot-rules.md) on parity-oracle retention)
