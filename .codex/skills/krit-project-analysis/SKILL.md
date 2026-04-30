---
name: krit-project-analysis
description: Use when analyzing Krit findings in context of a project's Gradle version catalog, library dependencies, SDK versions, or project model. Covers understanding what the librarymodel package knows about a project, diagnosing rules that fire incorrectly due to absent/present library context, and deciding which rules should be guarded by profile facts.
---

# Krit Project Analysis

Use this when investigating findings that depend on what libraries, SDK versions, or Gradle facts are present in a target project.

## Recent Project-Context Regression Checks

Recent Java/source-index PRs (#676-#680) fixed gaps where project analysis was Kotlin-complete but Java-incomplete. When a finding depends on cross-file, module, Android, or dependency context, verify the project surface includes every language and source kind the rule claims to support:

- Java files must participate in parse discovery, excludes/generated-source filtering, suppression indexing, dispatch cache writes, cross-file references, source-symbol indexing, module grouping, and output file counts.
- Java declarations need package/FQN, owner, arity/signature, static/final, field, method, constructor, record, enum, annotation, and class/interface evidence where the rule uses those facts.
- Module/dead-code checks must use both simple-name and FQN reference evidence and avoid declaration-site self-references marking the declaration live.
- Android or library rules ported to Java need Java positive fixtures plus Java local-lookalike negatives, not just Kotlin coverage.

If a rule appears precise in Kotlin but noisy or silent in Java, inspect the project profile and source index before changing rule heuristics.

## Understand the Project Profile

After running Krit with `--perf`, the JSON output contains the project profile derived from Gradle:

```bash
go build -o krit ./cmd/krit/
./krit -no-cache -perf -f json -q \
  -o /tmp/krit_project.json \
  /path/to/project || true
jq '.projectProfile' /tmp/krit_project.json
```

Key fields to check:

- `hasGradle` — false means Krit had no build files to parse; all library model decisions fall back to defaults
- `dependencyExtractionComplete` — false means Krit found convention plugins, `apply from:` scripts, or other unresolved forms; treat library-presence decisions conservatively
- `hasUnresolvedDependencyRefs` — same conservative signal, more granular
- `catalogCompleteness` — `none` means no TOML catalog was found; `standard_toml` means `gradle/libs.versions.toml` was parsed
- `kotlin.compiler.presence` / `kotlin.k2.presence` — affects language-version-gated rules
- `android.minSdkVersion` / `compileSdkVersion` — affects API-level rules
- `dependencies[].group` + `.name` — the flat list of resolved coordinates

## Top Rules That Need Project Context

From real-project analysis across Signal-Android, nowinandroid, dd-sdk-android, and firebase-android-sdk:

| Rule | Why project context matters |
|------|-----------------------------|
| `DatabaseQueryOnMainThread` | Only fires when Room or SQLDelight is present; uses `LibraryFacts.Database.*` |
| `InjectDispatcher` | True positive rate depends on whether Hilt/Dagger is present |
| `MutableStateInObject` | Correct when Compose is present; may overfire in pure JVM modules |
| `ComposeRawTextLiteral` / `ComposeUnstableParameter` | Should not fire in non-Compose modules |
| `RunBlockingInTest` | Coroutines-test presence affects expected fix |
| `ModuleDeadCode` | Requires `NeedsModuleIndex`; cross-module references are only known when the full module graph was indexed |
| `TestWithoutAssertion` | Test framework affects which assertion styles are valid |
| `MockWithoutVerify` | Only meaningful when Mockito/MockK is present |

## Check Library Model Facts in a Rule

Rules should call `librarymodel.EnsureFacts(ctx.LibraryFacts)` to get the project-derived (or conservative default) facts:

```go
facts := librarymodel.EnsureFacts(ctx.LibraryFacts)
if !facts.Database.Room.Enabled {
    return
}
```

Rules that assume a library is present without checking `facts.Profile.MayUseAnyDependency(...)` will fire in projects that don't use that library. To check:

```bash
grep -rn "LibraryFacts\|EnsureFacts\|MayUseAnyDependency" internal/rules/*.go | grep -v "_test\|gen\b"
```

## Diagnosing Project-Context False Positives

When a rule fires in a project that doesn't use the relevant library:

1. Print the profile to confirm the dependency was not found:
   ```bash
   jq '.projectProfile.dependencies[] | select(.group | contains("room"))' /tmp/krit_project.json
   ```
2. Check whether `dependencyExtractionComplete` is false — if so, absence is not confirmed.
3. Read the rule implementation and verify it calls `EnsureFacts`.
4. If the rule doesn't gate on profile, add a `MayUseAnyDependency` check and test with `CatalogCompletenessNone` and `CatalogCompletenessStandardTOML`.
5. If the rule was recently ported or claims Java support, verify the Java import/object-creation/call/annotation shape is indexed and dispatched before assuming the library gate is wrong.

## Version Catalog Analysis

When a project uses `gradle/libs.versions.toml`, Krit parses it via `internal/librarymodel/catalog.go`. To inspect what was parsed:

```bash
jq '.projectProfile | {catalogCompleteness, catalogSources, unresolvedCatalogAliases}' /tmp/krit_project.json
```

`unresolvedCatalogAliases` lists catalog aliases Krit could not resolve (e.g. from `include build` convention plugins). Rules must not treat absence of a dependency as confirmed when `hasUnresolvedDependencyRefs` is true.

To test catalog parsing directly:

```bash
go test ./internal/librarymodel/ -v -run TestProfile
```

## Adding a New Library-Gated Rule

1. In the rule's `CheckNode` / `CheckLines`, obtain facts:
   ```go
   facts := librarymodel.EnsureFacts(ctx.LibraryFacts)
   ```
2. Guard with `facts.Profile.MayUseAnyDependency(librarymodel.Coordinate{Group: "...", Name: "..."})`.
3. Add a positive fixture with the import/annotation present.
4. Add a negative fixture with `// krit: no-library-model` or in a project where the library is absent (use a minimal `build.gradle.kts` in the fixture dir with no Room/Compose dependency).
5. Add a `profile_test.go` entry confirming the coordinate triggers the library presence flag.

## SDK Version–Gated Rules

```go
profile := librarymodel.EnsureFacts(ctx.LibraryFacts).Profile
if profile.Android.MinSdkVersion < 26 {
    // API 26+ feature not safe to require
    return
}
```

Latest stable constants in `internal/librarymodel/tooling.go`:
- `LatestStableKotlinVersion`, `LatestStableAGPVersion`
- `LatestStableCompileSDK`, `LatestStableTargetSDK`
- `LatestStableJavaVersion`, `LatestStableKotlinJvmTarget`

## ModuleDeadCode Investigation

`ModuleDeadCode` is the highest-volume rule on large multi-module repos. It requires `NeedsModuleIndex` and the cross-file index to be complete. Before acting on findings:

1. Confirm the module graph was indexed:
   ```bash
   jq '.projectProfile.hasGradle' /tmp/krit_project.json
   jq '.perfTiming[] | select(.name=="indexBuild") | .durationMs' /tmp/krit_project.json
   ```
2. Check for Kotlin `@PublishedApi` / `internal` visibility — these affect cross-module reachability.
3. Sample 10–20 findings; look for references from generated code, XML layouts, reflection, or dynamic dispatch that the index cannot see.
4. If false-positive rate is high, check whether `DependencyExtractionComplete` is false — the module boundary may be incomplete.
