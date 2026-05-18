# External Rules

Krit's built-in registry follows the
[rule scope guardrail](rule-scope.md). For rules that don't qualify —
house style, project-specific conventions, opinionated checks — Krit
ships a public Kotlin SDK so a downstream project can add rules
without forking the analyzer.

This document is the front-to-back walkthrough for authoring,
packaging, and running a Kotlin custom rule. If you only want to embed
in-process Go rules (compile your own Krit binary), skip to
[In-process Go registration](#in-process-go-registration-advanced).

## Prerequisites

- JDK 21+
- Gradle 8.0+
- A consumer Kotlin/Android project that already applies the
  `dev.jasonpearson.krit` Gradle plugin (or is willing to add it).

## End-to-end walkthrough

A working reference project lives at `examples/external-rule/` (tracked
in [issue #303](https://github.com/kaeawc/krit/issues/303)). The
sections below describe the same scaffold step by step so the doc
itself stays the source of truth.

### 1. Create the rules module

Two paths produce the same artifact:

**Recommended — apply the custom-rules plugin.** It wires the
`krit-rule-api` dependency, generates the `META-INF/services`
registration, and stamps the manifest attributes Krit's daemon reads at
load time.

```kotlin
// my-rules/build.gradle.kts
plugins {
    id("dev.jasonpearson.krit.custom") version "<krit-version>"
}

kritCustomRules {
    // All properties optional — defaults shown for visibility.
    // ruleApiVersion = "<krit-version>"
    // sdkVersion = "<krit-version>"
    vendorId = "acme"
    defaultSeverity = "warning"
}
```

**Manual.** Add the dependency yourself and ship a hand-written
`META-INF/services` resource. Useful when you need full control over
the build (custom shading, multi-release jar, etc.).

```kotlin
// my-rules/build.gradle.kts
plugins {
    kotlin("jvm") version "<kotlin-version>"
}

dependencies {
    implementation("dev.jasonpearson.krit:krit-rule-api:<krit-version>")
}
```

### 2. Write the rule

`KritRule` is a single-method ServiceLoader interface. `@KritRuleInfo`
carries the metadata Krit needs to schedule and gate the rule.

```kotlin
// my-rules/src/main/kotlin/com/acme/NoTodoRule.kt
package com.acme

import dev.jasonpearson.krit.api.Finding
import dev.jasonpearson.krit.api.KritFile
import dev.jasonpearson.krit.api.KritRule
import dev.jasonpearson.krit.api.KritRuleInfo
import dev.jasonpearson.krit.api.RuleContext
import dev.jasonpearson.krit.api.Severity

@KritRuleInfo(
    id = "acme.NoTodo",
    category = "custom",
    severity = Severity.WARNING,
)
class NoTodoRule : KritRule {
    override fun check(file: KritFile, ctx: RuleContext): List<Finding> {
        val findings = mutableListOf<Finding>()
        file.text.lineSequence().forEachIndexed { index, line ->
            val col = line.indexOf("TODO")
            if (col >= 0) {
                findings += Finding(
                    message = "TODO left in source",
                    line = index + 1,
                    column = col + 1,
                )
            }
        }
        return findings
    }
}
```

`KritFile` exposes:

- `path` — canonical source path.
- `text` — raw source bytes (UTF-8).
- `ktFile` — the Kotlin compiler `KtFile` PSI root (`KtFile?`). Walk
  it with the standard `org.jetbrains.kotlin.psi.*` types. Null when
  the daemon could not parse the file.

`RuleContext` exposes:

- `ruleId` — the active rule's `@KritRuleInfo.id`.
- `config: Map<String, Any?>` — the per-rule `options` map from the
  consumer's `krit.yml`. Use the typed accessors (`stringOption`,
  `intOption`, `boolOption`, `stringListOption`) instead of casting
  directly; they fall through to the supplied default when the option
  is absent or the wrong type.
- `resolver: Resolver?` — narrow, type-aware queries backed by the
  Kotlin Analysis API. Non-null when the rule declared
  `Capability.NEEDS_RESOLVER` and the daemon successfully prepared a
  session for the current file. Methods (see `Resolver` Kdoc):
  `isSuspendCall(KtCallExpression)`, `resolvedCallFqName(...)`,
  `isLambdaSuspend(KtLambdaExpression)`, `expressionType(...)`.
- `gradle: GradleContext?` — project-wide Gradle facts (SDK versions,
  tool versions, declared dependencies). Non-null when the rule
  declared `Capability.NEEDS_GRADLE` and the daemon could derive
  Gradle facts (null on bare Kotlin directories with no build files).
  Properties: `minSdk`, `targetSdk`, `compileSdk`, `kotlinVersion`,
  `javaTargetVersion`, `agpVersion`. Methods:
  `hasDependency(group, name)`, `dependencyVersion(group, name)`.

The PSI surface is the Kotlin compiler's. To compile against it, add
the JetBrains intellij-dependencies redirector to your consumer
`settings.gradle.kts` (the host plugin can't add it for you when
`FAIL_ON_PROJECT_REPOS` is enabled):

```kotlin
// settings.gradle.kts
dependencyResolutionManagement {
    repositoriesMode.set(RepositoriesMode.FAIL_ON_PROJECT_REPOS)
    repositories {
        mavenCentral()
        exclusiveContent {
            forRepository {
                maven("https://redirector.kotlinlang.org/maven/intellij-dependencies")
            }
            filter {
                includeModule("org.jetbrains.kotlin", "kotlin-compiler")
            }
        }
    }
}
```

The host plugin adds `kotlin-compiler:<kotlinVersion>` to
`compileOnly` automatically — no extra dependency declaration needed.

Working example: `examples/external-rule/SuspendInNonSuspendLambdaRule`
flags `suspend` function calls inside non-suspend lambda bodies. It
walks PSI ancestors to find the enclosing lambda, asks the resolver
for the lambda's bound parameter type, and skips the call when that
type is suspend.

### 3. Register the implementation (manual path only)

If you applied `dev.jasonpearson.krit.custom`, skip this — the plugin
scans compiled classes for `KritRule` implementations and writes the
service file for you.

```
my-rules/src/main/resources/META-INF/services/dev.jasonpearson.krit.api.KritRule
```

```
com.acme.NoTodoRule
```

One fully-qualified class name per line. Multiple rules live in the
same file.

### 4. Build the jar

```bash
./gradlew :my-rules:kritRuleJar
```

The task lives in the `krit` group. The produced jar lands in
`my-rules/build/libs/my-rules-<version>-krit-rules.jar` with the
following manifest attributes:

| Attribute | Source |
| --- | --- |
| `Krit-SDK-Version` | `kritCustomRules.sdkVersion` (defaults to `ruleApiVersion`) |
| `Krit-Plugin-Version` | Plugin's own version, baked at publication time |
| `Krit-Vendor-Id` | `kritCustomRules.vendorId` (default `"custom"`) |
| `Krit-Default-Severity` | `kritCustomRules.defaultSeverity` (default `"warning"`) |

Krit's daemon reads `Krit-SDK-Version` when reporting rule
descriptors; the rest are reserved for future load-time gates.

If you took the manual path, the standard `jar` task produces an
equivalent artifact — just make sure the `META-INF/services` resource
is on the classpath.

### 5. Wire the jar into the consumer

The consumer applies `dev.jasonpearson.krit` and pulls the rules module
through the `kritCustomRules` resolvable configuration in the
`dependencies` block:

```kotlin
// app/build.gradle.kts
plugins {
    id("dev.jasonpearson.krit") version "<krit-version>"
}

dependencies {
    kritCustomRules(project(":my-rules"))
}
```

`dev.jasonpearson.krit.custom` publishes the stamped `kritRuleJar`
archive as an outgoing variant with a `krit-rule-bundle` category
attribute; `kritCustomRules` resolves it through Gradle's dependency
graph, so task ordering and artifact production are wired automatically
(no `evaluationDependsOn`, no cross-project task lookup, Project-
Isolation safe).

For one-off jars or task outputs that don't fit the dependency-block
model, append directly to `krit.customRuleJars`:

```kotlin
krit { customRuleJars.from(file("libs/my-rules-0.1.0-krit-rules.jar")) }
```

### 6. Run Krit

```bash
./gradlew :app:krit
```

Console output (text formatter, default):

```
app/src/main/kotlin/com/acme/Foo.kt:12:3: warning [acme.NoTodo] TODO left in source
```

JSON output (`krit { reports { json.required = true } }` or
`--format json` from the CLI):

```json
{
  "file": "app/src/main/kotlin/com/acme/Foo.kt",
  "line": 12,
  "column": 3,
  "ruleSet": "custom",
  "rule": "acme.NoTodo",
  "severity": "warning",
  "message": "TODO left in source",
  "fixable": false
}
```

For a CLI smoke test without Gradle wiring:

```bash
krit --custom-rule-jars my-rules/build/libs/my-rules-0.1.0-krit-rules.jar src/
```

`--custom-rule-jars` keeps Krit on the in-process path (the
daemon-eligibility gate routes around it), so first runs do not need
the prebuilt `krit-types` daemon on disk.

## Per-rule configuration (`pluginRules`)

The consumer's `krit.yml` controls plugin rules through a dedicated
top-level `pluginRules` section, keyed by `@KritRuleInfo.id`:

```yaml
pluginRules:
  acme.NoTodo:
    active: false           # silence a noisy rule without removing the jar
  acme.MaxLineLength:
    options:
      maxLineLength: 100    # forwarded as RuleContext.config["maxLineLength"]
      ignoredFiles:
        - 'generated/**'
```

Each entry accepts two keys:

| Key | Type | Behavior |
| --- | --- | --- |
| `active` | bool | When `false`, Krit skips the rule before sending the RPC — zero findings, zero work. Omit to use the rule's default activation. |
| `options` | map | Free-form key/value pairs exposed verbatim to the rule via `RuleContext.config`. Values come through with their YAML types (string, int, bool, list). |

Inside the rule, read options through the typed helpers so a missing
or wrong-typed value falls back to your default:

```kotlin
override fun check(file: KritFile, ctx: RuleContext): List<Finding> {
    val max = ctx.intOption("maxLineLength", default = 120)
    val ignored = ctx.stringListOption("ignoredFiles")
    // ...
}
```

`krit --validate-config` validates the `pluginRules` shape (object,
allowed keys `active` / `options`, correct types) so typos surface
before analysis runs. The rule IDs themselves are *not* validated
against the loaded jars — IDs are owned by user-supplied plugins and
may not be known to the binary at config-load time.

## Capability semantics

`@KritRuleInfo.needs` declares which project-scope facts the rule
expects. The daemon either delivers the requested fact into
`RuleContext` or refuses to load the jar — there is no third "advisory"
state. A rule that declares a capability the daemon does not recognise
(typo, ahead-of-daemon SDK) fails at jar load with a clear,
copy-pasteable diagnostic.

| Capability | Status | What it gives you |
| --- | --- | --- |
| `NEEDS_RESOLVER` | **supported** | Populates `RuleContext.resolver` with the [`Resolver`](#) bridge. Methods: `isSuspendCall`, `resolvedCallFqName`, `isLambdaSuspend`, `expressionType`. Each call opens a Kotlin Analysis API session — expect microsecond-class overhead per query. |
| `NEEDS_PARSED_FILES` | **supported (implicit)** | The daemon always parses the Kotlin file before invoking `check()` and exposes the result on `KritFile.ktFile`. Declaring it is a forward-compatible hint; omitting it changes nothing today. |
| `NEEDS_GRADLE` | **supported** | Populates `RuleContext.gradle` with a `GradleContext`. Properties: `minSdk`, `targetSdk`, `compileSdk`, `kotlinVersion`, `javaTargetVersion`, `agpVersion`. Methods: `hasDependency(group, name)`, `dependencyVersion(group, name)`. Null when the project has no Gradle build files. |
| `NEEDS_MANIFEST` | **supported** | Populates `RuleContext.manifest` with a `ManifestContext`. Properties: `packageName`, `minSdk`, `targetSdk`. Methods: `hasPermission(name)`, `hasActivity(name)`, `isActivityExported(name)`, plus the matching `hasService`/`isServiceExported`/`hasReceiver`/`isReceiverExported` pairs. Null when the project has no `AndroidManifest.xml`. |
| `NEEDS_RESOURCES` | **supported** | Populates `RuleContext.resources` with a `ResourcesContext`. Methods: `stringValue(name)`, `hasString(name)`, `colorValue(name)`, `dimensionValue(name)`, `hasDrawable(name)`, `hasLayout(name)`, `hasId(name)`. Null when the project has no Android `res/` directory. |
| `NEEDS_MODULE_INDEX` | **supported** | Populates `RuleContext.moduleIndex` with a `ModuleIndexContext`. Properties: `modulePaths`. Methods: `directoryOf(modulePath)`, `dependenciesOf(modulePath)`, `sourceRootsOf(modulePath)`. Null when the project has no Gradle modules. |
| `NEEDS_CROSS_FILE` | **supported** | Populates `RuleContext.crossFile` with a `CrossFileContext`. Methods: `declarationByFqn(fqn)`, `referenceFiles(name)`, `isReferenced(name)` (non-comment references only). Null when the daemon's cross-file pass did not run. The wire payload can be sizable on large projects — declare this capability only when the rule genuinely needs whole-project visibility. |

### What a load failure looks like

A jar that declares an unrecognised capability (typo, ahead-of-daemon
SDK) is rejected at `listPlugins` time, before any rule from that jar
runs:

```
error: krit-rule-api: /tmp/acme-rules.jar: rule jar declares capabilities
the daemon does not yet provide to plugin rules; the rule would run
without the facts it asked for. Remove the declaration(s) or wait for
support. Unsupported: [acme.NoTodo: NEEDS_TIME_TRAVEL]
```

The diagnostic surfaces on the same channels as the SDK-compat verdict:
`--list-rules` prints it before the rule table, `krit` (scan) fails the
run, and `listPlugins` returns it under `result.diagnostics`. The
remediation is to either delete the bad enum from `@KritRuleInfo.needs`
or — once the corresponding hook lands — rebuild against a daemon that
moves the entry into the supported set.

### Wire payload cost

Project-scope payloads ride on every `analyzeFile` RPC the Go pipeline
issues — one per Kotlin file. For payloads that are at most a few KB
(gradle, manifest, module index, most projects' resources) the per-file
re-serialisation is negligible. `NEEDS_CROSS_FILE` is different: on a
100 KLOC project the declarations + references payload can run into
the megabytes, and shipping it ~1000 times per scan is wasteful.

A future change will hoist project-scope payloads onto a separate
`setProjectContext` RPC the daemon caches per session; until then,
declare `NEEDS_CROSS_FILE` only when the rule genuinely needs
whole-project visibility, and prefer narrower lookups (`isReferenced`,
`declarationByFqn`) over walking the full lists. Tracked alongside the
per-file pattern in
[#357](https://github.com/kaeawc/krit/issues/357).

### Project facts can be null

The project-scope accessors (`gradle`, `manifest`, `resources`,
`moduleIndex`, `crossFile`) are non-null only when the rule declared
the matching `@KritRuleInfo.needs` value AND the Go pipeline assembled
a payload for that fact. A pure-Kotlin library project with no
`AndroidManifest.xml` leaves `manifest` and `resources` null even when
the rule declared them. Rules should defensively null-check before
dereferencing.

### Adding a new capability

Adding a new capability is a minor-version change on `krit-rule-api`:
extend the `Capability` enum, grow `RuleContext` with the matching
`XxxContext` interface, mirror the wire payload on both sides
(`PluginXxxProfile` in `internal/oracle/daemon.go`,
`XxxProfilePayload` in `tools/krit-types/.../Main.kt`), wire the
extractor and the `PayloadXxxContext` impl in `PluginRules.kt`, add
the per-project builder in `internal/pipeline/custom_kotlin_rules.go`,
add the value to `PluginCapabilities.SUPPORTED`, and update the matrix
above. Existing rule jars that already declared the capability start
running with the new fact wired in — no rule-jar rebuild required.

## FixSafety levels

`Finding.fix` carries an optional `Fix(startLine, endLine,
replacement, safety)`. `FixSafety` mirrors the built-in tiers:

| Tier | Use when | Examples |
| --- | --- | --- |
| `COSMETIC` | Pure formatting; cannot change runtime behavior or compile success. | Trailing whitespace, missing final newline. |
| `IDIOMATIC` (default) | Behavior-preserving rewrite a reviewer would accept without thinking. | `.let { it.foo() }` → `?.foo()`, redundant `toString()` removal. |
| `SEMANTIC` | Behavior change that a human must review. | Replacing `==` with `equals(..., ignoreCase = true)`, swapping deprecated API for a non-equivalent replacement. |

The consumer caps which tiers apply via `--fix-level` (CLI) or the
`krit { advanced { fixLevel = "..." } }` Gradle DSL. The flag default is
`idiomatic`, and the cap is inclusive moving up the table:

- `--fix-level=cosmetic` applies only `COSMETIC` fixes.
- `--fix-level=idiomatic` applies `COSMETIC` and `IDIOMATIC`.
- `--fix-level=semantic` applies all three.

Findings whose fix safety exceeds the cap stay in the report — only
the `fix` payload is stripped. Pick the most conservative tier that
honestly describes the rewrite: marking a semantic change as cosmetic
will silently apply it under any user's `--fix-level`.

## Versioning and compatibility

**Maturity.** `@KritRuleInfo.maturity` defaults to `EXPERIMENTAL`.
Experimental rules are off by default and only run when the consumer
opts in (`--experimental`, or `experimental: true` in `krit.yml`).
`STABLE` is on by default. `DEPRECATED` runs but emits a deprecation
hint and should be removed in a future release.

The whole external-rules surface is itself "experimental" in the
broader sense: the API may add fields and methods in a minor release.
Source-compatible changes are favored, but anything labeled
experimental on the Krit side may shift without a major version bump
until the SDK stabilizes.

**Resolver surface.** `Resolver` is the narrowest surface we could
ship that still lets rules express type-aware checks. We intentionally
do not expose `KaSession`, `KaSymbol`, or other JetBrains-internal
types — the Kotlin Analysis API is itself unstable across compiler
versions, and direct exposure would couple every rule jar to the exact
analysis-api-for-ide version Krit links. The methods on `Resolver`
will grow in source-compatible ways (new methods, new defaults); the
underlying KAA calls may change without notice between Krit releases.

**SDK version.** Krit publishes `dev.jasonpearson.krit:krit-rule-api`
to Maven Central in lockstep with each Krit release (see
[release notes](release.md)). The `krit-rule-api` jar carries an
`Implementation-Version` manifest attribute matching the published
coordinate, and the `dev.jasonpearson.krit.custom` plugin stamps
`Krit-SDK-Version` into every consumer rule jar's manifest. The Krit
daemon reads both at load time and compares them via the
[compatibility matrix](#compatibility-matrix) below.

#### Compatibility matrix

The daemon's verdict for a rule jar's `Krit-SDK-Version` vs. the
daemon's own bundled `krit-rule-api` version:

| Rule jar SDK | Daemon SDK | Verdict | Behavior |
| --- | --- | --- | --- |
| Exact match | — | ok | Silent. |
| Patch differs only (e.g. `1.2.3` vs. `1.2.7`) | within same `1.x.y` | ok | Silent. The API is source- and binary-compatible across patch releases. |
| Minor differs (e.g. `1.2.x` vs. `1.3.x`), major ≥ 1 | same major | warn | Rules still load; reporter prints a drift warning. New `Resolver` methods may not exist; rebuild against the daemon's version to stay supported. |
| Minor differs under `0.x` (e.g. `0.2.x` vs. `0.3.x`) | major = 0 | error | Daemon refuses to load any rules from the jar. Pre-1.0 minor bumps are treated as breaking under semver. |
| Major differs (e.g. `1.x.y` vs. `2.x.y`) | — | error | Daemon refuses to load. Rebuild required. |
| Missing `Krit-SDK-Version` manifest attribute | — | warn | Rules still load. Suggests the jar was built by hand or with an old version of the custom-rule plugin. |
| Unparseable version string | — | warn | Rules still load; the daemon cannot reason about compatibility. |
| Either side `0.0.0-SNAPSHOT` (local dev) | — | ok | Silent — composite-build dogfooding wouldn't otherwise be useful. |

Diagnostics surface in two places:

- **CLI**: `--list-rules --custom-rule-jars …` prints a `warn:` /
  `error:` line per jar before the rule table.
- **Scan**: warnings are routed through the standard warning stream
  (stderr by default). An `error`-level diagnostic fails the run with
  an `incompatible custom rule jar(s); rebuild …` message — silently
  skipping the jar would hide the fact that the rules never ran.

The same diagnostics ride on the daemon's `listPlugins` RPC response
under `result.diagnostics`, so other consumers (LSP, MCP, Gradle) can
render them their own way. `analyzeFile` does not re-emit them — call
`listPlugins` first if you need to surface compatibility verdicts in
a custom front-end.

**Semver policy for `krit-rule-api`.** The rule API follows standard
semver, with the usual pre-1.0 caveat that minor bumps may be
breaking. Concretely:

- **Patch** (`X.Y.Z` → `X.Y.(Z+1)`): bug fixes, doc-only changes,
  internal refactors. Source- and binary-compatible. Existing rule
  jars run unchanged.
- **Minor** (`X.Y.*` → `X.(Y+1).0`): additive only — new `Resolver`
  methods (with default implementations once Kotlin allows on
  interfaces, otherwise via new abstract methods scoped to the
  daemon-side implementation), new `Capability` enum entries, new
  `KritRuleInfo` fields with defaults. Rebuilding against the new
  version is recommended but not required to keep loading.
- **Major** (`X.*.*` → `(X+1).0.0`): removed or renamed symbols,
  changed method signatures, semantic changes to existing methods. A
  rebuild is required.
- **Pre-1.0** (`0.Y.Z`): minor bumps may be breaking. The daemon
  treats them as breaking by default.

Pre-release identifiers (`-rc1`, `-alpha`) and build metadata
(`+sha.abc`) are ignored for compatibility comparisons; the policy is
expressed in terms of `MAJOR.MINOR` only.

**ServiceLoader contract.** The interface name
`dev.jasonpearson.krit.api.KritRule` is stable. The metadata
annotation is `@KritRuleInfo` (not `@KritRule`) because Kotlin
disallows a class and an annotation sharing a name in the same
package; treat the asymmetric naming as load-bearing.

## In-process Go registration (advanced)

When the consumer builds Krit from source — for example, when shipping
a heavily-customized Krit binary internally — they can register rules
in-process via [`pkg/extension`](../pkg/extension/extension.go):

```go
package myteamrules

import (
    "github.com/kaeawc/krit/pkg/extension"
)

func init() {
    extension.Register(&extension.Rule{
        ID:          "MyTeamRule",
        Category:    "myteam",
        Description: "Forbid direct calls to InternalThing",
        Sev:         extension.SeverityWarning,
        NodeTypes:   []string{"call_expression"},
        Maturity:    extension.MaturityExperimental,
        Check: func(ctx *extension.Context) {
            // ...
        },
    })
}
```

Importing the package anywhere in the build
(`import _ "myorg/myteamrules"` in `cmd/krit/main.go`, for example)
is enough — Go's `init()` runs the registration, and the dispatcher
picks the rule up alongside the built-ins.

In-process rules participate in every analyzer feature on equal
footing:

- `Maturity` gating (default-inactive `Experimental`, opt-in via
  `--experimental` or `experimental: true` in `krit.yml`).
- `RunAfter` ordering, so an external rule can depend on a built-in
  by ID.
- All scope buckets: per-file node, line pass, cross-file, manifest,
  resource, gradle, aggregate.
- Suppression via `@Suppress("MyTeamRule")` and `// krit:ignore[...]`.

The `--list-rules` output, JSON/SARIF emitters, baseline files, and
LSP/MCP servers see in-process rules identically to built-ins. The
trade-off is that this path requires recompiling and distributing your
own Krit binary; the Kotlin SDK above is the right answer for almost
every team.

## Open design questions for out-of-tree loading

In-process Go registration requires recompiling Krit's binary. The
Kotlin SDK now covers the Kotlin direction. The broader loader
options remain open:

- **Go plugins** (`-buildmode=plugin`): supported on Linux/macOS but
  notoriously brittle (toolchain pinning, race-detector incompatibility,
  no reload on macOS). Not a good fit.
- **Sidecar process**: spawn a rule provider over a stable IPC contract
  (similar to `tools/krit-types/`). Most flexible, biggest implementation
  cost (RPC schema, lifecycle, performance).
- **WASM rule sandbox**: portable, sandboxed, slower than native. Open
  question whether the API surface (`scanner.File`, `FlatTree`, oracle
  facts) maps cleanly to a WASM contract.
- **Embedded scripting** (Starlark, Lua): cheaper to integrate; only
  expresses lexical/AST checks. Insufficient for type-aware rules.

The Kotlin SDK above is the stable path while these alternatives
remain prototypes.
