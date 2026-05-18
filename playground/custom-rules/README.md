# playground/custom-rules

Demonstrates the [`dev.jasonpearson.krit.custom`](../../krit-custom-rule-plugin/README.md)
Gradle plugin by authoring two custom Kotlin rules that are loaded by the
sibling [`kotlin-webservice`](../kotlin-webservice) project through the
[`dev.jasonpearson.krit`](../../krit-gradle-plugin/README.md) plugin.

The module deliberately has **no `settings.gradle.kts`** of its own — it is
included as a subproject of `kotlin-webservice` so the host project can reference
it via `customRules(project(":custom-rules"))`.

## Rules

| Rule ID                       | Severity | What it flags                                                                |
|-------------------------------|----------|------------------------------------------------------------------------------|
| `playground.NoHardcodedSecret`| ERROR    | Top-level `val`s named `*PASSWORD*`/`*SECRET*`/`*TOKEN*`/`*KEY*` with a literal value |
| `playground.NoTodoComment`    | WARNING  | `// TODO`, `// FIXME`, and bare `TODO(...)` calls left in production sources |

Both rules are pure line-scan implementations using the `KritRule` SPI from
`dev.jasonpearson.krit:krit-rule-api`. They illustrate the simplest path
(no Kotlin Analysis API dependency) and skip strings / block comments to
avoid common lexical false positives.

## How the plugin pipeline works

`dev.jasonpearson.krit.custom` does the following in this module:

1. Auto-applies `kotlin("jvm")`.
2. Adds `dev.jasonpearson.krit:krit-rule-api` to `implementation`. The
   coordinate resolves to the local source build at
   `tools/krit-rule-api` thanks to the `includeBuild(...)` call in
   `kotlin-webservice/settings.gradle.kts`.
3. Registers `generateKritRuleServices` — scans compiled classes for
   `KritRule` implementations and writes
   `META-INF/services/dev.jasonpearson.krit.api.KritRule`.
4. Registers `kritRuleJar` — a `Jar` task whose manifest is stamped with
   `Krit-SDK-Version`, `Krit-Plugin-Version`, `Krit-Vendor-Id`
   (`playground`), and `Krit-Default-Severity` (`warning`).

5. Publishes the stamped jar as an outgoing variant
   (`kritRuleBundleElements`, with a Krit-specific `Category` attribute) so
   host projects can consume it through Gradle's dependency graph.

The host (`kotlin-webservice`) then wires the produced jar into
`kritCheck` via the `kritCustomRules` resolvable configuration that
`dev.jasonpearson.krit` registers:

```kotlin
dependencies {
    kritCustomRules(project(":custom-rules"))
}
```

This automatically builds `:custom-rules:kritRuleJar` before each
`kritCheck` and passes the jar to the krit CLI through
`--custom-rule-jars` (forcing `--daemon` on, as the JVM-loaded rule path
requires the daemon). The matching `Category` attribute on both sides
means the consumer cannot accidentally pick up a project's normal
`runtimeElements` jar — the daemon-stamped variant is the only thing the
host configuration will resolve.

> **Alternatives** for one-off setups: `krit { customRules(file(...)) }`
> accepts raw jars or task outputs, and `krit { customRules(project(":foo")) }`
> resolves the project's `kritRuleJar` when present (falling back to its `jar`
> task). Both bypass the dependency graph, so prefer `kritCustomRules` for
> project deps.

## Running it

The host build wires in two local artifacts:

- The `krit` Go binary (`make build` at the repo root or
  `go build -o krit ./cmd/krit/`). Without it the plugin falls back to the
  binary published on GitHub Releases.
- The `krit-types` JVM daemon jar
  (`cd tools/krit-types && ./gradlew shadowJar`). The custom-rule loader
  spawns this jar to host the rule classes; set `KRIT_TYPES_JAR` to its
  path or `brew install krit` to enable auto-download.

Then run the host project's `kritCheck`:

```bash
cd playground/kotlin-webservice
KRIT_TYPES_JAR=$PWD/../../tools/krit-types/build/libs/krit-types.jar \
  ./gradlew kritCheck
```

Both custom rules should fire on the existing sources:

- `playground.NoHardcodedSecret` on `JWT_SECRET` and `DATABASE_PASSWORD`
  in `src/main/kotlin/com/example/utils/Constants.kt`.
- `playground.NoTodoComment` on the `// TODO: read port from an environment
  variable…` line added to `src/main/kotlin/com/example/Application.kt`.

Findings are written to `build/reports/krit/krit.sarif`,
`build/reports/krit/krit.json`, and `build/reports/krit/krit.txt`.

`ignoreFailures = true` is set in the host build so the check task does not
fail the build while you iterate — flip it off (or remove it) once the rules
are tuned.
