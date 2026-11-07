# Kotlin toolchain migration rules for Gradle

These rules read `build.gradle(.kts)` scripts (and, where noted, `gradle.properties`) and report Kotlin,
Android Gradle plugin (AGP) and Compose compiler settings that a newer toolchain rejects, ignores or
deprecates. They are in the `supply-chain` rule set and are scoped to Gradle scripts only: the Kotlin
dispatcher does not run them on `.kts` files because they use the Gradle phase.

| Rule | Default | Fix | Reports |
| --- | --- | --- | --- |
| `KotlinOptionsDsl` | on | partial | `kotlinOptions` DSL |
| `JvmDefaultFlag` | off | partial | `-Xjvm-default` |
| `ObsoleteKotlinVersionFlags` | off | partial | stale compiler flags, language versions, `gradle.properties` keys |
| `KotlinAndroidPluginWithBuiltInKotlin` | on | partial | `org.jetbrains.kotlin.android` with AGP 9 |
| `KaptUsage` | off | none | kapt plugin and `kapt(...)` configurations |
| `ComposeCompilerRemovedFlags` | on | partial | removed Compose compiler Gradle options and feature flags |

Every claim below was checked against the linked primary source. Anything the sources do not state is
not encoded in a rule.

## How the project versions are found

Rules that depend on a version read the Kotlin and AGP versions Krit already extracts from Gradle: plugin
`version` literals in build scripts, and version catalog plugin entries applied through `alias(libs.plugins.*)`.
A version is used only when it is a literal or resolves through the catalog. Krit never assumes a latest
version. When a rule needs a version and none is known, the rule's section says whether it skips the
construct or reports it with lower confidence.

Severity follows the version where a construct changed level: an error where the compiler already
rejects the construct, a warning where it is only deprecated.

Suppress a finding with `@Suppress("RuleName")`, `// krit:ignore RuleName` or a rule `excludes` glob.
In `gradle.properties`, put `# krit:ignore RuleName` or `# noinspection RuleName` on the line above.

## KotlinOptionsDsl

Reports `kotlinOptions { ... }` blocks (inside `android {}`, a Kotlin compile task configuration, a KMP
compilation) and `kotlinOptions.<property>` statements. The finding sits on the `kotlinOptions` identifier.

Verified facts:

- `kotlinOptions` DSL: warning in 2.0.0, error in 2.2.0; every `kotlinOptions` property: error in 2.3.0
  ([compatibility guide 2.2](https://kotlinlang.org/docs/compatibility-guide-22.html),
  [compatibility guide 2.3](https://kotlinlang.org/docs/compatibility-guide-23.html),
  [what's new in 2.2](https://kotlinlang.org/docs/whatsnew22.html)).
- `compilerOptions` takes typed values, not strings, and `+=` becomes `add()`/`addAll()`
  ([Gradle compiler options, migration](https://kotlinlang.org/docs/gradle-compiler-options.html#migrate-from-kotlinoptions-to-compileroptions)).
  The fix uses `JvmTarget.fromTarget("17")` and `KotlinVersion.fromVersion("2.0")`, both shown on that page.
- With AGP built-in Kotlin, `android.kotlinOptions {}` moves to the top-level `kotlin { compilerOptions {} }`
  ([Migrate to built-in Kotlin](https://developer.android.com/build/migrate-to-built-in-kotlin)).

Version handling: Kotlin 2.2 and newer reports an error, 2.0 and 2.1 a warning, an unknown version a
medium-confidence warning. Kotlin older than 2.0 is skipped because the DSL was not deprecated yet.

Default on: the construct is an error for the Kotlin versions current projects use, and the match is the
literal identifier outside comments and strings, so false positives are rare.

Fix (semantic): inside `tasks.withType<KotlinCompile>`-style task configuration in a `.kts` script, the
block becomes `compilerOptions { ... }` when every statement is `jvmTarget = "N"`,
`freeCompilerArgs += listOf(...)` or `+= "..."`, `allWarningsAsErrors`/`suppressWarnings`/`verbose`/
`javaParameters = true|false`, or `languageVersion`/`apiVersion = "X.Y"`. `kotlinOptions.<property> = value`
statements in the same context are rewritten one by one. Everything else is report-only, including
`android { kotlinOptions {} }` (the target moves to `kotlin {}`), Groovy scripts and blocks with comments.

Limits: convention plugins written in `.kt` and the `buildSrc`/`build-logic` sources are not scanned.

## JvmDefaultFlag

Reports the `"-Xjvm-default=..."` string literal in `freeCompilerArgs` lists. The finding spans the literal.

Verified facts:

- Kotlin 2.2.0 adds the stable `-jvm-default` option with modes `enable` (default), `no-compatibility` and
  `disable`, replacing the deprecated `-Xjvm-default`
  ([what's new in 2.2](https://kotlinlang.org/docs/whatsnew22.html),
  [compatibility guide 2.2](https://kotlinlang.org/docs/compatibility-guide-22.html)).
- The Gradle property is `jvmDefault` (`JvmDefaultMode.ENABLE`, `NO_COMPATIBILITY`, `DISABLE`). The documented
  mapping is `-Xjvm-default=all-compatibility` to `ENABLE`, `all` to `NO_COMPATIBILITY` and `disable` to
  `DISABLE`
  ([Gradle compiler options](https://kotlinlang.org/docs/gradle-compiler-options.html),
  [compiler options reference](https://kotlinlang.org/docs/compiler-reference.html)).

Version handling: reports only when the Kotlin version is known to be 2.2.0 or newer. An unknown or older
version is skipped, because a compiler older than 2.2.0 rejects `-jvm-default`.

Default off: the old flag is deprecated, not removed, so it is not an error yet. It also matters for Java
interop, so the choice of mode is a project decision.

Fix (semantic): rewrites the three documented values in place to `-jvm-default=enable`,
`-jvm-default=no-compatibility` and `-jvm-default=disable`. Other values (including the bare flag and the
old `enable` value) have no documented mapping and are report-only.

## ObsoleteKotlinVersionFlags

Reports compiler flags that the project's Kotlin made redundant, language versions the compiler no longer
supports, and obsolete `gradle.properties` keys. Each message names the Kotlin version that changed it.

Verified facts, by construct:

| Construct | Status | Source |
| --- | --- | --- |
| `-Xwhen-guards`, `-Xnon-local-break-continue`, `-Xmulti-dollar-interpolation` | preview flags in 2.1.0, stable in 2.2.0 | [2.1](https://kotlinlang.org/docs/whatsnew21.html), [2.2](https://kotlinlang.org/docs/whatsnew22.html) |
| `-Xcontext-parameters` | needed in 2.2.0, stable in 2.4.0 ("you no longer need to opt in") | [2.2](https://kotlinlang.org/docs/whatsnew22.html), [2.4](https://kotlinlang.org/docs/whatsnew24.html) |
| `-Xexplicit-backing-fields` | needed in 2.3.0, stable in 2.4.0 | [2.3](https://kotlinlang.org/docs/whatsnew23.html), [2.4](https://kotlinlang.org/docs/whatsnew24.html) |
| `-Xannotation-default-target=param-property` | preview flag in 2.2.0, new defaults stable in 2.4.0; `first-only` keeps the old rules and is not reported | [2.2](https://kotlinlang.org/docs/whatsnew22.html), [2.4](https://kotlinlang.org/docs/whatsnew24.html), [annotations](https://kotlinlang.org/docs/annotations.html) |
| `-Xcontext-receivers` | "no longer supported" in 2.3.20 | [2.3.20](https://kotlinlang.org/docs/whatsnew2320.html) |
| `-language-version` 1.6 and 1.7 | error since 2.2.0 | [compatibility guide 2.2](https://kotlinlang.org/docs/compatibility-guide-22.html) |
| `-language-version` 1.8 | error since 2.3.0 | [compatibility guide 2.3](https://kotlinlang.org/docs/compatibility-guide-23.html) |
| `-language-version` 1.9 | error since 2.4.0, with the K1 compiler | [compatibility guide 2.4](https://kotlinlang.org/docs/compatibility-guide-24.html) |
| `kotlin.experimental.tryK2` | replaced by `kotlin.experimental.tryNext` in 2.0.0 | [2.0](https://kotlinlang.org/docs/whatsnew20.html) |
| `kotlin.mpp.enableOptimisticNumberCommonization`, `kotlin.mpp.enablePlatformIntegerCommonization` | error in 2.2.0, removed in 2.2.20 | [compatibility guide 2.2](https://kotlinlang.org/docs/compatibility-guide-22.html) |
| `kotlin.mpp.resourcesResolutionStrategy` | removed in 2.3.0 | [compatibility guide 2.3](https://kotlinlang.org/docs/compatibility-guide-23.html) |
| `kotlin.mpp.enableKotlinToolingMetadataArtifact` | removed in 2.4.0 | [compatibility guide 2.3](https://kotlinlang.org/docs/compatibility-guide-23.html) |
| `kotlin.mpp.import.enableKgpDependencyResolution=false` | deprecation warning since 2.3.0 | [compatibility guide 2.3](https://kotlinlang.org/docs/compatibility-guide-23.html) |

Not encoded because the sources do not state it: any removal version for `apiVersion`, and what a leftover
`-Xcontext-receivers` does on 2.3.20 or newer beyond "no longer supported".

Version handling: a construct is reported only when the project's Kotlin version is known and has reached the
version in the table. The version is lowered to the script's own `languageVersion` when that is older, because a
module pinned to an older language version still needs the experimental flags. An unknown version skips every
construct. `-Xjvm-default` is reported by `JvmDefaultFlag`.

Default off: most constructs are redundant, not errors, so a project can leave them in place. The language
version findings are errors and cannot be turned into a fix, because the replacement is a project decision.

Where it looks: flag and language-version string literals in `freeCompilerArgs` and in
`languageVersion` assignments of build scripts; `gradle.properties` keys through the `settings.gradle(.kts)` in
the same directory.

Fix (semantic): deletes a redundant flag that is a plain list element or the only argument of a
`freeCompilerArgs` statement (the whole statement line goes), and deletes a removed `gradle.properties` key.
Several flags on one line share one edit, attached to the first finding on that line. Language versions,
`-Xcontext-receivers` and the `kotlin.mpp.import.enableKgpDependencyResolution` opt-out are report-only.

## KotlinAndroidPluginWithBuiltInKotlin

Reports `id("org.jetbrains.kotlin.android")`, `id("kotlin-android")`, `kotlin("android")`,
`apply(plugin = "kotlin-android")`, Groovy `id 'kotlin-android'` and `alias(libs.plugins.x)` entries whose
catalog plugin id is one of those, in build scripts of a project on AGP 9.0 or newer.

Verified facts:

- AGP 9.0 enables built-in Kotlin by default, so the `org.jetbrains.kotlin.android` (or `kotlin-android`) plugin is
  no longer applied
  ([AGP 9.0.0 release notes](https://developer.android.com/build/releases/agp-9-0-0-release-notes#android-gradle-plugin-built-in-kotlin)).
- Applying it fails with "The 'org.jetbrains.kotlin.android' plugin is no longer required for Kotlin support since
  AGP 9.0." The opt-out is `android.builtInKotlin=false` together with `android.newDsl=false` in `gradle.properties`;
  one module can opt in with the `com.android.built-in-kotlin` plugin; `kotlin-kapt` becomes
  `com.android.legacy-kapt`; `android.kotlinOptions {}` and `kotlin.sourceSets {}` need migration
  ([Migrate to built-in Kotlin](https://developer.android.com/build/migrate-to-built-in-kotlin)).
- Kotlin 2.3.0 deprecates the plugin for AGP 9.0.0 and later
  ([compatibility guide 2.3](https://kotlinlang.org/docs/compatibility-guide-23.html#deprecate-kotlin-android-plugin-for-agp-versions-9-0-0-and-later),
  [what's new in 2.3](https://kotlinlang.org/docs/whatsnew23.html)). The two Kotlin pages differ on whether the
  result is a warning or an error, so the rule relies on the AGP behavior above.

Version handling: reports only when an AGP version of 9.0 or newer is known, from a plugin `version` literal or the
version catalog. An unknown version skips the script. Pre-release versions such as `9.0.0-alpha05` count as 9.0.

Opt-out: a script is skipped when an applicable `gradle.properties` (its own directory up to the one with the
`settings.gradle(.kts)`) sets `android.builtInKotlin=false`, unless the script applies `com.android.built-in-kotlin`.
Plugin declarations followed by `apply false` only place the plugin on the classpath and are not reported.

Default on, error severity: the build fails on this construct, and the match is an exact plugin id.

Fix (semantic): deletes the plugin line when it holds nothing else and the script has no kapt plugin,
`kotlinOptions` or `sourceSets` use that must be migrated first.

## ComposeCompilerRemovedFlags

Reports the `composeCompiler {}` options `enableStrongSkippingMode`, `enableIntrinsicRemember`,
`enableNonSkippingGroupOptimization`, `stabilityConfigurationFile` and `generateFunctionKeyMetaClasses`
(inside the block, or as `composeCompiler.<option>`), and the flags `ComposeFeatureFlag.StrongSkipping`,
`IntrinsicRemember`, `OptimizeNonSkippingGroups` and `PausableComposition`.

Verified facts:

- Kotlin 2.4.0 turns the five options and `ComposeFeatureFlag.StrongSkipping` and `ComposeFeatureFlag.IntrinsicRemember`
  from warnings into errors. The three `enable*` options warned since 2.0.20 and `stabilityConfigurationFile` since
  2.1.0; the page states no earlier warning for `generateFunctionKeyMetaClasses`. Use `featureFlags` and
  `stabilityConfigurationFiles` instead
  ([compatibility guide 2.4](https://kotlinlang.org/docs/compatibility-guide-24.html)).
- `StrongSkipping`, `IntrinsicRemember` and their DSL properties graduated, are on by default, are
  `DeprecationLevel.ERROR` in 2.4.0 and are removed in 2.5.0. `OptimizeNonSkippingGroups` and `PausableComposition` are
  deprecated in 2.4.0 and removed in 2.6.0
  ([what's new in 2.4](https://kotlinlang.org/docs/whatsnew24.html)).
- The `StrongSkipping` and `IntrinsicRemember` flags are deprecated in 2.2.0
  ([what's new in 2.2](https://kotlinlang.org/docs/whatsnew22.html)).

Version handling: Kotlin 2.4.0 and newer reports an error. Earlier versions report a warning when the construct is
already deprecated there (see the dates above) and nothing before. An unknown version reports the error-class
constructs as a medium-confidence warning. `OptimizeNonSkippingGroups` and `PausableComposition` are reported only from
2.4.0, because they are valid opt-in flags before that.

Default on: the options are explicit DSL names and an error in current Kotlin.

Fix (semantic): deletes `enableStrongSkippingMode = true` and `enableIntrinsicRemember = true` lines, which restate the
defaults. Opt-outs and the other options need a decision and are report-only.

## KaptUsage

Reports the kapt plugin (`id("org.jetbrains.kotlin.kapt")`, `id("kotlin-kapt")`, `kotlin("kapt")`, `apply plugin`
forms and catalog aliases) and `kapt`, `kaptTest`, `kaptAndroidTest`, ... configurations inside `dependencies {}`.
Plugin declarations with `apply false` are skipped.

Verified facts:

- Migration is `kapt(...)` to `ksp(...)`, then removing the kapt plugin; kapt and KSP can run together, so a project can
  move one processor at a time
  ([kapt to KSP migration](https://kotlinlang.org/docs/ksp-kapt-migration.html)).
- Processors with a KSP variant that the rule names: Room, Moshi and Glide are "officially supported"
  ([KSP overview](https://kotlinlang.org/docs/ksp-overview.html)); Dagger and Hilt KSP support is stable as of Dagger 2.60
  and KSP 2.3.9, with `ksp("com.google.dagger:dagger-compiler")` and `ksp("com.google.dagger:hilt-compiler")`
  ([Dagger KSP guide](https://dagger.dev/dev-guide/ksp)). The KSP overview page still lists Dagger as alpha and Hilt
  as in progress; the Dagger page is newer and is the source used. Every other processor gets a generic message.
- `kapt.include.compile.classpath=false` in `gradle.properties` stops kapt from discovering processors on the compile
  classpath ([kapt](https://kotlinlang.org/docs/kapt.html)). The plugin finding includes this hint unless the property is
  already `false`. The `includeCompileClasspath` deprecation warning in the Kotlin 2.4 notes belongs to the Maven
  plugin, so it is not encoded for Gradle.
- With AGP 9 built-in Kotlin the `kotlin-kapt` plugin is incompatible; use `com.android.legacy-kapt` with AGP's version
  until the move to KSP
  ([Migrate to built-in Kotlin](https://developer.android.com/build/migrate-to-built-in-kotlin)). The plugin finding
  says so when AGP 9.0 or newer is known and the project has not opted out of built-in Kotlin.

Default off, info severity, no fix: many projects keep kapt on purpose, and a move to KSP can need a library bump.
