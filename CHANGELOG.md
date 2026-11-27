# Changelog

All notable changes to Krit will be documented in this file.

## [Unreleased]

### Added
- The per-file analysis cache (`.krit/cache`) survives a move to another absolute checkout path, such as a CI cache
  restored into another agent's workspace. Paths under the repository are stored relative to it, and the cache file
  name, rule hash and library facts fingerprint no longer include the checkout path (#1662).
- Module-shard FIR sessions (source-set isolation, per-settings-group sessions) put the upstream project outputs from
  `--gradle-project-outputs` on the classpath, ahead of a jar at a conventional build path, instead of compiling those
  projects from source again (#1681).
- Per-project attribution for whole-build scans: with a Gradle model loaded, JSON findings carry `project` and
  `summary.byProject` counts findings, errors and warnings per Gradle project (files outside every project under `""`);
  SARIF has the same in each result's `properties.project` and `runs[0].properties.byProject`. The Gradle plugin's
  `kritCheckAll` logs the counts (#1635, #1651).
- Gradle model: Kotlin Multiplatform Android targets record the generated roots AGP's Variant API registers for their
  component (view binding, BuildConfig, AIDL, `addGeneratedSourceDirectory`), and `kritCheck`/`exportGenerated` run
  their producers, as plain Android modules already did (#1643).
- Gradle model freshness key for CI: `krit gradle-model key` hashes the build files that can change
  `kritExportModel`'s output (settings and build scripts, version catalogs, lockfiles, dependency verification metadata,
  `gradle.properties`, the wrapper version, the sources of buildSrc and of builds nested under the root) by root-relative
  path and content. A CI job caches `.krit/gradle-model` under it and skips Gradle on a hit. `--write` stamps the key and
  its inputs' hashes in the model, and krit warns, naming the file, when the build no longer matches. A model restored under another checkout path or Gradle user home is rebased when
  loaded (new `gradleUserHome` model field). New `krit { exportTests = false }` exports and resolves only production
  source sets (#1663).
- `UseOfNonLambdaOffsetOverload`, `UnusedBoxWithConstraintsScope` and `TextFieldBufferInternalAppend`, ports of the
  Lint checks bundled with `androidx.compose.foundation` 1.13.0, under their Lint IDs. Their findings match the Lint
  CLI running that release's `lint.jar` on the inputs under `internal/rules/testdata/androidx_compose_foundation/`
  (#1438, #1439, #1440).
- `ExperimentalAnnotationRetention` and `WrongRequiresOptIn`, ports of the declaration checks bundled with
  `androidx.annotation:annotation-experimental` 1.6.0, under their Lint IDs. Their findings match the Lint CLI running
  that release's `lint.jar` on the inputs under `internal/rules/testdata/androidx_annotation_experimental/` (#1317,
  #1318).
- `ExperimentalAnnotationRetention`, `WrongRequiresOptIn`, `UnsafeOptInUsageError` and `UnsafeOptInUsageWarning`,
  ports of the checks bundled with `androidx.annotation:annotation-experimental` 1.6.0, under their Lint IDs; the usage
  checks cover opt-in markers declared in the project. Their findings match the Lint CLI running
  that release's `lint.jar` on the inputs under `internal/rules/testdata/androidx_annotation_experimental/` (#1315,
  #1316, #1317, #1318).
- `NullSafeMutableLiveData` and `UnsafeLifecycleWhenUsage`, ports of the Lint checks bundled with
  `androidx.lifecycle:lifecycle-livedata-core` and `lifecycle-runtime` 2.11.1, under their Lint IDs. Their findings
  match the Lint CLI running those releases' `lint.jar` on the inputs under
  `internal/rules/testdata/androidx_lifecycle_runtime/` (#1451, #1452).
- `IncorrectReferencesDeclaration`, `IncorrectMatchParentUsage` and `IncorrectChainMarginsUsage`, ports of the
  ConstraintLayout Compose DSL Lint checks bundled with `constraintlayout-compose-android` 1.1.2, under their Lint IDs.
  Their findings match the Lint CLI running that release's `lint.jar` on the inputs under
  `internal/rules/testdata/androidx_constraintlayout/` (#1329, #1330, #1331).
- `EnsureInitializerNoArgConstr` and `EnsureInitializerMetadata`, ports of the Lint checks bundled with
  `androidx.startup:startup-runtime` 1.2.0, for Kotlin and Java under their Lint IDs. Their findings match the Lint CLI
  running that release's `lint.jar` on the project under `internal/rules/testdata/androidx_startup/`, manifest
  `<meta-data>` entries included (#1484, #1485).
- PR mode for `--diff` (#1660): with a Gradle model, the type oracle and the FIR check compile only the modules that
  hold the changed Kotlin files, with their dependencies as built jars or source context, instead of the whole
  repository; every other file keeps Go's verdict. It falls back to the whole-repository compile when a module would
  lose dependency context or the model is missing, and `--no-pr-mode` turns it off. The FIR summary line and
  `summary.fir.outsidePr` say how many files stayed with Go.
- `MissingOnRenderProcessGone`, `CrossOriginIsolatedAllowlist` (`androidx.webkit` 1.18.0) and
  `DontUseAccessibilityNodeInfoGetText` (`androidx.test.uiautomator` 2.4.1), under their Lint IDs, checked against
  the Lint CLI running those releases' `lint.jar` (`internal/rules/testdata/androidx_webkit/`,
  `androidx_uiautomator/`). As in Lint, each runs only when the project declares the artifact that ships it.
  `DontUseAccessibilityNodeInfoGetText` also matches the released `onElement*` names, which the shipped detector no
  longer recognizes (#1505, #1506, #1507).

- Gradle plugin: `kritCheck` with custom rule jars writes every enabled report from one krit run (one
  `--report FORMAT=PATH` each), like every other scan, instead of analyzing once per report (#1634).
- `DoNotMockAnything`, `DoNotMockPlatformTypes`, `DoNotMockDataClass` and `DoNotMockObjectClass`, slack-lints 0.11.1's
  `MockDetector` checks, in the opt-in `slack-lint` rule set for Kotlin and Java test sources. One pass over the test
  files finds Mockito, mockito-kotlin and `slack.test.mockito` factory calls and `@Mock`/`@Spy` fields, resolves the
  mocked class through imports and the cross-file index, and runs the detector's checkers in its order. Findings match
  slack-lints' own detector tests (`internal/rules/testdata/slack_lint_upstream/`); a `mock()` whose type Lint infers
  from an enclosing call's parameter is not reported (#1375, #1376, #1377, #1380).
- `--min-sdk N` / `--target-sdk N` set the Android SDK levels of files whose own `build.gradle(.kts)`, manifest or Gradle
  version catalog declares none, for the Go rules (per-file and project-wide minSdk alike) and the FIR pass. They are
  part of the analysis cache key. The Bazel rules pass them from `--@krit//:min_sdk_version` /
  `--@krit//:target_sdk_version`, and an `android_binary`'s own `krit_aspect` action from its `manifest_values`, since
  Bazel builds often declare the levels only there (#1682).
- `LifecycleCurrentStateInComposition`, `ViewModelConstructorInComposable`, `TestLifecycleOwnerInCoroutine` and
  `RepeatOnLifecycleWrongUsage`, ports of the Lint checks bundled with `androidx.lifecycle` 2.11.1, under their Lint
  IDs, checked against the Lint CLI running those releases' `lint.jar` (`internal/rules/testdata/androidx_lifecycle/`).
  `TestLifecycleOwnerInCoroutine` follows the detector source, as the CLI reports nothing for it against the released
  classes (#1453, #1454, #1455, #1456).
- `UnusedSharedTransitionModifierParameter`, `ConstantContentStateKeyInItemsCall`,
  `DisallowLookaheadAnimationVisualDebug` and `ArcAnimationSpecTypeIssue`, the remaining Lint checks bundled with
  `androidx.compose.animation` 1.13.0, under their Lint IDs and checked against the Lint CLI running those releases'
  `lint.jar` (`internal/rules/testdata/androidx_compose_animation/`). Lint's quick fixes are not ported (#1310, #1311,
  #1312, #1314).
- `LaunchDuringComposition`, `NoCollectCallFound` and `ContextCastToActivity`, ports of the Lint checks bundled with
  `androidx.activity:activity-compose` 1.13.0, under their Lint IDs. Their findings match the Lint CLI running that
  release's `lint.jar` on the inputs under `internal/rules/testdata/androidx_activity_compose/` (#1302, #1303, #1304).
- `UnusedCrossfadeTargetStateParameter`, `UnusedContentLambdaTargetStateParameter`,
  `UnusedTargetStateInContentKeyLambda` and `UnusedTransitionTargetStateParameter`, ports of the Lint checks bundled
  with `androidx.compose.animation` 1.13.0 (`animation-android`, `animation-core-android`), under their Lint IDs.
  Their findings match the Lint CLI running those releases' `lint.jar` on the inputs under
  `internal/rules/testdata/androidx_compose_animation/` (#1307, #1308, #1309, #1313).
- `RecyclerView`, `FindViewByIdCast`, `RemoteViewLayout`, `DefaultUncaughtExceptionDelegation` and
  `WrongGradleMethod`, ports of Android Lint 32.4.1's `RecyclerViewDetector`, `ViewTypeDetector` (FindViewByIdCast,
  with Lint's cast fix), `RemoteViewDetector`, `UncaughtExceptionHandlerDetector` and `WrongGradleMethodDetector`
  (command-line patterns). Findings match Lint on 16 recorded project fixtures; `scripts/lint-parity-record.sh` now
  records Groovy build scripts and flavored modules too (#1248, #1290, #1250, #1282, #1300).
- `ScopedStorage`, `SelectableText`, `CustomSplashScreen`, `VectorDrawableCompat` and `TranslucentOrientation`, ports
  of Android Lint 32.4.1's `ScopedStorageDetector`, `TextViewDetector` (SelectableText), `SplashScreenDetector`,
  `VectorDrawableCompatDetector` and `TranslucentViewDetector` under their Lint IDs. Findings match Lint on 23
  recorded project fixtures (#1259, #1273, #1266, #1288, #1279).
- `StringEscaping`, `InvalidNavigation`, `ValidRestrictions` and `RelativeClassResolution`, ports of Android Lint
  32.4.1's `StringEscapeDetector`, `StartDestinationDetector`, `RestrictionsDetector` and
  `RelativeClassAttributeDetector` under their Lint IDs and severities. Findings match Lint on 17 recorded project
  fixtures (one documented column divergence). `scripts/lint-parity-record.sh` gains a standalone mode that runs Lint
  32.4.1's own CLI for fixtures whose resources AGP refuses to compile (#1270, #1267, #1253, #1249).
- `UnusedMaterialScaffoldPaddingParameter`, `UnusedMaterial3ScaffoldPaddingParameter`, `ConflictingOnColor` and
  `UsingMaterialAndMaterial3Libraries`, ports of the Lint checks bundled with `androidx.compose.material` 1.13.0 and
  `androidx.compose.material3` 1.5.0, under their Lint IDs. Their findings match the Lint CLI running those releases'
  `lint.jar` on the inputs under `internal/rules/testdata/androidx_compose_material/`. The material3 import check needs
  a material3 import in the file or a material3 dependency in the build (#1457, #1458, #1459, #1461).
- Compose lint parity, fourth batch of compose-lints 1.6.1 ports: `ComposeMutableParameters`,
  `ComposeUnstableCollections`, `ComposeViewModelForwarding` and `ComposeViewModelInjection` (with
  `viewModelFactories`), under their Lint IDs, messages and severities, experimental and off by default. On
  nowinandroid and compose-samples the findings, messages included, match the Lint CLI exactly (#1343, #1351, #1352,
  #1353).
- Compose lint parity, third batch of compose-lints 1.6.1 ports: `ComposeModifierMissing` (with `contentEmitters` and
  `visibilityThreshold`), `ComposeModifierWithoutDefault` (autofix: a default in the parameter type's own spelling) and `ComposeParameterOrder`, under
  their Lint IDs, messages and severities, experimental and off by default. On nowinandroid and compose-samples their
  findings, messages included, match the Lint CLI exactly (#1338, #1340, #1345).
- Compose lint parity, second batch of compose-lints 1.6.1 ports: `ComposePreviewPublic`, `ComposePreviewNaming`
  (cross-file, so multipreview annotations from other files count), `ComposeM2Api` (with `allowedM2Apis`),
  `ComposeItemKeyHashCode` and `ComposeModifierComposed`, under their Lint IDs, messages and severities, experimental and
  off by default. On nowinandroid and compose-samples their findings match the Lint CLI exactly (#1337, #1341, #1346,
  #1347, #1354).
- Gradle plugin: `kritCheckAll` and the per-variant / per-source-set `kritCheck<Name>` tasks run the generated-source
  producers (Variant API and KGP roots, R-class, kapt stubs) when FIR is on, as `kritCheck` already did, so FIR no
  longer gates generated-code users on a clean checkout without `exportGenerated` (#1642, #1686).
- Compose lint parity, first batch of [compose-lints](https://github.com/slackhq/compose-lints) 1.6.1 ports:
  `ComposeCompositionLocalUsage` (with the `allowedCompositionLocals` option) and `ComposeCompositionLocalGetter`,
  under their Lint IDs, messages and severities, experimental and off by default. `ComposeNamingUppercase` and
  `ComposeNamingLowercase` map onto the existing `ComposableNaming`: suppressing either ID silences its casing direction,
  and `ComposableNaming` gains compose-lints' `allowedComposableFunctionNames` option. Findings are checked against the Lint CLI running compose-lint-checks
  1.6.1 on the same inputs (#1332, #1333, #1334, #1335).
- `MustBeInModule`, `BindsMustBeAbstract`, `ProvidesMustNotBeAbstract`, `BindingReturnType` and
  `BindingReceiverParameter`, slack-lints 0.11.1's `DaggerIssuesDetector` checks, in the opt-in `slack-lint` rule set
  for Kotlin and Java. They follow the detector's one-issue-per-method order and match the Lint CLI on the same inputs
  (`internal/rules/testdata/slack_lint/dagger/`), except that a finding is anchored at the declaration rather than at a
  KDoc or comment before it. The detector's `BindsWrongParameterCount` and `RedundantBinds` are now Lint aliases of
  `BindsMismatchedArity` and `BindsReturnTypeMatchesParam` (#1359, #1360, #1361, #1362, #1363, #1364, #1365).
- `TrimLambda`, `TextConcatSpace` and `WrongCommentType`, ports of Android Lint 32.4.1's `TrimDetector`,
  `TextConcatDetector` and `WrongCommentTypeDetector` (Kotlin; Kotlin and Java; Kotlin and Java) under their Lint IDs,
  with Lint's autofixes for `TrimLambda` and `WrongCommentType`. Findings match Lint on 14 recorded project fixtures,
  including Kotlin comment binding, escapes and templates at concatenation joints, and suppression (#1280, #1272,
  #1299).
- `SquareAndRoundTilePreviews`, `TilePreviewImageFormat` and `StartActivityAndCollapseDeprecated`, ports of Android
  Lint 32.4.1's Wear tile preview checks and its `TileService.startActivityAndCollapse(Intent)` check (Kotlin and
  Java, error from target SDK 34, warning below). Findings match Lint on 22 recorded project fixtures, including
  WebP and XML previews, round/square drawable folders, suppression and lookalike receivers (#1275, #1276, #1277).
- `WearableConfigurationAction`, `WearableActionDuplicate`, `WatchFaceForAndroidX`, `WearBackNavigation` and
  `WearMaterialTheme`, ports of Android Lint 32.4.1's Wear OS watch face configuration, swipe-to-dismiss and
  Compose `MaterialTheme` checks under their Lint IDs, severities and messages. Their findings match Lint on 24
  recorded project fixtures, `tools:ignore` and `@file:Suppress` included. `scripts/lint-parity-record.sh` records
  what Lint (AGP 9.4.1) reports for a fixture module, and `TestAndroidLintParityFixtures` replays krit on every
  recorded fixture (#1291, #1292, #1293, #1294, #1295).
- FIR sessions resolve Java sources: every `.java` file under the source dirs, generated source dirs and unpacked
  srcjars (same-module Java, view binding, BuildConfig, Wire-Java, kapt/KSP Java output) is given to K2 as a Java
  source root, so Kotlin that uses them is no longer gated `Unresolved reference` on an unbuilt checkout. The Java is
  read for resolution only, never checked. A Java edit changes krit-fir's compile key and the Go FIR and oracle cache
  fingerprints, so no stale verdict is served (#1626, #1630).
  longer gated; generated Java is resolved too (#1626). Each srcjar is unpacked once into

- `CompileTimeConstant`, a partial port of Android Lint 32.4.1's `CompileTimeConstantDetector` under its Lint ID for
  Kotlin and Java, checked against the Lint CLI on the same inputs. For callees and variables declared in the same
  file it reports a non-constant argument for a `@CompileTimeConstant` parameter and a non-constant (or compound)
  assignment to a `@CompileTimeConstant` variable, using Lint's notion of a constant: literals, `const val`, final Java
  fields and declared-final locals with constant initializers, annotated parameters, constant string templates and
  concatenations, both branches of an `if`, and `listOf`/`setOf`/`emptyList`/`emptySet` of constants. It also reports
  Lint's declaration checks: non-final annotated fields, annotated public properties without `@JvmField`,
  `@field:CompileTimeConstant` constructor properties, annotated setter parameters, `@receiver:` use, `++`/`--` on an
  annotated variable, overrides that add the annotation to a same-file supertype method, and function references to
  such methods. Calls into other files or libraries, references to declarations in other files, Kotlin operator
  overloads, and methods inherited through interfaces are not checked yet (part of #1151).
- `RemoveWorkManagerInitializer` and `SpecifyForegroundServiceType`, the `androidx.work:work-runtime` 2.12.0 checks that
  join source with `AndroidManifest.xml`, ported for Kotlin and Java on the new `manifest-source` scope under their Lint
  IDs, severities and messages. Their findings match the Lint CLI running that release's `lint.jar`, including manifest
  locations and the one-finding-per-missing-type order. `@SuppressLint` and `tools:ignore` are honored (#1512, #1514).
- `BadConfigurationProvider`, `SpecifyJobSchedulerIdRange` and `WorkerHasAPublicModifier`, the project-wide checks
  bundled in `androidx.work:work-runtime` 2.12.0, ported for Kotlin and Java under their Lint IDs and checked against
  the Lint CLI running that release's `lint.jar` (`internal/rules/testdata/androidx_work/`). They collect classes and
  calls over every main-source file, as Lint's `afterCheckRootProject` does, and follow class hierarchies across files.
  Findings match Lint except that `BadConfigurationProvider`'s whole-file location is anchored at line 1.
  `SpecifyJobSchedulerIdRange` also needs evidence that the project uses WorkManager. Any `setWorkerFactory` /
  `setJobSchedulerJobIdRange` call counts as configured (#1508, #1515, #1516).
- Bazel: `krit_aspect`'s per-target actions run only file-scope rules (`--only-scope file`; `--@krit//:scope=all`
  restores every rule), so project-scope rules no longer report cross-target false positives such as DeadCode on a
  function another target calls. The new `krit_project_report` rule runs the project-scope rules (cross-file, Android
  manifest and resource rules) in one cacheable Go-only action over the main-repository sources, manifests and
  resources of its deps' transitive graph (#1593).
- Bazel: `krit_aspect`'s Krit actions run `krit bazel-worker`, a Bazel persistent worker (JSON protocol) that keeps one
  krit-fir JVM warm across the actions it serves; each action is still a fresh krit process with its own empty state
  and incremental FIR off. Re-analyzing a target with an unchanged classpath drops from about 4.6 s to 0.6 s per action
  on a 24-target benchmark (`scripts/bazel-worker-bench.sh`). Each worker holds at most one JVM, tied to the worker's
  PID through the new `KRIT_DAEMON_PARENT_PID`; without workers (`--@krit//:worker=false`, remote execution) the verb
  runs one-shot (#1591).
- Bazel: `bazel run @krit//:export_model -- //...` exports a krit model of the build graph (`krit_model_aspect`, one
  project per target: label, deps, source dirs, compile classpath, srcjars, Kotlin settings, Android inputs) into
  `.krit/bazel-model/`, which krit, `krit-lsp` and `krit-mcp` now discover like `.krit/gradle-model/`, so FIR scans
  outside Bazel get the full classpath without a hand-maintained `oracle.classpath`. Missing exported entries (after
  `bazel clean`) get a Bazel-specific warning (#1592).
- Bazel: `krit_aspect` and type-aware `krit_test` supply rules_android inputs: the Android SDK toolchain's
  `android.jar` as the boot classpath, the R class jars, and the target's own manifest and resource files, so FIR
  resolves `R` and Android SDK types and the manifest and resource rules run on rules_android and `kt_android_library`
  targets. The SDK toolchain is resolved only for Android targets, so JVM-only builds need no Android SDK. The module now
  depends on rules_android (>= 0.6.4) (#1590).
- Bazel: `krit_aspect` and type-aware `krit_test` compile rules_kotlin targets with the settings their
  `KotlinCompile` uses: the toolchain's language/API/JVM target, the target's (or toolchain's) `kotlinc_opts`, compiler
  plugins from `plugins` and deps' `exported_compiler_plugins`, and KSP/kapt generated srcjars, filtered through the
  Gradle model's Kotlin-settings allowlist. The krit module now depends on rules_kotlin (>= 2.2.0) (#1589).
- Bazel: `krit_aspect` runs krit with FIR as one cacheable, remotely executable action per Kotlin/Java target
  (`bazel build //... --aspects=@krit//:defs.bzl%krit_aspect --output_groups=krit`), against the deps' compile jars,
  writing `<target>.krit.sarif` and `.krit.txt`. `--@krit//:config`, `--@krit//:baseline`, `--@krit//:fir` and
  `--@krit//:fail_on_findings` (a validation action that fails the build on findings) configure it (#1587).
- Bazel: `krit_test(type_aware = True)` runs the FIR checker and type oracle against each target's Bazel compile
  classpath (`JavaInfo.transitive_compile_time_jars`, including the rules_kotlin stdlib), with Bazel's Java runtime
  toolchain as `JAVA_HOME`, the release's checksum-verified `krit-fir` jar (a lazily fetched repository) and
  `--fail-on-degraded`, so no hand-maintained `oracle.classpath` is needed. `KRIT_BAZEL_LOCAL_DIST` points the module
  at a local krit build (#1550).
- FIR coverage is visible without `-v`: when the FIR pass gates files on compiler errors, the scan prints `FIR analyzed
  N of M files ...` with the top causes (unresolved references grouped by import package prefix), JSON reports carry
  `summary.fir` and SARIF reports `runs[0].properties.fir` (`authoritative`, `gated`, `excluded`, `topCauses`).
  `--min-fir-coverage=0.8` exits 2 below that share, and the stale-model warning says when most of the model's classpath
  is missing (#1573).
- `--fir` splits the FIR check into one session per group of source sets that share their Kotlin compiler settings
  (compiler arguments, compiler plugin jars and options, JVM target) when a group's settings differ from the merged
  whole-repository session's. A file gated only because the merged session dropped a flag or plugin option becomes
  authoritative, and a missing opt-in the merged opt-in union hid is reported. `-v` reports the session count and the
  grouping key; `KRIT_MODULE_SHARDS=merged` keeps the single session (#1577).
- `--generated-srcjar PATH` (repeatable) and the model field `generatedSourceJars` accept generated-source jars
  (Bazel's KSP, kapt, Wire, SQLDelight and view-binding outputs). Like `generatedSourceDirs`, their Kotlin files are
  compiled as FIR checker and type-oracle context and never reported, so code using a generated Kotlin type is no
  longer gated; generated Java is not resolved by FIR yet (#1630). Each srcjar is unpacked once into
  `$KRIT_HOME/srcjars/<sha256>/` (#1588).
- `--report FORMAT=PATH`, repeatable, writes several report formats (json, sarif, checkstyle, plain) from one analysis
  run instead of one scan per format. Each file is published atomically like `-o`; with only such reports stdout gets
  no report. A bare `--report FORMAT` is still an alias for `-f` (#1578).
- Gradle: `kritCheckAll` on the root project (`dev.jasonpearson.krit.aggregate`, applied by the settings plugin) runs
  one krit scan of the whole build with every project's exported model, instead of one krit process and FIR JVM per
  project. Models reach it through variant-aware project dependencies, so it is configuration cache and Isolated
  Projects safe. `krit.advanced.moduleShards` opts the scan into module sharding (#1580).
- `ProviderReadPermissionOnly`, `UnsafeImplicitIntentLaunch` and `UnsafeIntentLaunch`, ported from Android Lint
  32.4.1's `ProviderPermissionDetector`, `UnsafeImplicitIntentDetector` and `UnsafeIntentLaunchDetector` under their
  Lint IDs, for Kotlin and Java, on the manifest-source scope. Each was checked against the Lint CLI on the same
  projects. `ProviderReadPermissionOnly` reports, at the manifest `android:readPermission`, a `<provider>` without
  `android:permission` or `android:writePermission` whose ContentProvider class implements `insert`, `update` or
  `delete` with a real body (not a lone throw, literal return, `TODO()` or `error()`). `UnsafeImplicitIntentLaunch`
  reports an implicit intent started with `startActivity` or sent with `sendBroadcast`/`sendBroadcastAsUser` in the
  same function whose constant action matches an intent filter of a non-exported activity, activity alias or
  receiver, or of a receiver registered with `RECEIVER_NOT_EXPORTED`. `UnsafeIntentLaunch` reports an untrusted intent
  (`getIntent()`/`intent`, `getParcelableExtra`, `getParcelable`, `Intent.parseUri` of a string extra, or the intent
  parameter of a component callback) that an exported, unprotected activity, service or receiver passes to a Context,
  Activity, BroadcastReceiver or PendingIntent launch method, following locals, builder chains, `let`/`also`/`apply`
  and same-file helpers up to three calls deep, with Lint's protected-broadcast `if`/`when` exemption in `onReceive`
  (#1242, #1283, #1284).
- `TileProviderPermissions` and `UnsafeNativeCodeLocation`, ported from Android Lint 32.4.1's `TileProviderDetector`
  and `UnsafeNativeCodeDetector` under their Lint IDs and checked against the Lint CLI on the same inputs.
  `TileProviderPermissions` reports a manifest `<service>` with a `BIND_TILE_PROVIDER` tiles action whose
  `android:permission` is not `com.google.android.wearable.permission.BIND_TILE_PROVIDER`, once per such action as
  Lint does. `UnsafeNativeCodeLocation` reports ELF binaries placed directly in a `res/raw*` folder or in the source
  set's `assets` folder, with Lint's separate messages for `.so` libraries and other executables. The resource cache
  fingerprint now also covers the files directly inside `assets`, so adding or editing one there invalidates cached
  resource findings (#1274, #1285).
- `InclusiveNaming`, slack-lints 0.11.1's inclusive-naming check, in the opt-in `slack-lint` rule set for Kotlin and
  Java: names (file, package, classes, functions, properties, parameters, locals, labels) containing a word of the
  new `blockList` option. Findings match the Lint CLI on `internal/rules/testdata/slack_lint/inclusive/`, apart from
  the module suffix of `internal` members' JVM names. `Deprecation` gains slack-lints' `DeprecatedCall` as a Lint
  alias (#1367, #1368).
- `JavascriptInterface` and `UnsanitizedFilenameFromContentProvider`, ported from Android Lint 32.4.1's
  `JavaScriptInterfaceDetector` and `UnsafeFilenameDetector` under their Lint IDs for Kotlin and Java, and checked
  against the Lint CLI on the same inputs. `JavascriptInterface` reports an object passed to
  `WebView.addJavascriptInterface` (targetSdk 17 or higher) whose class, superclasses and methods carry no
  `@android.webkit.JavascriptInterface`, naming the class as Lint does (`Object`, `String`, `null` for an anonymous
  object). It also reports a private or protected declaration (package-private in Java) annotated with
  `@JavascriptInterface`. The bridge class is resolved from constructor calls, `this`, string literals, anonymous
  objects, locals and declared types in the file, and from well-known platform types; other classes are a known false
  negative. `UnsanitizedFilenameFromContentProvider` follows a `Cursor` display-name column through
  `getString` and local variables within one function, including lambdas and local functions, and reports the value
  where it reaches a `java.io.File` constructor before any other call uses it.
- `BindsTypeMismatch` and `DoNotCallProviders`, the cross-file slack-lints 0.11.1 Dagger checks, in the opt-in
  `slack-lint` rule set for Kotlin and Java: a `@Binds` parameter that cannot be assigned to its return type, and a
  user-code call to a `@Binds`/`@Provides`/`@Produces` method. Findings match the Lint CLI on
  `internal/rules/testdata/slack_lint_crossfile/dagger/` (#1358, #1366).
- `AddJavascriptInterface` no longer reports "None of the methods in the added interface ...". That finding moved to
  `JavascriptInterface`, matching Lint's split between the two issues. `@Suppress`/`@SuppressLint`/`tools:ignore`
  entries and baseline entries written for `AddJavascriptInterface` still silence the moved finding: a rule's
  `Aliases` (its former IDs) now also match baseline entries (#1204, #1286).
- `ProguardAndroidTxtUsage`, `NotShrinkingResources`, `KaptUsageInsteadOfKsp` and
  `LifecycleAnnotationProcessorWithJava8`, ported from Android Lint 32.4.1's `ProguardAndroidTxtDetector`,
  `R8GradleConfigDetector` and `GradleDetector` under their Lint IDs. Findings were checked against those detectors
  through Lint's own test harness (`TestLintTask` with its mocked Gradle model) on the same build scripts. The R8
  checks skip library modules and use the Lint command line's severities: a warning for `NotShrinkingResources`,
  and for `ProguardAndroidTxtUsage` a warning before AGP 9 and an error from AGP 9. Both have semantic fixes
  (`proguard-android-optimize.txt`, `shrinkResources = true`). `KaptUsageInsteadOfKsp` keeps Lint's KSP table
  (Room, Glide, Moshi, Epoxy, ...) and only the exact `kapt` configuration, so it is narrower than krit's
  `KaptUsage`, which stays as it is. `LifecycleAnnotationProcessorWithJava8` needs Java 8 or later from
  `sourceCompatibility`, a Java toolchain, or the AGP 4.2+ default (#1179, #1181, #1240, #1246).
- `WearableBindListener` and `DuplicateStrings`, ported from Android Lint 32.4.1's `ManifestDetector` and
  `StringCasingDetector` under their Lint IDs. They were checked against Lint on the same inputs: lint-tests'
  `TestLintTask` with a mocked Gradle model for the manifest check, and the Lint CLI for the resource check.
  `WearableBindListener` reports the BIND_LISTENER action of a `<service>` in a Gradle project. It says "deprecated"
  when `play-services-wearable` 8.2.0 or later is declared, and "upgrade" from compileSdk 24 otherwise.
  `DuplicateStrings` (opt-in, as in Lint) groups each locale's strings by case-insensitive value. It skips
  untranslatable, marked-up and empty strings, and reports Lint's message (sorted names, case-varies note) at the
  first definition. The resource index now records each `<string>`'s tag position, `translatable` value and whether it
  has child elements (resource cache version 21) (#1211, #1269).
- `MutableImplicitPendingIntent`, ported from Android Lint 32.4.1's `PendingIntentMutableImplicitDetector` under its
  Lint ID for Kotlin and Java, and checked against the Lint CLI on the same inputs. It reports
  `PendingIntent.getActivity`/`getActivities`/`getBroadcast`/`getService` calls whose constant flags include
  `FLAG_MUTABLE` but neither `FLAG_NO_CREATE` nor `FLAG_ALLOW_UNSAFE_IMPLICIT_INTENT`, when the intent is proven
  implicit: built with an implicit `Intent` constructor (including copies), with no `setPackage`, `setComponent`,
  `setClass` or `setClassName` (or `package`/`component` property) applied in a call chain, an `apply`/`also` block, or
  on a local variable before the call. Intents that reach other code first, and intents of unknown origin, are not
  reported, as in Lint. Below targetSdk 23 nothing is reported; below 34 the finding is a warning about Android 14
  (#1232).
- `AnimatorKeep`, ported from Android Lint 32.4.1's `ObjectAnimatorDetector` under its Lint ID for Kotlin and Java,
  as a project-scope check over the parsed files. It reports the setter an `ObjectAnimator.ofInt/ofFloat/ofArgb/
  ofMulti*/ofObject` (or a `PropertyValuesHolder` passed to `ofPropertyValuesHolder`) reaches by its string property
  name. That is a Java method, a Kotlin `fun setX`, or a Kotlin `var`, in the project's sources, with the expected
  parameter type and without `@Keep` on the setter (`@set:Keep` for a property) or an enclosing class. It reports
  once per setter. It only checks modules that shrink: the project profile now records `minifyEnabled true` /
  `isMinifyEnabled = true` (profile cache version 4). It is opt-in, because Lint also accepts a ProGuard/R8 keep rule,
  which krit does not read. It was checked against the Lint CLI, which matches same-file setters exactly. For setters
  in another file, Lint reports the same setters at positions computed against the wrong file; krit reports them at
  their declarations (#1227).
- `UseRequireInsteadOfGet`, `UseGetLayoutInflater`, `DialogFragmentCallbacksDetector` and
  `DetachAndAttachSameFragment`, ported from the Lint checks bundled in `androidx.fragment` 1.9.1 for Kotlin and Java
  and checked against that AAR's `lint.jar` through the Lint CLI on the same sources, with Lint's rewritten
  suggestions in the messages. `UseRequireInsteadOfGet` does not report a parameter that shadows a Fragment getter,
  which Lint does (#1446, #1447, #1448, #1450).

### Added
- `IdleBatteryChargingConstraints`, `InvalidPeriodicWorkRequestInterval`, `BadPeriodicWorkRequestEnqueue` and
  `UseRxSetProgress2`, ported from the Lint checks bundled in `androidx.work:work-runtime` 2.12.0 for Kotlin and Java
  under their Lint IDs, severities and messages. They are checked against the Lint CLI 9.4 running that release's
  `lint.jar` on the same inputs (`internal/rules/testdata/androidx_work/`). `IdleBatteryChargingConstraints` and
  `UseRxSetProgress2` match Lint's findings exactly. `BadPeriodicWorkRequestEnqueue` reports at the `enqueue()` call,
  where Lint reports inside the library jar. It skips Lint's `List<WorkRequest>` match on one-time requests.
  `InvalidPeriodicWorkRequestInterval` follows the detector source, because the shipped detector reported nothing
  against compiled WorkManager. Receivers are proven from imports, constructors, builder chains and same-file
  declarations. The checks skip test sources, and projects whose dependency list proves androidx.work absent.
  `UseRxSetProgress2` renames a statement call to `setCompletableProgress` (#1509, #1510, #1511, #1513).
- `FragmentLiveDataObserve`, `FragmentBackPressedCallback`, `FragmentAddMenuProvider` and
  `UnsafeRepeatOnLifecycleDetector`, ported from the Lint checks bundled in `androidx.fragment` 1.9.1 for Kotlin and
  Java and checked against that AAR's `lint.jar` through the Lint CLI on the same sources. Like Lint, the first three
  follow same-file helpers called from a Fragment's view lifecycle methods. Without the type oracle a LiveData whose
  type is only declared in another file (a ViewModel property) is not proven, a known false negative
  (#1443, #1444, #1445, #1449).
- `TestManifestGradleConfiguration`, ported from `androidx.compose.ui:ui-test-manifest` 1.12.1 under its Lint ID and
  checked against the Lint CLI on the same Groovy build file. It reports a bare-string
  `androidx.compose.ui:ui-test-manifest` dependency on any configuration but `debugImplementation`, using Lint's
  name test (so `testImplementation`, `androidTestImplementation` and an `ext` assignment named like a configuration
  are reported, while `releaseImplementation` and `testDebugImplementation` are not), and its fix renames the
  configuration to `debugImplementation`. Its ID is the Lint issue ID, so `lint.xml` and Gradle `lint { }` severities
  apply (#1503).

### Added
- Rule scope `manifest-source` for checks that join Kotlin/Java source with `AndroidManifest.xml`. A rule declaring
  `NeedsManifest` together with `NeedsParsedFiles` runs once per project in the Android phase. It sees every parsed
  project manifest (`ctx.Manifests`) and every Kotlin and Java file (`ctx.ParsedFiles`), and may report into either.
  Source findings honor `@Suppress`/`@SuppressLint`. Manifest findings honor `tools:ignore`. Results are cached on the
  content of every manifest plus the whole source set, so editing either side reruns the rules on CLI, warm and daemon
  runs.
- `NotificationTrampoline` and `LaunchActivityFromNotification`, ported from Android Lint 32.4.1's
  `NotificationTrampolineDetector` under their Lint IDs for Kotlin and Java, as a project-scope check over the
  parsed files. They were checked against the Lint CLI on the same sources
  (`internal/rules/testdata/android_lint_performance/`). `setContentIntent`, `setFullScreenIntent` and `addAction`
  on a proven `androidx.core.app.NotificationCompat.Builder` (explicit, chained, or the receiver of
  `apply`/`run`/`with`) are traced to a `PendingIntent.getBroadcast`/`getService` and its `Intent` through the last
  assignment of locals, and through same-class Java helpers. As in Lint, only Java receiver and service classes are
  inspected for `startActivity`. A Kotlin one is not found, which yields `LaunchActivityFromNotification` instead of
  `NotificationTrampoline` for `setContentIntent`/`setFullScreenIntent`, and no finding for `addAction`
  (#1225, #1226).
- `DuplicateDivider` and `SyntheticAccessor`, ported from Android Lint 32.4.1's `ItemDecoratorDetector` and
  `SyntheticAccessorDetector` under their Lint IDs for Kotlin and Java. They were checked against the Lint CLI on the
  same sources (`internal/rules/testdata/android_lint_performance_files/`, with pinned known false negatives) and
  compared with Lint on 24 corpus files. `DuplicateDivider` reports a support-library `RecyclerView.ItemDecoration`
  subclass named `DividerItemDecoration` with `HORIZONTAL_LIST` and `VERTICAL_LIST` fields; as in Lint, the androidx
  class is not matched. `SyntheticAccessor` (opt-in, as in Lint) resolves references to private members lexically
  within the file and reports those made from another class. Like Lint it skips Kotlin properties, constants,
  companion callers, sealed and value-class constructors. Past a superclass declared elsewhere it only resolves
  distinctive names (#1202, #1271).
- `UnnecessaryArrayInit`, `ExpensiveAssertion` and `AssertionSideEffect`, ported from Android Lint 32.4.1's
  `ArrayInitDetector` and `AssertDetector` under their Lint IDs and checked against the Lint CLI on the same sources.
  They follow the detectors' UAST walks, including the depth limits and Lint's quirks: qualified calls are cheap for
  `ExpensiveAssertion`, and `kotlin.collections` members are not side effects. Calls are followed into functions of
  the same file only. A call to a function declared elsewhere, or a mutating library call whose receiver type is not
  proven to be a JDK or Android class, is not reported. `AssertionSideEffect` also checks Java `assert` statements.
  `ExpensiveAssertion` is opt-in, as in Lint. `UnnecessaryArrayInit` removes the lambda as an idiomatic fix (#1140,
  #1141, #1142).
- `BidiSpoofing`, `AuthLeak` and `SecretInSource`, ported from Android Lint 32.4.1's `BidirectionalTextDetector`,
  `StringAuthLeakDetector` and `SecretDetector` under their Lint IDs, for Kotlin and Java. Messages, locations and
  severities were checked against the Lint CLI on the same inputs. `BidiSpoofing` reports comments (but not KDoc or
  Javadoc) and string literal entries whose source text leaves a bidirectional embedding, override or isolate open
  before a line break. `AuthLeak` reports literals of at most 512 characters holding `scheme://user:password@host`,
  skipping a `%s`-style password. Like Lint, both look at each Kotlin template entry separately: interpolations,
  escape sequences and raw-string line breaks split entries. `SecretInSource` reports a `GenerativeModel` API key
  argument that is an `AIza...` literal longer than 30 characters, either written inline or read from a local
  variable or same-file field initializer. A key held in a field in another file is a known false negative (#1145,
  #1263, #1268).
- `ComposableNaming`, `ComposableLambdaParameterNaming`, `ComposableLambdaParameterPosition` and
  `CompositionLocalNaming`, ported from the Lint checks in `androidx.compose.runtime:runtime-android` 1.12.1 under
  their Lint IDs and checked against the Lint CLI running that release's `lint.jar` on the same inputs
  (`internal/rules/testdata/androidx_compose_runtime/`). They match Lint's messages, locations, severities and
  defaults: the two lambda-parameter checks are opt-in, as in Lint, and test sources are analyzed. Local functions
  and overrides are skipped as Lint skips them. Shadowing member or local functions and same-file declarations are
  respected. `ComposableNaming` and `CompositionLocalNaming` also check Java. An expression-bodied `@Composable`
  function without a declared return type is only checked when its body is `Unit` or a call to a same-file function;
  otherwise its return type needs type inference, and it is a known false negative. Lint's rename quick fixes are
  not ported, because they leave call sites unchanged (#1467, #1468, #1469, #1471).
- `CoroutineCreationDuringComposition`, `FlowOperatorInvokedInComposition` and `StateFlowValueCalledInComposition`,
  ported from `androidx.compose.runtime:runtime-android` 1.12.1 under their Lint IDs and checked against the Lint CLI
  on the same inputs. They report `launch`/`async`/`launchIn`, Flow operators (`map`, `filter`, `stateIn`, ...,
  and same-file `Flow` extensions returning a `Flow`) and `StateFlow.value` reads and writes made in composition. A
  shared composition-context walk mirrors Lint's `isInvokedWithinComposable`: it sees through local properties,
  anonymous objects and lambdas inlined into their caller (same-file `inline` functions, `let`/`run`/`apply`/...),
  and stops at functions, accessors and other lambdas. A lambda counts as composition when it is annotated or typed
  `@Composable`, when it is passed to a same-file parameter typed `@Composable`, or when it is passed to a known
  Compose content parameter (`Column`, `Box`, `Scaffold` slots, `CompositionLocalProvider`, `key`, lazy `item`/`items`,
  ...). Receivers must be proven to be a `CoroutineScope`, `Flow` or `StateFlow` by same-file declarations or
  initializers, which rules out lookalike members such as `ActivityResultLauncher.launch` (#1465, #1466, #1470).
- `AutoboxingStateValueProperty`, `AutoboxingStateCreation` and `MutableCollectionMutableState`, ported from
  `androidx.compose.runtime:runtime-android` 1.12.1 under their Lint IDs and checked against the Lint CLI on the same
  inputs. `AutoboxingStateValueProperty` reports reads and plain assignments of `value` on a proven
  `IntState`/`LongState`/`FloatState`/`DoubleState` (or the `Mutable` variants, including `remember { mutableIntStateOf(0) }`
  and `asIntState()`), with an idiomatic autofix to `intValue`/`longValue`/`floatValue`/`doubleValue`; like Lint it
  skips compound assignments (`+=`). `AutoboxingStateCreation` (informational) reports `mutableStateOf` of a non-null
  `Int`, `Long`, `Float` or `Double` with the default policy. `MutableCollectionMutableState` reports `mutableStateOf`
  holding a mutable collection (`mutableListOf`, `ArrayList()`, `java.util` collections, `toMutableList()`, mutable
  collection parameters and same-file subclasses). Both take the state type from an explicit type argument, the
  declared type of the property or delegate it initializes, or the value argument when nothing else constrains it
  (#1463, #1464, #1473).
- `RememberReturnType`, `OpaqueUnitKey`, `ProduceStateDoesNotAssignValue` (from `runtime-android` 1.12.1) and
  `RememberSaveableSaverParameter` (from `runtime-saveable-android` 1.12.1), ported under their Lint IDs and checked
  against the Lint CLI on the same inputs. `RememberReturnType` reports `remember` calls whose calculation returns
  `Unit` (`remember<Unit>`, an empty or Unit-valued calculation, or a `() -> Unit` value). `OpaqueUnitKey` reports keys
  of `remember`, `LaunchedEffect`, `DisposableEffect`, `produceState`, `ReusableContent` and `key` that always evaluate
  to `Unit`, naming the parameter (`key1`, `key2`, `keys`, `key`) as Lint does. `ProduceStateDoesNotAssignValue`
  reports producers that never assign `value`; calls that might receive the producer scope or a `MutableState` count
  as assigning. `RememberSaveableSaverParameter` reports a `Saver` passed as the only vararg `inputs` argument, with a
  semantic autofix that names it `saver =` (or `stateSaver =` when the calculation returns a `MutableState`)
  (#1474, #1475, #1477, #1483).
- `RetainUnitType`, `RetainRememberObserver`, `RetainingDoNotRetainType` and `RetainLeaksContext`, ported from
  `androidx.compose.runtime:runtime-retain-android` 1.12.1 under their Lint IDs and checked against the Lint CLI on the
  same inputs. For each `retain` call they report a calculation that returns `Unit`, a retained value implementing
  `RememberObserver` but not `RetainObserver`, a retained value or key whose class (or direct supertype) is
  `@DoNotRetain` (with the annotation's explanation), and a retained value or key that is a `Context`, `View`, `Window`,
  `Resources`, `Fragment` or `Dialog` (including `LocalContext.current` and `LocalView.current`), with Lint's messages
  and canonical type names. Retained containers such as `listOf<Context>()` are not checked yet (#1479, #1480, #1481,
  #1482).
- `UseAndroidAlpha`, `UseAppTint`, `UseSwitchCompatOrMaterialXml`, `UseCompatTextViewDrawableXml` and
  `UsingOnClickInXml`, ported from the Lint checks bundled in `androidx.appcompat:appcompat` 1.8.0 and checked
  against that AAR's `lint.jar` through the Lint CLI on the same files. They run only when the project declares
  appcompat (or Material, which brings it), as Lint only loads them from that AAR. `UsingOnClickInXml` abstains when
  minSdk cannot be read from the build, where Lint would assume API 1 (#1320, #1323, #1325, #1327, #1328).
- `FragmentGradleConfiguration` and `FragmentTagUsage` (from the Lint checks bundled in `androidx.fragment` 1.9.1) and
  `InvalidFragmentVersionForActivityResult` and `InvalidUseOfOnBackPressed` (from `androidx.activity:activity` 1.13.0),
  checked against those AARs' `lint.jar` through the Lint CLI on the same files. `InvalidFragmentVersionForActivityResult`
  reads the module's declared and version-catalog `androidx.fragment:fragment` version; a version that arrives only
  transitively is not seen yet (#1120). `InvalidUseOfOnBackPressed` keeps Lint's verdicts, which only match a method
  named `handledOnBackPressed` (#1305, #1306, #1441, #1442).
- `UseSupportActionBar`, `UseCompatLoadingForColorStateLists`, `UseCompatLoadingForDrawables`,
  `UseCompatTextViewDrawableApis` and `UseSwitchCompatOrMaterialCode`, ported from the Lint checks bundled in
  `androidx.appcompat:appcompat` 1.8.0 for Kotlin and Java and checked against that AAR's `lint.jar` through the Lint
  CLI on the same sources. They run only when the project declares appcompat (or Material). Receivers are proven from
  declarations, imports, the resolver hierarchy and Java facts; an implicit receiver inside `apply`/`run`/`with` whose
  subject is not proven abstains (#1319, #1321, #1322, #1324, #1326).
- `UnrememberedMutableState`, `RememberInComposition` and `FrequentlyChangingValue`, ported from
  `androidx.compose.runtime:runtime-android` 1.12.1 under their Lint IDs and checked against the Lint CLI on the same
  inputs. `UnrememberedMutableState` reports state factories (`mutableStateOf`, the primitive variants,
  `mutableStateListOf`/`MapOf`/`SetOf`, `derivedStateOf`, same-file `@StateFactoryMarker` functions), constructors of
  same-file `State` classes and `object : MutableState` literals created in composition outside `remember { }`.
  `RememberInComposition` does the same for `@RememberInComposition` APIs: same-file declarations and the annotated
  Compose 1.12.1 / material3 1.5.0 APIs (`Animatable`, `MutableInteractionSource`, `FocusRequester`,
  `BringIntoViewRequester`, `SliderState`, `movableContentOf`, ...). `FrequentlyChangingValue` reports reads of
  `@FrequentlyChangingValue` values in composition (same-file declarations, and `LazyListState`/`LazyGridState`
  `firstVisibleItemIndex`/`firstVisibleItemScrollOffset`/`layoutInfo`, `ScrollState.value`,
  `PagerState.currentPageOffsetFraction`, `AnchoredDraggableState` offsets and similar on proven receivers). The
  retired Lint IDs `UnrememberedAnimatable`, `UnrememberedMutableInteractionSource` and
  `FrequentlyChangedStateReadInComposition` still suppress the matching rule (#1472, #1476, #1478).

### Changed
- The Bazel module registers krit's release binaries as toolchains (`@krit//toolchains:toolchain_type`), one per
  execution platform, instead of downloading the host's archive into `@krit_bin`. `krit_aspect` and
  `krit_project_report` resolve it for the platform that runs their actions, so remote execution on another OS or
  architecture runs the right binary, and `krit_toolchain` lets a root module register its own build. `@krit//:krit`
  and `krit_test` take the binary for the target platform. Only the archives of the platforms Bazel picks are
  fetched. The `krit_bin` repository is gone: register a `krit_toolchain`, or override `krit_toolchains`, instead. The BCR presubmit runs on Debian 13
  instead of the end-of-life Debian 11.
- `publish-bcr.yml` authenticates with a fine-grained token limited to the `kaeawc/bazel-central-registry` fork instead
  of a classic token whose `repo` scope reached every repository. Fine-grained tokens cannot open pull requests
  against `bazelbuild/bazel-central-registry`, so the run pushes the entry to the fork and prints a link; a maintainer
  opens the pull request as a draft and marks it ready, which BCR takes as their approval. The workflow previously
  opened non-draft pull requests, which a maintainer could not approve as their own.
- `krit_aspect`'s persistent worker keeps one warm krit-fir JVM for every target with the same compiler settings
  instead of one per classpath: its scans share a krit-fir daemon pool slot, and the daemon rebuilds its session for
  each action's sources and classpath. On a 24-target benchmark the Krit actions of a full build went from 79.9 s to
  7.0 s (mean action 6.56 s to 0.54 s), with the same reports as one-shot JVMs (#1666).
- The `--fir` duplicate-declaration recheck compiles only the source sets that hold a gated or crashed file and checks
  only those files, the rest of each set compiling as context, instead of every scanned set of each owning project.
  Its worker JVMs are balanced by the files each set compiles, unbuilt dependency sources included (#1661).
- Module workers compile several modules at a time on threads of one JVM instead of one JVM per concurrent module
  (#1656): they share the JIT-compiled compiler and heap. On a 4-core machine the `--fir` source-set recheck of 24
  sets went from about 32s to 25s with the same results; module sharding, PR mode and settings-group sessions use the
  same workers. `KRIT_WORKER_THREADS` caps the threads per JVM (default 4; `1` restores a JVM per module). Each
  concurrent compile keeps the heap a JVM per module had, and an out-of-memory error in one stops that JVM's
  remaining modules.
- `DoNotMockAnything` runs only when named (`--enable-rules`, `krit.yml`): `--experimental` no longer turns it on, and
  `--strict` leaves it off. As in Lint, it is too broad to ride a blanket opt-in.
- The Gradle plugins `dev.jasonpearson.krit` and `dev.jasonpearson.krit.settings` declare configuration-cache
  support on the Gradle Plugin Portal. `kritCheck`, `kritFormat` and `kritBaseline` now resolve (and download) the
  krit binary when they run instead of while Gradle builds the task graph, so `--dry-run` no longer downloads it and
  the build after a first download reuses the configuration cache. Their up-to-date input is the tool version
  rather than the binary's contents; an explicit `advanced.binary` is still tracked as a file. `kritCheck` no longer
  reads `Task.project` while it runs, which broke configuration-cache builds with every report disabled. These
  tasks and the settings plugin's builds are now tested to store and reuse the configuration cache.
- `ComposeMutableStateInComposition` is retired: `UnrememberedMutableState` (the Lint port) reports everything it did,
  plus state created in any expression position, `mutableStateSetOf`, `derivedStateOf`, `@StateFactoryMarker`
  functions and `State` subclasses. The old ID keeps working: `@Suppress("ComposeMutableStateInComposition")`
  suppresses the new rule, a `compose: ComposeMutableStateInComposition:` block in `krit.yml` configures it (with a
  validation warning naming the new ID) when the new rule has no block of its own, and baseline entries recorded under
  the old ID silence one `UnrememberedMutableState` finding each in the same file. **Severity change:** these findings
  are now errors (the old rule reported warnings), matching Lint; a `severity` set under the old ID still applies
  (#1478).
  `FrequentlyChangedStateReadInComposition` still suppress the matching rule. Until `ComposeMutableStateInComposition`
  is retired, `UnrememberedMutableState` leaves the plain `val x = mutableStateOf(...)` locals that rule already
  reports to it (#1472, #1476, #1478).
- `MissingColorAlphaChannel` and `InvalidColorHexValue` (from `androidx.compose.ui:ui-graphics-android` 1.12.1) and
  `InvalidLanguageTagDelimiter` (from `androidx.compose.ui:ui-text-android` 1.12.1), ported under their Lint IDs and
  checked against the Lint CLI running those releases' `lint.jar` on the same inputs
  (`internal/rules/testdata/androidx_compose_ui/`). The Color checks count the hex digits of a literal passed to
  Compose `Color(...)` as Lint does; `MissingColorAlphaChannel` adds the `FF` alpha channel as a fix.
  `InvalidLanguageTagDelimiter` evaluates the tag passed to Compose `Locale`/`LocaleList` (literals, `+`, same-file
  vals and constants), checks Java too, and fixes a literal tag by replacing `_` with `-` (#1501, #1502, #1504).
- A `slack-lint` rule set with ports of slack-lints' `MoshiUsageDetector` (0.11.1), starting with its five enum
  checks: `MoshiUsageEnumMissingJsonClass`, `MoshiUsageEnumJsonClassGenerated`, `MoshiUsageEnumMissingUnknown`,
  `MoshiUsageEnumAnnotatedUnknown` and `MoshiUsageEnumCasing`. Rule IDs are the Lint IDs, so `@SuppressLint`,
  `tools:ignore` and `lint.xml` entries carry over. They are experimental and off by default; enable them by ID or
  with `--experimental`. One shared project pass serves every `MoshiUsage*` rule. A new `api.Rule.LintIssue` flag
  lets rules outside `android-lint` whose IDs are Lint issue IDs honor `lint.xml` and Gradle `lint { disable }`
  (#1391, #1392, #1393, #1394, #1397).
- `DenyListedApi` and `DenyListedBlockingApi`, slack-lints 0.11.1's `DenyListedApiDetector`, in the opt-in
  `slack-lint` rule set for Kotlin and Java. They report the detector's built-in list (RxJava 3 `hide()` and blocking
  calls, `java.util.Date`/`Calendar` and `java.text.DateFormat` APIs, `LinkedList`/`Stack`/`Vector`, `runCatching`,
  `Flow.launchIn`, dispatcher-less rx3 builders and more) once the owner is proven from imports or the receiver's type.
  Findings match the Lint CLI on `internal/rules/testdata/slack_lint/denylist/`, except that a top-level
  `runCatching` of the file's package shadows kotlin's (#1428, #1429).
- `MultipleAwaitPointerEventScopes` and `ReturnFromAwaitPointerEventScope`, ported from `androidx.compose.ui:ui-android`
  1.12.1 under their Lint IDs and checked against the Lint CLI on the same inputs. The first counts
  `awaitPointerEventScope` calls within the enclosing `pointerInput` call (its own arguments, not the chain before it)
  or declaration. The second reports an unqualified call that initializes a local or is returned (`return`, an
  expression body, or a lambda's last expression) outside `pointerInput`, `PointerInputEventHandler`, `coroutineScope`
  or a lambda declared `suspend PointerInputScope.() -> ...`; as in Lint, a qualified `scope.awaitPointerEventScope`
  is never reported (#1497, #1498).
- `DoNotCallViewToString` and `InflationInItemDecoration`, slack-lints 0.11.1's View checks, in the opt-in
  `slack-lint` rule set for Kotlin and Java: a `toString()` call resolving to a View subclass's override, and a
  `LayoutInflater` or ViewBinding `inflate()` inside a `RecyclerView.ItemDecoration`. Findings match the Lint CLI on
  `internal/rules/testdata/slack_lint/view_checks/` (#1435, #1436).
- `ModifierFactoryExtensionFunction`, `ModifierFactoryReturnType`, `ModifierFactoryUnreferencedReceiver` and
  `ModifierParameter`, ported from `androidx.compose.ui:ui-android` 1.12.1 under their Lint IDs and checked against the
  Lint CLI on the same inputs. A declaration is checked when its declared type, or an expression body rooted at
  `Modifier` (or a same-file Modifier subtype constructor or object literal), proves it returns a Modifier; members,
  interface functions and overrides are checked as in Lint, local functions and class properties are not, and
  `ParentDataModifier` factories are exempt. The receiver counts as used through `this` or any unqualified call that
  may bind it. Lint reports every extension-property getter as not using its receiver; krit only reports getters that
  really do not. An expression-bodied `@Composable` without a declared return type is a known `ModifierParameter`
  false negative unless its body proves `Unit`. The Compose UI ports, `MissingColorAlphaChannel`,
  `InvalidColorHexValue` and `InvalidLanguageTagDelimiter` included, are marked as Lint issues, so `lint.xml` and
  Gradle `lint { }` severities apply to them (#1492, #1493, #1494, #1496).
- `UnnecessaryComposedModifier`, `SuspiciousModifierThen`, `ModifierNodeInspectableProperties` and
  `SuspiciousCompositionLocalModifierRead`, ported from `androidx.compose.ui:ui-android` 1.12.1 under their Lint IDs
  and checked against the Lint CLI on the same inputs and on android/snippets' Modifier samples.
  `UnnecessaryComposedModifier` reports a `composed` factory that provably calls no `@Composable` function (it abstains
  on calls to functions declared in other files and on `.current` reads). `SuspiciousModifierThen` reports
  implicit-receiver Modifier factories inside `then(...)`. `ModifierNodeInspectableProperties` (informational, Kotlin
  and Java) follows same-file superclasses to `ModifierNodeElement`. `SuspiciousCompositionLocalModifierRead` reports
  `currentValueOf` in `onAttach`/`onDetach`, `init` blocks and secondary constructors of a direct
  `CompositionLocalConsumerModifierNode`, and inside any `lazy { }` (#1486, #1495, #1499, #1500).
- `LocalContextConfigurationRead`, `LocalContextGetResourceValueCall`, `LocalContextResourcesRead`,
  `ConfigurationScreenWidthHeight` and `NonObservableLocale`, ported from `androidx.compose.ui:ui-android` 1.12.1 under
  their Lint IDs and checked against the Lint CLI on the same inputs. The `LocalContext` checks follow
  `LocalContext.current` and its `resources` through same-file variables, and a `resources` read that feeds a
  `configuration` read is reported once, as in Lint. `ConfigurationScreenWidthHeight` runs in the manifest-source
  scope so that, like Lint, it skips modules whose manifest declares `android.hardware.type.watch` (and files that
  import `androidx.wear`). `NonObservableLocale` reports `Locale.getDefault()`, `LocaleList.getAdjustedDefault()` and
  `LocaleListCompat.getAdjustedDefault()` in composition; reads inside stdlib inline lambdas other than the scope
  functions (such as `forEach`) are a known false negative (#1487, #1488, #1489, #1490, #1491).
- `slack-lint` gains `MoshiUsageDetector`'s class-level `@JsonClass` checks for Kotlin: `MoshiUsageDoubleClassAnnotation`,
  `MoshiUsageClassVisibility`, `MoshiUsageBlankGenerator`, `MoshiUsageGenerateAdapterShouldBeTrue`, `MoshiUsageObject`,
  `MoshiUsageUnsupportedType`, `MoshiUsageUseData` and `MoshiUsageMissingPrimary` (#1385, #1388, #1398, #1402, #1410,
  #1419, #1420, #1422).
- `AvoidUsingNotNullOperator`, `ExceptionMessage`, `NullableConcurrentHashMap`, `GuavaPreconditionsUsedInKotlin` and
  `ParcelizeFunctionProperty`, five slack-lints 0.11.1 checks, in the opt-in `slack-lint` rule set. Their findings match
  the Lint CLI running slack-lint-checks 0.11.1 on `internal/rules/testdata/slack_lint/kotlin_checks/`, except that a
  nullable type argument of a `val` constructor parameter is reported once, where Lint reports it twice
  (#1374, #1430, #1431, #1434, #1437).
- `slack-lint` gains `MoshiUsageDetector`'s moshi-sealed checks for Kotlin: `MoshiUsageBlankTypeLabel`,
  `MoshiUsageSealedMustBeSealed`, `MoshiUsageDoubleTypeLabel`, `MoshiUsageGenericSealedSubtype`,
  `MoshiUsageMissingTypeLabel` and `MoshiUsageInappropriateTypeLabel`. Subtypes are matched to a sealed parent in the
  same package across files, including files that never mention Moshi (#1387, #1389, #1399, #1400, #1403, #1415).
- `SubscribeOnMain`, `RxObservableDoesNotEmit` and `RxFlowableDoesNotEmit`, slack-lints' Rx checks, in the opt-in
  `slack-lint` rule set. `SubscribeOnMain` (Kotlin and Java) matches the Lint CLI running slack-lint-checks 0.11.1 with
  RxJava 3 on the classpath (`internal/rules/testdata/slack_lint/rx/`) and needs the file to import RxJava 3 and none of
  RxJava 1/2 or Reactor. The two emit checks follow slack-lints main: 0.11.1 looks for a nonexistent
  `kotlin.coroutines.ProducerScope` and so never reports on kotlinx-coroutines-rx code (#1371, #1372, #1373).
- `slack-lint` gains `MoshiUsageDetector`'s primary-constructor checks for Kotlin: `MoshiUsagePrivateConstructor`,
  `MoshiUsagePrivateConstructorProperty`, `MoshiUsageVarProperty`, `MoshiUsageParamNeedsInit` and
  `MoshiUsageTransientNeedsInit`. Unlike Lint 9.4, which no longer sees `@Transient` on a constructor property,
  krit reports a `@Transient` property without a default, as slack-lints' own tests expect (#1411, #1412, #1413,
  #1418, #1421).
- `DoNotMockSealedClass`, `DoNotMockAutoValue` and `DoNotMockRecordClass`, the rest of slack-lints' `MockDetector`
  checks, matched against slack-lints' detector tests like the first four. The factories also resolve under an import
  alias, and a well-known platform class named through an on-demand import of its package is a platform type
  (#1378, #1379, #1381).
- `slack-lint` gains `MoshiUsageDetector`'s JSON name checks, on Kotlin constructor properties and on Kotlin and
  Java enum members: `MoshiUsageBlankJsonName`, `MoshiUsageRedundantJsonName`, `MoshiUsageDuplicateJsonName`,
  `MoshiUsageSerializedName`, `MoshiUsageRedundantSiteTarget` and `MoshiUsageSnakeCase`. Unlike Lint 9.4, krit
  reports `@SerializedName` on a constructor property and does not report `@Json(ignore = true)` members as
  duplicates of Moshi's unset name (#1386, #1390, #1401, #1414, #1416, #1417).
- `CastingViewContextToActivity`, `ArgInFormattedQuantityStringRes` and `SpanMarkPointMissingMask`, slack-lints 0.11.1's
  Android source checks, in the opt-in `slack-lint` rule set for Kotlin and Java. Findings match the Lint CLI with
  `android.jar` on the classpath (`internal/rules/testdata/slack_lint/android_ui/`), except that a mask inside
  parentheses, `(flags & Spanned.SPAN_POINT_MARK_MASK) == ...` (the only form Java allows), is not reported
  (#1356, #1357, #1424).
- `slack-lint` gains `MoshiUsageAdaptedByRequiresAdapter` and `MoshiUsageAdaptedByRequiresKeep` for moshix
  `@AdaptedBy` adapters declared in the project (Kotlin or Java, followed through project base classes), and
  `MoshiUsageQualifierRetention` and `MoshiUsageQualifierTarget` for Kotlin and Java `@JsonQualifier` annotation
  classes (#1382, #1383, #1405, #1406).
- `FullyQualifiedResource`, `MissingResourceImportAlias` and `WrongResourceImportAlias`, slack-lints 0.11.1's R class
  import-alias checks for Kotlin, in the opt-in `slack-lint` rule set. Their `importAliases` option takes the
  `package.R as Alias` entries of Lint's `import-aliases`. Findings match the Lint CLI with that option set in
  `lint.xml` (`internal/rules/testdata/slack_lint/resource_aliases/`) (#1425, #1426, #1427).
- `slack-lint` gains `MoshiUsageDetector`'s constructor-property type checks for Kotlin: `MoshiUsageArray`,
  `MoshiUsageMutableCollections`, `MoshiUsageNonMoshiClassCollection`, `MoshiUsageNonMoshiClassMap`,
  `MoshiUsageNonMoshiClassPlatform`, `MoshiUsageEnumPropertyCouldBeMoshi` and `MoshiUsageEnumPropertyDefaultUnknown`.
  Type arguments are checked where Kotlin passes them to Java without a wildcard, as Lint sees them; types that
  cannot be classified from source (star imports, typealiases, unresolved annotations that may be qualifiers) are
  skipped. This completes the `MoshiUsageDetector` port except for Slack's package-specific
  `MoshiUsageNonMoshiClassInternal`/`External` (#1384, #1395, #1396, #1404, #1407, #1408, #1409).
- `TestParameterSiteTarget`, `MustUseNamedParams` and `RestrictCallsTo`, slack-lints 0.11.1's annotation-driven
  checks, in the opt-in `slack-lint` rule set. `MustUseNamedParams` and `RestrictCallsTo` resolve Kotlin calls to
  project functions and constructors annotated with slack-lint-annotations' `@MustUseNamedParams`/`@RestrictCallsTo`
  through imports, packages and receiver types (overrides included), and match the Lint CLI on
  `internal/rules/testdata/slack_lint_crossfile/annotations/` (#1423, #1432, #1433).
- `JavaOnlyDetector`, slack-lints 0.11.1's `@JavaOnly`/`@KotlinOnly` check, in the opt-in `slack-lint` rule set: in
  Kotlin sources, a class or function annotated with both, a subclass or override of an annotated type or function
  that does not repeat the annotation, an anonymous object or SAM lambda of a `@JavaOnly` type, and calls and
  callable references to `@JavaOnly` functions or members of `@JavaOnly` classes, with Java declarations indexed.
  Findings match the Lint CLI on `internal/rules/testdata/slack_lint_crossfile/javaonly/`, except that a finding on
  a class is reported at the declaration rather than its KDoc (#1369).

### Fixed
- Manifest rules resolve plugin aliases (`alias(<catalog>.plugins.x)`) against the catalogs Gradle builds, so they
  tell apps from libraries correctly. That covers catalogs a settings file declares with
  `versionCatalogs { create("<name>") { from(files(...)) } }` (any path, Kotlin or Groovy DSL) or programmatically,
  and the default catalog renamed by `defaultLibrariesExtensionName`. An alias reads only the catalog its accessor
  names, so `alias(tools.plugins.x)` no longer resolves against `libs.versions.toml`, and an unimported
  `gradle/<name>.versions.toml` serves only the accessor `<name>`, when no imported catalog has that name. Version
  catalog sources in librarymodel carry this accessor name. Warm runs and the daemon key the settings-derived source
  list: the files, their kinds and their names. Editing a settings-declared catalog, renaming a `create(...)`, or
  pointing `from(files(...))` at another file reruns the manifest rules, and the resource rules that read the
  module's build script, such as RelativeClassResolution and LibraryCustomView. A settings edit that leaves the
  catalogs alone doesn't (#1872). Settings scanning skips comments and string literals, triple-quoted ones included,
  nests Kotlin block comments, lexes string templates, and reads Groovy's paren-less `from files('...')` and the
  brace-less `create("<name>").from(...)`. A settings block that declares a catalog krit can't read, such as a
  published one, stops an unimported `gradle/<name>.versions.toml` of that name from serving its accessor. Programmatic
  `library(...)`/`plugin(...)` text in a settings string literal adds no library fact, and the parsed-catalog caches
  follow file content, not only size and mtime.
- `--no-parse-cache` now works on a scan delegated to a daemon. The flag was not sent, so the daemon still parsed
  through its parse cache and could serve stale trees with no warning. The daemon now parses every file afresh for that
  request, skipping both its parse cache and the parsed trees it keeps in memory, and confirms the bypass in its
  reply. If a daemon does not confirm it, the CLI scans in process. Later requests keep using the daemon's caches
  (#1919).
- Compose lint parity follow-ups for the fourth batch: `ComposeViewModelInjection` reports configured
  `viewModelFactories` in files that never say "ViewModel"; `ComposeViewModelForwarding` no longer takes JDK, Kotlin
  and Android framework classes (`Pair`, `Intent`, `WeakReference`) or a function whose comment says "Composable" for
  composables, and treats a local function's parameter as a shadow; `vararg` parameters follow Lint's array type in
  `ComposeMutableParameters` and `ComposeUnstableCollections` (#1948).
- MCP `analyze` (mode `android`) reports OnClick for a relative `projectPath`. The sources stayed relative while the
  res dirs were absolute, so no Activity matched its res dir's module (#1908). Rule pages now list a rule's known
  limitations, including OnClick's: a layout is checked only against Activities in its own module.
- Warm runs no longer serve three kinds of stale findings (#1893). A Java edit made while the daemon was stopped
  reaches the next daemon's first scan, even when no active rule parses Java before dispatch.
  VisibleForTestingCallerInNonTest, TestFixtureAccessedFromProduction and DatabaseQueryOnMainThread report on callers
  that need not mention what they flag, so an edit to a caller always re-runs them instead of reusing the cross-file
  findings. That reuse also checks that the findings came from the same rules and config.
- The LSP, the daemon's buffer analysis and the MCP server see module build-script edits (library or application,
  minSdk, targetSdk), version-catalog `[plugins]` edits and rewritten Java imports on the next request instead of after
  a restart. Those facts are read from disk and were memoized for the life of the long-lived analyzer, which also read
  whatever the daemon's last project run had cached. Facts read from disk now last for one request, as the
  version-catalog memo has since #1787, and a request still running in the LSP cannot hand a pre-edit value to a
  later one (#1788).
- `MustUseNamedParams`, `RestrictCallsTo` and `JavaOnlyDetector` no longer scale quadratically. Each call that
  reached the annotated member's hierarchy rescanned every same-named function in the project; the per-name arities are
  now built once per run. A 2,000-file project with an override per file went from 37.9 s to 0.6 s.
  `JavaOnlyDetector` matches an override by parameter count, so `override fun on(x: Int)` is not reported against
  `@JavaOnly fun on()` (#1899).
- `DenyListedApi`: a member function of a class in the file's package no longer silences unqualified calls across
  the package (a `Button` subclass overriding `setOnClickListener`, a class with its own `runCatching`); only a
  top-level function shadows the listed one. The type index records which functions are top-level (cache version 21),
  and a warm run follows a function moving between top level and a class body in another file. An unqualified
  property write inside `cal.apply { }` (`timeInMillis = t`) is now reported (#1888).
- `DenyListedApi`: a `private` top-level function in another file of the package no longer silences a listed call;
  only its own file can call it. A member of an implicit receiver now wins over a top-level function of the file or
  its package, so `setOnClickListener(null)` inside a View subclass and `set(...)` inside `cal.apply { }` are
  reported even when the package declares a top-level function of that name (#1940).
- Warm runs follow version-catalog edits for every manifest rule. Manifest rules read whether a module is an app or a
  library from its build script's `alias(libs.plugins...)` entries, resolved through `gradle/*.versions.toml`, and
  whether it uses the SplashScreen library from its `libs.` dependency aliases, resolved through the catalogs the
  library model finds (settings-declared ones included). Only `ProviderReadPermissionOnly`'s module-graph path keyed
  those catalogs, so changing only the plugin id behind `alias(libs.plugins.mod)` from `com.android.application` to
  `com.android.library` kept reporting `DataExtractionRules` on warm CLI and daemon runs while `--no-cache` did not.
  Whenever a manifest or manifest-source rule is active, the per-manifest, manifest-bundle, Android project-bundle and
  manifest-source keys and the findings-bundle fingerprint now hash the build script each manifest is read from and,
  for a script with aliases, every directory its catalog search globbed, with what it found, and the catalogs found.
  Catalogs no search reaches don't move the keys. On a 300-module project a no-op warm scan takes about 10 ms more
  (#1853).
- Warm runs follow build-script and version-catalog edits for the resource rules that read them: `LibraryCustomView`,
  `RelativeClassResolution`, `VectorDrawableCompat`, `TranslucentOrientation`, `LabelForResource`,
  `UnusedAttributeResource`, `UnusedResources` and `UnusedIds`. The res-directory fingerprint hashed only the module's
  build script and the nearest `gradle/libs.versions.toml` and settings script, so changing only the plugin id behind
  `alias(deps.plugins.mod)` in `gradle/deps.versions.toml` kept reporting `LibraryCustomView` on warm CLI and daemon
  runs while `--no-cache` did not. Those rules now declare what they read for a res directory
  (`rules.ResourceBuildInfoReader`), and whenever one is active the per-res-directory, resource-bundle and Android
  project-bundle keys and the findings-bundle fingerprint hash it through the fingerprint the manifest keys use,
  computed once per run for both. Runs without such a rule, the default, no longer hash build scripts, catalogs or
  settings scripts into the res-directory fingerprint (#1883).
- The manifest-source cache key canonicalizes every source-file hash to one width. It mixed the analysis cache's
  16-hex hashes with the hash memo's 64-hex ones, so the same content gave different keys, and the first warm run after
  a cold one, or a res-only edit after a source edit, re-ran the manifest-source rules (#1853).
- A Kotlin file with a nested body closed on the same line as its last member, such as
  `class H { companion object { val X = 1 } }` or `class H { object O { const val X = 1 } }`, lost every finding.
  The grammar needs a separator before that `}`, and its error recovery turned the enclosing class and the rest of the
  file into one ERROR node, so the parse-error filter dropped all of them and `@Suppress` on the class did not apply.
  Krit now reparses such a file with a `;` in the space before the brace and keeps the result when its error region is
  smaller. Parse-cache entries and cross-file index shards written before the fix are rebuilt (#1839).
- The same repair covers a one-line nested body followed by `;` and more members, such as
  `class H { companion object { val X = 1 }; val y = 2 }` or `} ; val y = 2 }`, and two such classes on one line. The
  inner `}` sat alone in an ERROR node with its member just before it, so it was never a candidate and a later
  `fun after() { val unused = 1 }` lost its `UnusedVariable` finding. Parse-cache entries and cross-file index shards
  written before the fix are rebuilt (#1875).
- The same repair covers a nested body whose `}` directly follows its last member, such as
  `class H { companion object { val X = 1}}` or `{ val X = 1}; val y = 2 }`, which still lost every finding in the
  file. With no space to hold the `;`, krit inserts one, reparses, and maps the tree's byte offsets, rows and columns
  back to the source, so finding positions, `@Suppress` scopes and autofix ranges are those of the file. Parse-cache
  entries and cross-file index shards written before the fix are rebuilt (#1866).
- `--no-cache` skips every cache that can change findings. A one-shot scan read the cross-file findings cache, so a
  stale entry replayed into the run warm results are checked against; it now skips that cache and also the cross-file
  index, resource index, file-walk, tracked-files and incremental oracle caches. With the type oracle on, every
  `--no-cache` scan therefore runs krit-fir in full. A daemon-delegated scan forwards the oracle part too, including
  the `KRIT_MODULE_SHARDS` module facts. The content-keyed parse cache and the content-hash memo stay on (#1818,
  #1831).
- `ArgInFormattedQuantityStringRes` reports `getQuantityString` on a Kotlin subclass of `Resources`, direct or
  indirect and declared in any file, and on `resources` of a Kotlin `ContextWrapper`, Activity or View subclass. The
  resolver records supertypes as written, so each is now resolved through its declaring file's imports. A lookalike
  `Resources` stays silent. A Java subclass is still missed (#1885). The per-file cache keys a class's supertype imports,
  so swapping only the import a supertype resolves through re-analyzes the files that use the class (#1857).
- `DoNotCallViewToString` no longer reports `toString()` in an extension on `List<KView>`, `Map<String, KView>` or
  `Array<KView>`: an extension receiver's type is now its own name, not the last name in its type arguments. The same
  receiver lookup feeds the Wire proto receiver exemption of `UnsafeCallOnNullableType` and `UnnecessaryNotNullOperator`
  and the same-file `toString()` extension check of `NullableToStringCall` (#1926).
- Gradle plugin: with FIR on, the check tasks run the R tasks of exported Android test components
  (`generate<Variant>UnitTestStubRFile`, `process<Variant>AndroidTestResources`). On a clean checkout FIR compiled unit
  and instrumented tests without their own R class, so test files that use `R` were gated (#1724).
- Gradle plugin: the configuration cache is reused after the first build that creates an Android module's R jar.
  `kritExportModel` recorded the R jar by listing `build/intermediates` while configuring, which made the directory a
  configuration input. It now records the path AGP writes the jar to, and krit looks the jar up when it loads the model
  (#1645).
- The resource index cache (`.krit/resource-cache`) kept everything a fresh parse finds. Indexes written by the
  background writer dropped top-level `<item type="id">` declarations and each plural `<item>`'s position, so a warm
  run lost `NotSiblingResource` findings on predeclared ids and reported plural-quantity findings on the `<plurals>`
  line (#1830).
- Rule path heuristics read only the folders inside the scanned project (one with a VCS root, krit config or Gradle
  settings): `InvalidPackageDeclaration`, `TestOnlyImportInProduction`, `MatchingDeclarationName`'s source-set check,
  test-source detection, `nonProductionPathMarkers` and per-rule `excludes` globs. A package matched by folders above
  the checkout (a clone named `app` holding `sub/` with `package app.sub`, or a parent folder named `kotlin`), a
  checkout under a folder named like a test framework (`junit-kit/`, `test/`) or a `**/vendor/**` exclude matching a
  parent folder used to change verdicts with the checkout path; the CLI, daemon, LSP and MCP now agree everywhere
  (#1814).
- `InvalidResourceFolderResource` reads the innermost `res/` folder of a layout path. A checkout under a folder ending in
  `res` (`/builds/mres/...`) reported every layout as an invalid resource folder.
- `MutableImplicitPendingIntent` follows more ways of building the intent, as Lint 32.4.1 does: `getActivities`
  intent arrays held in locals (`arrayOf`, a Java array initializer, and `arrayOfNulls(n)` / `new Intent[n]` of
  constant size whose elements are the last write to each index before the call), and `let`, `run` and `with`
  blocks whose every statement is an Intent method call on `it` or the receiver. Any other use of the intent or the
  array before the `PendingIntent` call, including a write in a nested lambda, still makes the rule abstain
  (#1622).
- The LSP, daemon buffer analysis and MCP tools see version-catalog and settings changes made while they run. The
  version-catalog discovery memo now lasts one request in those long-lived analyzers, not their whole session, and a
  buffer request reads its own analyzer's memo instead of the last project run's. Before, adding
  `gradle/libs.versions.toml` left `InvalidFragmentVersionForActivityResult` and the Gradle catalog lookups on the
  catalog-less result until restart (#1784). CLI scans already built a fresh memo per run.
- `UnsafeIntentLaunch` treats a receiver class registered at runtime with `Context.registerReceiver` or
  `ContextCompat.registerReceiver` as an unprotected component unless, as in Lint, its flags include
  `RECEIVER_NOT_EXPORTED`, its filter holds only protected broadcasts, or its broadcast permission is set to a value
  other than null, blank or a normal one such as `INTERNET`. A filter counts only when krit sees every action: one
  with an action krit cannot evaluate does not, nor does a property or field filter, a filter registered outside a
  function, constructor or initializer block, or a local filter used before the registration other than by
  `addAction` (passed to a function, given to `apply`, `also`, `let`, `run` or `with`, or aliased). It also follows
  the incoming intent into helpers declared
  in other files: top-level functions imported (also under an alias or through a wildcard) or from the same package,
  members brought in by member or static imports, and members of project classes, objects and companion objects
  (`Launcher.launch(this, intent)`), including through aliased or nested imports (#1623).
- `UnspecifiedRegisterReceiverFlag` reads the `addAction` calls of a filter built inline in the registration,
  `registerReceiver(r, IntentFilter().apply { addAction("custom") })`, of a filter registered in a Kotlin `init`
  block or secondary constructor or a Java constructor or initializer block, and on `this.filter`, which it skipped
  before, so such a filter with an unprotected action is reported (#1813).
- `ProviderReadPermissionOnly` attributes providers declared in a library module's manifest the way Lint's merged
  manifest does: they are reported only when the scan holds an application module that depends on the library,
  directly or through other modules, by project dependencies in the Gradle module graph that merge a manifest (not test,
  `compileOnly`, `lintChecks`, `kapt`, `ksp` or `annotationProcessor` ones). They are not reported when every such
  app removes the entry (`tools:node="remove"`) or ignores the rule on it, on its `<application>` or on its
  `<manifest>` (`tools:ignore`), in any of its source-set manifests. Relative names resolve against the module's
  Gradle `namespace`. A library-only scan no longer reports them; a library outside any known module graph, or in a
  build with a module mapped outside its root, is still reported (#1625).
- The project-dependency parser behind the module graph skips commented-out declarations and reads Groovy
  `implementation project(':lib')`, `project(path: ':lib')`, `platform(project(...))` and type-safe accessors for
  modules named with underscores or camel case.
- Warm runs re-run manifest-source rules and replay no findings bundle after a Groovy `build.gradle` edit. The
  manifest-source cache key and the findings-bundle fingerprint now hash every build script, which manifest rules read
  from disk; before, only Kotlin DSL scripts were covered, as source files. For a build whose module graph
  `ProviderReadPermissionOnly` reads (one holding a manifest with `android:readPermission`), they also hash the
  settings file and every module build script the scan didn't, so editing an intermediate module or the settings file
  outside the scan re-runs them: `krit app lib`, an explicit file list, or a gitignored or dot-directory module. They
  also hash, present or missing, each manifest's own build-script candidates (`build.gradle.kts` and `build.gradle`
  in the four directories above it, where the rule reads whether a module is an app and its namespace), so a build
  with no settings file scanned as `krit app/src lib/src` re-runs them when an app module's plugin changes. They also
  hash each module's version-catalog search (every directory it globs for `gradle/*.versions.toml`, with what it
  found, and the catalogs found), so changing the plugin id behind an `alias(libs.plugins...)`, or creating a nearer
  catalog, re-runs them too. The graph is discovered once per run and only when an active rule reads it.
- `UnsafeNativeCodeLocation` reads the assets folders of every source set of the module (`src/debug/assets`,
  `src/<flavor>/assets`, ...) and the `res/raw*` folders of the other source sets' res directories, not only those
  beside `src/main/res`; test source sets are skipped. The resource cache key now covers those assets folders, so an
  ELF binary added under `src/debug/assets` invalidates cached findings (#1619).
- `UnsanitizedFilenameFromContentProvider` proves a Cursor local whose type is inferred from its initializer:
  `ContentResolver.query`, `SQLiteDatabase.query`/`rawQuery` and their variants (also through `!!` or an Elvis that
  returns or throws), or a call to a function or method in the same file declared to return `Cursor`, in Kotlin and
  Java (`var`). The receiver is resolved to the declaration visible at the call (an earlier local in the same or an
  enclosing block, a nearer parameter, a member of an enclosing class), so a same-named declaration in a sibling
  block, a nested function, lambda or another class no longer decides its type, and a `contentResolver` declared with
  another type is not taken for `Context.contentResolver`. A helper call resolves to the innermost local function,
  enclosing-class member or top-level function with that name and a matching argument count; overloads that disagree
  on the return type abstain (#1621).
- `JavascriptInterface` reads Kotlin accessor use-site targets: a property or primary-constructor property annotated
  `@get:`/`@set:JavascriptInterface` (including the array form `@get:[... JavascriptInterface]`), or whose getter or
  setter carries the annotation, counts as an annotated bridge method, and a protected one is reported as
  `Must be public`, as Lint sees the generated accessors through UAST. `@field:`, `@JvmField` and `const`
  properties, and a private property's accessor without a body, generate no annotated accessor and do not count
  (part of #1620).
- The Lint ports' shared call parser is memoized per node for each file's dispatch pass, instead of re-parsing a call
  for every port that looks at it, and their import scan finds `import` keywords before running its regex. On
  element-x-android that cut about 1.3s of CPU from a `--no-fir` scan, with the same findings.
- `UsingMaterialAndMaterial3Libraries` no longer reports material-only modules of a multi-module build because
  another module depends on material3: as in Lint, material3 must be imported in the file or declared by the file's
  own module. `UnusedMaterialScaffoldPaddingParameter` and `UnusedMaterial3ScaffoldPaddingParameter` now resolve
  aliased `Scaffold` imports (#1728).
- `CompileTimeConstant` resolves Kotlin calls into other source files through the type index: top-level functions
  (explicit import, same package, wildcard import), members of the receiver's type and its supertypes (including
  interface methods), operator calls (`a[i]`, `a[i] = v`, `a += v`, `invoke`), and overrides that add the annotation
  to a supertype method declared elsewhere. Inherited members are tried before imports, and calls under an
  extension receiver or a `with`/`apply`/`run` block abstain. The type index now records parameter annotations. The
  rule moves to the cross-file phase, whose findings cache is keyed by every file's content, so an edit to the
  declaring file changes warm results; editor (LSP) and MCP single-file analysis no longer run it. Files that mention
  neither the annotation nor such a function are skipped (part of #1151, #1610).
- `UnsafeImplicitIntentLaunch` evaluates intent actions named by framework constants (`Intent.ACTION_VIEW` and the
  other `android.*` action constants, when the qualifier resolves to the framework class) and by project constants
  declared in other files (`const val`, a Kotlin `val` with a constant initializer, a Java `static final` field,
  top-level or in a class, object or companion object), so an implicit intent with such an action that matches a
  non-exported component is reported as Lint reports it. A same-named parameter or local shadows a constant from
  another file (#1624).
- `--diff` run from a subdirectory of the repository reported no findings: git's repository-relative paths were
  resolved against the working directory.
- Incremental FIR checking works again: the krit-fir shadow jar left out the IR manglers the JVM backend needs, so
  every incremental snapshot build failed, no `.krit/fir-incremental` bundle was written and every scan was a full
  compile, reported as `N requested files gated by compiler errors, no snapshot`. A full check now says under `-v`
  whether it left a snapshot (`snapshot built without 9 files with compiler errors`) or why not, with counts (#1599).
- The dispatch phase appends each file's findings to the run's findings once, instead of rebuilding all findings so far
  for every file that has any, under a lock all workers share. Rule execution on a 12,000-file synthetic project went
  from 16-17s to 5s (total 21s to 10s), and on element-x-android from 9.1-9.5s to 7.3-7.9s, with the same findings.
- `CompileTimeConstant` only runs on files that mention `CompileTimeConstant`. Every finding needs an annotated
  declaration in the same file, so other files cannot be reported; the rule spent about 2s on element-x-android and
  0.7s on okhttp finding nothing. Lint ports can opt into the same gate by declaring `fileMentions()`.
- Module-aware rules run when the scan paths are files (Bazel rules, pre-commit hooks, editors, the Gradle plugin's
  per-module `kritCheck`): module discovery used to open `<file>/settings.gradle.kts`, warn `module discovery failed`
  and skip them. A file's build is the nearest directory above the first file with a Gradle settings file or a Bazel
  `MODULE.bazel`/`WORKSPACE`, looking no higher than a repository root (`.git`) or a Bazel execution root. A file in no
  build has no module graph and walks no directory tree, and `--only-scope file` scans of a file skip discovery unless
  the type oracle or custom rule jars read the graph (#1548, #1669, #1704).
- Bazel targets with a generated srcjar in `srcs` (Wire, KSP) are no longer degraded under `krit_aspect`: krit-fir
  excluded the target's own files in a Gradle-style `src/main/kotlin` as "not part of the JVM compilation" whenever the
  session had any source root, and an unpacked srcjar was a Bazel target's only one. That check now applies only when
  a root is itself a source-set root, so the files compile against the srcjar's sources and get FIR verdicts (#1688,
  #1632).
- The manifest-source Security ports and the `ExportedPreferenceActivityManifest` facts share each file's parsed
  package, imports and class declarations through the run's file-facts cache, so imports are parsed once per run
  instead of four times. On element-x-android the manifest-source phase went from 2.4s to 1.1s, with the same
  findings.
- Source-only scans of large Android projects are fast again: `ProviderReadPermissionOnly`,
  `UnsafeImplicitIntentLaunch` and `UnsafeIntentLaunch` index project classes once per run instead of rebuilding the
  index on every superclass query, and run concurrently. On a 300-module synthetic project the manifest-source phase
  went from 343s to 1.2s, with the same findings (#1658).
- `UnrememberedMutableState` and `RememberInComposition` no longer walk the whole file for every call: same-file class
  and object lookups use a per-file index, built once per run. `IgnoredReturnValue` skips type resolution for calls
  it cannot report without the oracle, and finds a detached `return` without a regex. On element-x-android
  (`--no-fir`), the three rules went from 3.2s, 2.6s and 1.1s to 0.2s, 0.1s and 0.5s, with the same findings (#1664).
- Gradle rules no longer rediscover the version catalog for every Gradle file and catalog lookup. Discovery can walk
  every build file of an included build that provides a settings plugin, and a nested build with `includeBuild("..")`
  walks the whole repository, so a repository with several such builds paid that walk many times over. Each run now
  looks at a directory, and walks an included build, once. On metro (`--no-fir --no-type-oracle`, cold) Gradle
  analysis went from 3.5s to 0.25s and the whole scan from 4.7s to 1.4s, with the same findings.
- The built-in test-data skip (`testdata/`, `testData/`, `test-data/`, `test/data/`, `compiler-tests/`,
  `compilerTests/`, `src/<test set>/resources/`) only counts directories below the scanned path, so a project checked
  out under an ancestor such as `/home/runner/work/testdata/` is analysed instead of reporting no files and exiting 0.
  `krit src` from a module still skips `src/test/resources/`. The CLI also stops dropping files named explicitly on the
  command line from those directories, as `docs/configuration.md` describes; because the Gradle plugin's `krit check`,
  the pre-commit hook and Bazel `krit_test` / `krit_aspect` pass source files explicitly, `testdata/` files listed
  there are now analysed while `krit .` still skips them (#1549).
- FIR applies `-Xname-based-destructuring`, `-Xjspecify-annotations` and the other language-feature, Java-nullability
  and dependency-loading flags a Gradle model carries, instead of dropping them and gating the files that need them.
  Repeated `-Xjsr305` / `-Xnullability-annotations` values now all apply (#1571).
- With FIR on, `kritCheck` runs the producers of the generated sources FIR compiles, without compiling and without
  `exportGenerated`. Producers come from AGP's Variant API and from the Kotlin plugin's `kotlin`/`generatedKotlin`
  source sets, so JVM and Kotlin Multiplatform generators (KSP, SQLDelight, Wire) are covered too. Test source sets'
  generators no longer run, because they need the compiled production classes (#1586).
- The Gradle plugin records each Android source set's generated roots from AGP's Variant API (view binding,
  BuildConfig, SQLDelight, Wire, KSP and any other generator registered with the build), and `exportGenerated` runs
  their producer tasks instead of only R and KSP/kapt tasks found by name (#1574).
- The Gradle plugin exports models for `com.android.test` (instrumentation, benchmark and baseline-profile) and
  `com.android.dynamic-feature` modules, and the settings plugin auto-applies krit to them. A test module records its
  `targetProjectPath` as a project dependency. Before, these modules wrote no model and FIR gated their files on
  unresolved dependencies (#1572).
- Gradle: `kritCheck` no longer reads the project layout when it runs; its fallback SARIF location is the
  configured `sarifOutput`. Functional tests now check `kritCheck`, `kritBaseline`, `kritFormat` and `kritCheckAll`
  store and reuse the configuration cache with Isolated Projects on, in a multi-project build (#1583).
- Gradle: `kritCheck` with several reports enabled writes them all from one krit run (`--report FORMAT=PATH`)
  instead of rerunning the scan per format; one report still uses `-o`, and custom-rule scans keep one run per
  report (#1578).
- Gradle: `kritCheck` and `kritCheckAll` cache keys are relocatable and track content. The exported model is keyed
  with location-independent paths (relative to the build root or the Gradle caches) and its jars by content, so
  checkouts at different paths share entries and a jar re-published at the same path reruns the check. `KRIT_FIR_JAR`
  is an input while the task runs FIR; `parallel` and `noCache` no longer are (#1584).
- Gradle: FIR runs on a Java toolchain (`krit { fir { javaLauncher = ... } }`, default the newer of Java 21 and
  Gradle's JVM), passed to krit as `JAVA_HOME`, and the krit-fir jar resolves through a `kritFir` configuration that
  defaults to `dev.jasonpearson.krit:krit-fir:<krit version>` from the project's repositories when one has it
  (otherwise krit downloads the jar itself, as before). Both are task inputs; `KRIT_FIR_JAR` still overrides the jar
  (#1585).
  defaults to `dev.jasonpearson.krit:krit-fir:<krit version>` from the project's repositories. Both are task
  inputs; `KRIT_FIR_JAR` still overrides the jar (#1585).
- Gradle: `krit { fir { useBuiltDependencies = true } }` makes a per-project `kritCheck` resolve project dependencies
  from their compiled classes, built through Gradle, instead of krit compiling their sources with every dependent
  project. The classes are an ABI-normalized input. The new `--gradle-project-outputs FILE` scan flag carries them:
  a project listed there goes on the classpath and is not compiled as source context (#1581).
- Gradle: the krit tasks of a build share a pool of warm FIR JVMs (`krit { fir { maxDaemons = 2; maxHeap = "4g" } }`)
  through a build service, instead of krit starting and keeping one per project. Each krit run leases a pool slot
  through the new `KRIT_FIR_DAEMON_POOL` environment variable, which keys krit's FIR check daemon by slot; every
  check request retargets that JVM to its project (#1582).
- Gradle: `kritBaseline` writes the baseline to the configured file; it passed `--create-baseline --baseline <file>`,
  so krit wrote a file named `--baseline` in the working directory. It now also skips missing default source roots
  (krit rejected them with exit 2), honors `fir = false`, and passes the project directory as `--base-path`, as
  `kritCheck` now does, so baseline IDs and report paths no longer depend on which source file came first (#1639).
- `InvalidFragmentVersionForActivityResult` finds the module build file for a source file in a package more than
  a dozen directories deep, instead of falling back to the project profile.
- `UnsafeCast` no longer reports "Cast can never succeed" for casts that do
  succeed: an explicitly imported type is no longer checked against an
  unrelated project class with the same simple name, and an `is` check in an
  `if` condition no longer narrows the variable in the `else` branch.
  Source-level smart casts are tighter in general: `is`, `!= null` and `&&`
  checks narrow only the then-branch, `!is` and `!(x is T)` narrow the
  else-branch and the code after an early exit, and `||` conditions and
  multi-condition `when` entries no longer narrow (#1115).
- `--fir` no longer gates files in variant directories the Gradle model did not export (`src/release` next to an
  exported debug variant, other flavors' directories) on a class each variant declares on its own. The source-set
  recheck now compiles each such set with its variant's base and combined sets, never with the exported variant's
  directories, up to 16 sets per scan (#1023).
- Source-level smart casts from an early-exit guard (`if (x !is String) return`) apply only after the guard:
  a use of `x` earlier in the same scope keeps its declared type (#1520).
- `UselessElvisOnNonNull` and `UnnecessarySafeCall` (Go path) no longer report operands that can be null. Source
  type inference narrows on `x != null` only when it is the whole `if` condition, not an operand of `||`, `!` or a
  call argument. Inside an extension function or property accessor, `this` is the receiver type rather than the
  enclosing class, and a bare name that may be a receiver member no longer resolves to an enclosing declaration
  of the same name. `first()`, `single()` and other element-returning stdlib calls keep a nullable element type
  (`Flow<T?>.first()` is `T?`) and abstain when the receiver's type arguments are unknown, instead of returning a
  non-null `Any`; `listOf(a, b)` and `mapOf(k to v)` infer their type arguments from arguments that share one type.
  `UnnecessarySafeCall`'s same-file fallback reads only the nearest declaration in scope, so a local no longer
  borrows the non-null type of a property or parameter it shadows (#1116).
- `UnnecessarySafeCall` (Go path) no longer reports `this?.` inside `with(x) { }`, `x.run { }` or `x.apply { }`
  when `x` is nullable: source type inference types the lambda's `this` as `x`. A destructured lambda parameter
  (`{ (items) -> }`) shadows an outer declaration of the same name, and lambdas inside `return` values, call
  chains, parenthesized arguments and assignments get their own scopes (#1117).

## [0.5.0] - 2026-11-24

### Added
- The `dev.jasonpearson.krit` Gradle plugin and the
  `dev.jasonpearson.krit.settings` settings plugin are published to Maven
  Central (`dev.jasonpearson.krit:krit-gradle-plugin`,
  `dev.jasonpearson.krit:krit-settings-plugin`) with their plugin markers
  (`dev.jasonpearson.krit:dev.jasonpearson.krit.gradle.plugin`,
  `dev.jasonpearson.krit.settings:dev.jasonpearson.krit.settings.gradle.plugin`),
  so `kritExportModel` and FIR work without a local plugin build. Add
  `mavenCentral()` to `pluginManagement { repositories { … } }`, keeping
  `gradlePluginPortal()` there for other plugins. The settings plugin can also be
  applied from a Gradle init script. Release builds resolve both plugins from
  the staged artifacts in a throwaway project before uploading them (#1119,
  #1122).
- `krit-rule-api` and the `krit-fir` oracle jar publish to Maven Central on
  every stable tag; 0.4.0 is the first version there.
- The `krit` Bazel module is set up for the Bazel Central Registry: a stable
  release opens the registry pull request once the GitHub release is public
  (#1122).
- Releases upload `krit-bazel-<tag>.tar.gz`, a reproducible source archive
  of the Bazel module, listed in `checksums.txt` and checked before the
  release is published. The Bazel Central Registry entry now downloads it
  from the release, so registry pull requests pass BCR's stable-URL check
  without a `skip_check` comment. Nightly releases run the Bazel e2e module
  against the published archive (#1195).
- The Bazel module has a `krit_test` rule
  (`load("@krit//:defs.bzl", "krit_test")`): `bazel test` runs krit over
  the `srcs` of `kt_jvm_library`, `kt_android_library`, `java_library` or
  any target with a `srcs` attribute, Go-only and in-process, so the result
  needs no JDK or network and Bazel caches it. A test whose sources krit
  skips (test-data directories) fails instead of passing vacuously. BCR
  presubmit also runs on Bazel 9, and docs/release.md lists the first
  registry submission steps (#1060).

### Fixed
- Compiling the Java-facts helper gives each `javac` run a 2-minute timeout, so a `javac` that never
  exits (such as macOS's `/usr/bin/javac` stub under `JAVA_HOME=/usr`) produces the "Java facts
  unavailable" warning instead of hanging the scan. The helper's `JAVA_HOME` cache-key test now skips
  unless `javac` sits in a real JDK home (#1518).
- Oracle daemons whose java argv moved into an argfile (over 8000 chars, common on large projects) now
  finish Leyden AOT training: `Release`, `Shutdown` and `Close` read the pre-argfile launch arguments,
  so a recording daemon is shut down, its profile is finalized or discarded, and its `.aot.lock` build
  lock is released (#1262).
- JVM helper launches no longer fail with "argument list too long" on large
  Linux projects: the Java-facts helper, the krit-fir checker daemon and the
  oracle's cached run pass long file lists, source dirs and classpaths through
  a JDK argument file (#1118).
- The settings plugin's README listed `java` among the plugins that trigger
  auto-apply; only Kotlin JVM, Kotlin Multiplatform and the Android plugins do.

### Changed
- Cold scans are about 38% faster: rebuildable cache writes skip `fsync`
  (#1114).

## [0.4.0] - 2026-11-24

### Added
- The repository root is a GitHub Action: `uses: kaeawc/krit@vX.Y.Z`
  installs that release, verified against `checksums.txt` (failing closed),
  cached and on `PATH` for Linux (glibc and musl), macOS and Windows; with
  `run: 'true'` it writes SARIF with workspace-relative paths, reports
  `findings-count` and can upload to code scanning. Release builds move a
  floating `v0` tag to each stable release (#1058).
- `krit daemon stop --all` also stops every krit-fir and oracle JVM helper
  recorded in the daemon registry, signalling only helpers whose PID it can
  prove (#974).
- `ForegroundServicePermission` and `QueryAllPackagesPermission`, ported from Android Lint 32.4.1 and checked against
  the Lint CLI on synthetic and fuzzed manifests. `ForegroundServicePermission` reports each `foregroundServiceType`
  value of a `<service>` whose permissions are missing from the same manifest, once the manifest declares
  `FOREGROUND_SERVICE` and targets API 34 or later. `QueryAllPackagesPermission` reports `QUERY_ALL_PACKAGES` when the app
  targets API 30 or later. Both skip library modules, because Lint judges a library by the target SDK of the app
  that includes it. `QueryAllPackagesPermission` is opt-in, matching Lint's disabled-by-default issue.
  A single literal Gradle target SDK declaration takes precedence over manifest values; when a module Gradle file
  exists but its target SDK cannot be proven, the checks abstain instead of falling back to the manifest. Ambiguous
  SDK overrides (including expressions, repeated declarations, block comments or multiline strings), unrecognized
  module types, test-source-set manifests, and nonstandard Android namespace bindings conservatively abstain.
  `ForegroundServiceType`
  (a source check that reads the merged manifest) is not ported. Neither permission check needs a FIR checker.

### Fixed
- The `krit-action` composite action counts SARIF findings with `jq`
  (`findings-count` was always 0 outside this repository) and no longer
  installs a binary it could not verify against `checksums.txt` (#1058).
- A warm daemon call with no edits no longer runs the type oracle's
  `jvmAnalyze` before serving the full findings bundle: the post-parse bundle
  preview keys an oracle run on the prior run's per-file oracle facts, as the
  canonical fingerprint does for clean files, and a run the preview served
  keeps those facts in its fingerprint and manifest (#974).
- The daemon's `ready` line no longer reports `0 module Kotlin files` when
  module source roots hold no Kotlin (custom `sourceSets`); it counts the
  repository instead, and labels its time as module discovery, since
  analysis is lazy (#974).
- Warnings from the daemon-side FIR preflight reach a `-v` client instead of
  only the daemon's stderr (#974).
- A `--fir` scan whose FIR pass returns without a usable verdict now reports
  it like a checker failure (a warning, `summary.firDegraded`, exit 2 under
  `--fail-on-degraded`) instead of silently returning Go-only findings: the
  compile crashed or failed for every requested file, the jar ran none of the
  requested FIR rules, or the pass was cancelled. A crash shared by every
  file of a compile is no longer cached, so it is not replayed on later
  runs (#994).
- Releases publish the Homebrew cask and Scoop manifest with a GitHub App
  installation token (falling back to the PAT secrets), in the stanza order
  the tap's `brew style` audit requires. Build provenance is attested before
  the package-manager steps, and one publisher failing no longer skips the
  other. Maven Central publishing skips with a warning while its secrets are
  unset (#1057).
- FIR `TooGenericExceptionCaught` honors Go-accepted plain configured patterns
  made from letters, digits, `_`, `.`, `|`, parentheses, brackets (including a
  leading `^` negation), `-`, and single `*`, `+`, or `?` quantifiers. It
  abstains for patterns containing backslashes, `(?`, `[[`, `[:`, `&&`,
  consecutive quantifiers, `{` (bounded repetition) or `^`/`$` anchors, and for
  Go-accepted patterns Java cannot compile.
  These conservative misses are pinned; this is not full RE2/Java regex parity.
- Go normalizes only `TooGenericExceptionCaught.allowedExceptionNameRegex`
  before sending it to FIR, dropping empty strings and patterns Go rejects so
  the Go struct default applies in both engines. Other rules' options pass
  through unchanged.

### Changed
- `CursorLoopWithColumnIndexInLoop` gains a FIR checker requiring a resolved
  Android Cursor receiver, preserving while-loop first-match semantics.
  Both engines require an explicit bare simple-name receiver
  bound by a parameter of the directly enclosing function (including an anonymous
  function)/method, an explicit value parameter of a call's trailing lambda
  (including labeled and annotated expressions) or sole lambda argument whose body
  directly contains the loop, or a preceding
  non-delegated local in the loop's directly enclosing callable, with no
  intervening shadowing or writes. Calls inside nested lambdas, local functions,
  or anonymous classes/objects stay silent, as do
  properties/fields, top-level values, implicit receivers, explicit `this`, and
  compound expressions. Constructor, implicit `it`, and non-direct enclosing lambda parameters are also
  conservatively missed; these limitations are pinned in tests and goldens.
  Go column arguments must be non-interpolated string literals, bare immutable/effectively-final
  parameters or preceding locals of the enclosing function, or qualified platform constants:
  the root must be explicitly imported from `android.*`, `java.*`, or `javax.*`, or the
  reference must be fully qualified in one of those packages; the field must be
  `UPPER_SNAKE_CASE`, no root name may be declared anywhere in the file, and no
  reference segment may be bound in function scope. Conflicting root imports
  (including static imports and aliases) are rejected. Kotlin rejects
  non-platform wildcards; Java rejects static wildcards for explicitly imported
  roots and non-platform wildcards for fully qualified references. Extension functions
  are rejected;
  every lambda enclosing the lookup must be passed
  to a call named `use`, `let`, `also`, `forEach`, `forEachIndexed`, `onEach`,
  `map`, `mapNotNull`, `filter`, `takeIf`, `takeUnless`, or `repeat` (allowlisted
  by name; a project member function with these names and a receiver lambda is a
  pinned Go gap); any other lambda scope is rejected. Inherited members from
  other project files remain a pinned Go gap when their name exactly matches
  the SDK root (FIR resolves the member and stays silent).
  This includes nested SDK classes inside safe-call `?.use` lambdas and lookups
  nested in another call's arguments in a local initializer. FIR accepts bare or class-qualified constants
  only by resolved symbol: `const val` properties or Java `static final String` fields with
  compile-time constant initializers and no custom getter. Mutable names, calls, indexing, interpolated
  templates and other computed arguments stay silent, with conservative misses pinned.
  FIR uses the same call pre-order as Go. Go remains
  the fallback without a classpath or when files do not compile (#1076).
- `ExplicitGarbageCollectionCall` and `LocaleDefaultForCurrency` now have FIR checkers, with the Go rules kept as the
  fallback. On files the compiler analyzes, the call is the resolved `System`/`Runtime` `gc` or `runFinalization`, or
  the resolved `Currency.getInstance(Locale.getDefault())`, so an import alias, a type alias or a `Runtime` held in a
  variable is found and a project class named `System`, `Runtime`, `Currency` or `Locale` is not reported.
  Currency checks retain Go's nearest enclosing class scope: named, companion and anonymous objects do not
  independently widen the money-related-name heuristic (#1086).
- `TooGenericExceptionCaught`, `ReturnFromFinally` and `ThrowingExceptionFromFinally` now have FIR checkers, with
  the Go rules kept as the fallback. On files the compiler analyzes, `TooGenericExceptionCaught` compares the resolved
  caught class, so a type alias or import alias of a generic exception is reported and a project class that only
  shares the simple name is not (a written qualifier such as `java.lang.Exception` is still not matched). The two
  `finally` rules follow the jump itself: a `return` is reported when the function it returns from lies outside the
  block, so a custom lambda label no longer hides or triggers it by name, and a `throw` inside a lambda that is not run
  in place (`thread { throw ... }`, `lazy { ... }`) is not reported. A jump in a `finally` nested in another `finally`
  is reported once.

### Removed
- The native IntelliJ IDEA / Android Studio plugin (`editors/intellij-plugin/`)
  and its CI and JetBrains Marketplace publish workflows. IntelliJ users can
  keep using krit through the LSP server (`editors/intellij/`).

## [0.3.1] - 2026-11-23

### Added
- krit reads the project's Android Lint configuration (`lint.xml` issues set
  to `severity="ignore"`, Gradle `lint { disable ... }`) and keeps its ports of
  ignored issues off unless `krit.yml` sets them active;
  `androidLintConfig: {enabled: false}` opts out (#1071).
- krit reads ktlint settings and `max_line_length` from `.editorconfig`: a
  disabled ktlint rule turns off krit's equivalent, IntelliJ's
  import-on-demand packages are exempt from `WildcardImport`, and
  `max_line_length` sets `MaxLineLength`'s limit;
  `ktlintEditorConfig: {enabled: false}` opts out (#1077).

### Fixed
- A detekt 2 `ForbiddenImport.forbiddenImports` list is imported as
  whole-path globs (#1072).
- `PackageNaming` allows digits and underscores after a segment's first
  letter (`okhttp3`, `coil3`); the detekt-compat profile keeps detekt's
  pattern (#1078).

### Changed
- `OptInMarkerExposedPublicly` does not report a library's own markers in
  its own packages (#1073).
- `TestOnlyImportInProduction` skips test-support, test-framework
  integration and benchmark modules and Gradle test source sets (#1074).

## [0.3.0] - 2026-11-23

### Added
- Bazel workspaces get a project model: one module per Bazel package with
  JVM or Android targets (`//app/src/main`) and dependency edges from their
  `deps`/`exports`/`runtime_deps`/`associates`/`plugins` labels, Android
  resource directories from `resource_files` globs, and Maven artifacts from
  `MODULE.bazel` `maven.install`/`maven.artifact`, `WORKSPACE`
  `maven_install` and rules_jvm_external lock files as library facts (#1067).
- A `krit` Bazel module (`bazel/`) that downloads the prebuilt release for
  the host and verifies it against `checksums.txt`:
  `bazel_dep(name = "krit")`, then `bazel run @krit//:krit` (#1069).
- `make corpus-bench` / `internal/devtools/corpusbench`: a pinned public
  corpus (Gradle Kotlin DSL and Groovy, KMP, Android/Compose, Bazel) with
  completion, wall time, memory, syntax-error, project-model and findings
  metrics (#1056).

### Fixed
- Indexing a file with thousands of top-level declarations no longer hangs
  (rules_kotlin: timeout -> 3 s) (#1055).
- Empty SARIF output writes `"results": []` and `"rules": []`, which GitHub
  code scanning accepts (#1062).
- New files that are not `git add`-ed yet are analyzed (#1063).
- `krit .` and `krit $PWD` report the same findings (#1066).
- Gradle modules declared through custom settings DSLs, directory-walking
  helpers, `module(path)` helpers and repositories of separate builds are
  discovered; `ModuleDependencyCycle` ignores test-scope edges and
  `KotlinVersionMismatchAcrossModules` ignores intentional higher targets
  and minority baselines (#1065).
- Seven Kotlin constructs that recovered with ERROR nodes now parse: KMP
  `public actual val` constructor properties, `suspend { }`, `open(...)`
  calls, `return e...` in `when` branches, char ranges after a char-literal
  line, line-leading `in1`/`by`, and `getOrPut(k) { } += v` (#1070).
- Nightly releases tag with `GITHUB_TOKEN` and dispatch the release
  workflow (#1064).

### Changed
- `FunctionNaming` ignores `@Composable` functions in the shipped config;
  `InvalidPackageDeclaration` accepts a directory layout that omits the
  common root package; `UnusedUnaryOperator` skips string and lambda
  operands (#1068).


### Fixed
- Source inference treats simple-name library method matches as unknown when the
  source-written receiver type annotation or constructor uses a bare simple name
  resolving to a visible same-file project class, object, or interface rather
  than an explicit import, such as a local `Context`. Explicit imports take
  precedence over top-level package declarations, while nested classifiers
  (including interfaces, enums, and annotation classes) and ordinary object
  declarations directly in class, object, or interface bodies take precedence
  throughout their containing declaration, including its header. The
  declaration index records top-level and member/directly nested classifiers,
  plus local classifiers reachable through its indexed syntax-node kinds.
  Local lookalikes take precedence only where that index records them. It does
  not traverse companion objects (`companion_object`), enum class bodies
  (`enum_class_body`), anonymous object bodies (`object_literal`), accessor
  bodies (`getter`, `setter`), or control-flow wrappers such as `if_expression`,
  `when_expression`/`when_entry`, `for_statement`, `while_statement`,
  `do_while_statement`, and `try_expression`. A `lambda_literal` is traversed
  when reached directly, but call/lambda wrappers such as `call_expression`,
  `call_suffix`, `annotated_lambda`, and `parenthesized_expression` can hide
  it. Anonymous-function bodies (`anonymous_function`), declarations anywhere
  inside property declarations (initializers, delegates, accessors), and
  declarations behind expression-wrapped function bodies are likewise
  unindexed. These classifiers retain library signatures as before. Direct
  declarations in init blocks are traversed through `anonymous_initializer`
  and `statements`. Named companion objects themselves are not indexed as
  lookalikes. Qualified names do not trigger this suppression. Stdlib
  factories, their inferred locals, and library call chains retain library
  signatures even when an indexed project lookalike is visible;
  library mappings and subtype behavior remain unchanged.

### Changed
- Go's `UnsafeCallOnNullableType` no longer reports `!!` on a simple identifier whose local parameter or `val`/`var` declaration has an explicitly written, syntactically proven non-null type. Untyped declarations remain reportable because Go does not infer their initializer types.
- `UnnecessarySafeCall` gains a FIR checker that reports only definitely non-null
  resolved receivers, excluding platform types and unstable or cross-module
  smart casts. Stable same-module smart casts are followed. Go remains the
  fallback for files that do not compile and scans without a classpath.
- `ClickableViewAccessibility` skips custom-view override checks in abstract classes
  in Go and in abstract or sealed Kotlin classes in FIR; Go also skips sealed Kotlin
  classes. Go skips overrides owned by Kotlin object literals and companion objects or
  Java anonymous classes, nested interfaces, enums and records rather than attributing
  their methods to enclosing Views. FIR keeps anonymous Kotlin Views conservatively
  silent. Listener checks remain active.
- `BindsInsteadOfProvides` no longer reports local functions, or members of anonymous objects, declared inside functions, lambdas, property initializers or accessors, constructors or `init` blocks, in either engine. FIR also preserves the kotlin-inject wildcard-import exemption.
- Pin cross-rule FIR/Go fixture divergences for `UnsafeCast`: resolved nullable
  class-name interpolation and Go's platform-type heuristic on a local `Context` lookalike.
- FIR `UnsafeCast` now normalizes `customPreviewPrefixes` like Go, accepting scalar
  strings and `{value, reason}` entries while trimming whitespace and dropping blanks.
- `UnnecessaryNotNullOperator` gains a conservative FIR checker for definitely non-null operands;
  Go remains the fallback without a classpath or when a file fails compilation.
- `DoubleMutabilityForCollection` adds `onlyValCandidates` (default `true`,
  preserving private/local, non-nullable, never-reassigned, non-test narrowing).
  Imported detekt configurations default it to `false` in both Go and FIR,
  reporting properties whose resolved type is in `mutableTypes`, including
  public, nullable, reassigned and test-source vars, but not constructor
  parameters. With source resolution, compatibility mode does not guess mutable
  types from shadowed factory names or report read-only declared types. Without a
  resolver, Go checks only default-mode candidates with unshadowed explicit mutable
  types or proven factories without an explicit read-only type. This conservative
  daemon/IDE, MCP and precommit fallback can miss compatibility-mode findings.
  The CLI builds the source resolver whenever this rule is active.
  Go reads qualified written types as written; FIR expands
  aliases and resolves calls such as `toMutableList()` that Go skips. Both engines
  miss detekt's Java platform-type findings (`System.getenv()` and Java `List`
  getters); this gap is documented, not implemented. The `var` → `val` fix remains
  available only for val candidates.
- The opt-in `LogConditional` rule follows Android Lint 32.4.1 fully in FIR. Go reports `Log.v`, `Log.d`,
  `Log.i`, and `Log.println` only when the message is a call or directly contains one in a string template or
  `+` concatenation, or is an Elvis expression with a direct call on its left. `Log.w`/`Log.e`/`Log.wtf` do
  not report. The nearest `if` or `when` branch decides
  guarding (literal `true` and `when`'s `else` do not guard), with enclosing calls and declaration boundaries
  stopping the search. Diagnostics use Lint's method-specific message; the rule remains off by default.

### Added
- FIR-backed `UnspecifiedImmutableFlag` resolves Android PendingIntent factories and constant flags,
  only confirming or removing Go findings on the same line. Go retains SDK gating and severity;
  stable same-file vals are followed, and unmatched FIR findings are dropped. Scans without a
  classpath or with compiler errors retain the Go fallback.
- FIR-backed `UselessElvisOnNonNull` reports provably non-null Elvis operands,
  preserving Java platform-type fallbacks and the Go fallback for scans without
  a classpath or files with compiler errors.
- FIR-backed `ClickableViewAccessibility` resolves Android View touch handlers,
  listener registrations and `performClick` overrides, preserving Go exemptions
  and retaining Go fallback without a classpath or for files with compiler errors.
- FIR checker for `BindsInsteadOfProvides` proves Dagger module annotations and unchanged parameter returns, retaining the Go fallback for non-compiling files and scans without a classpath.
- `UnsafeCast` uses resolved compatibility proofs in FIR requiring both sides to be final non-generic classes (enums and objects included), abstains on expect/actual classes, and keeps the Go fallback for scans without a classpath and files that do not compile.
- FIR checker for `RunBlockingInTest` resolves coroutine calls and JUnit/kotlin.test/TestNG annotations, preserves test intent and bridge exemptions, and uses a confirm-only verdict so FIR may confirm or remove Go findings but never add findings; Go remains the fallback without a classpath or for non-compiling files.
- `CoroutineScopeCreatedButNeverCancelled` has a resolved-symbol FIR checker for scope factories and cancellation/ownership,
  with the unchanged Go fallback for scans without a classpath and files that do not compile.
- FIR checker for `PlainFileWriteOfSensitive` resolves plain-file writes and sensitive filename literals, retaining the Go fallback and its diagnostic text.
- FIR checking for `TestWithoutAssertion` preserves Go's name/text exemptions and only narrows its findings.
  It resolves known assertions and same-file helpers and inspects nested lambdas. Unknown calls,
  cross-file/source-set/module helpers, and library calls outside an explicit non-asserting allowlist abstain
  without disabling incremental slices;
  non-compiling files and scans without a classpath keep the Go fallback.
- FIR-backed `ComposeSideEffectInComposition` resolves composable bodies, state receivers and Compose APIs; Go remains the fallback without a classpath or when compilation fails.
- FIR checker for `MutableStateInObject`, with resolved synchronization and thread-safe type exemptions; Go remains the fallback without a classpath or for non-compiling files.
- FIR `AvoidReferentialEquality` checker with resolved, alias-expanded operand
  types and configurable `forbiddenTypePatterns`. FIR retains call-argument
  same-instance probes, including receiver chains and safe-call receivers, but
  drops broad identity-name heuristics when a forbidden operand type is resolved. In both engines,
  `forbiddenTypePatterns: []` turns the rule off.
- FIR-backed `SingletonOnMutableClass` resolves singleton scopes, mutable collection types and state-protection references; Go remains the fallback without a classpath or for files with compiler errors.
- FIR-backed `OptInMarkerExposedPublicly` is confirm-only: resolved markers can confirm or remove Go findings, never add findings on Go-silent lines. It supports the shared well-known table and configured `additionalMarkers` by simple name, requires `kotlin.RequiresOptIn` metadata, and skips protected APIs. The package-family skip applies only to well-known markers; configured `additionalMarkers` bypass it, including for library stubs. Go remains the fallback without a classpath or for compiler errors.
- `IntentReset` and `IntentWithNullActionLaunch`, ported from Android Lint 32.4.1 and checked against the Lint CLI
  on synthetic Kotlin and Java fixtures. Both follow an `Intent` from its constructor through local variables,
  aliases and builder-style calls inside one method. `IntentReset` reports `setType` after `setData` (or the reverse,
  or `setType` after a `Uri` constructor argument) when both calls sit in the same block, including Kotlin
  `intent.data =` and `intent.type =` assignments. `IntentWithNullActionLaunch` reports an `Intent()` that gets no
  action or component and reaches `startActivity` or `sendBroadcast`; passing it to any other call, except logging
  and printing calls, stops the check.
- `AppLinkUrlError` and `AppLinkWarning`, ported from Android Lint 32.4.1's `AppLinksValidDetector` and checked
  against the Lint CLI on synthetic and fuzzed manifests. They cover `<data>` scheme, host, port, path, MIME type and
  case checks, the elements an `autoVerify` filter needs to be an Android App Link, VIEW filters without a URI,
  scheme, host or `BROWSABLE` category, non-exported VIEW activities, and `http(s)` App Link filters without
  `autoVerify`. Filters whose scheme or `autoVerify` is a resource reference are not judged, because the manifest
  rules do not read resources. `tools:validation` test URLs, `<uri-relative-filter-group>` checks,
  `AppLinkSplitToWebAndCustom` and the network-based `AppLinksAutoVerify` are not ported.
- Android Lint manifest checks ported with Lint 32.4.1 semantics and checked against the Lint CLI on synthetic
  manifests: `SystemPermissionTypo` and `CustomPermissionTypo` (edit-distance matches against the SDK and system
  permission lists and the permissions declared in the manifest), `ReservedSystemPermission`,
  `PermissionNamingConvention` (off by default, like Lint), `KnownPermissionError`, `LockedOrientationActivity`,
  `NonResizeableActivity` (both skipped in library modules), `IntentFilterUniqueDataAttributes`, and the manifest
  attribute part of `DiscouragedApi` (fixed orientation, aspect ratios and non-resizeable activities, which
  Android 16 ignores on large screens).

### Changed
- `HardcodedSecretKey` and `StaticIv` now skip test source files in both Go and FIR. They remain enabled by default and
  continue to report hardcoded keys and static IVs in production source.
- `AllowAllHostnameVerifier` now reports Apache `AllowAllHostnameVerifier` constructions and constructor
  references, and `SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER` references, using Android Lint 32.4.1's
  messages. Custom always-true `javax.net.ssl.HostnameVerifier.verify` implementations and SAM lambdas
  belong only to `BadHostnameVerifier`, removing duplicate findings. `BadHostnameVerifier`'s Go implementation retains
  the original class-wide Lint visitor, signature matching, inheritance and `verify` name anchor,
  including extra statements, last-return-wins and overload quirks, and restores enum entry coverage.
  `BadHostnameVerifier`'s FIR checker replaces Go findings on compiled Kotlin files and retains the
  former custom-verifier checker, including implicit SAM conversions,
  objects, companions, aliases and cross-file inheritance. FIR now requires every targeted return
  and every expression-body branch to yield the literal `true` for classes, objects, enums, SAMs and
  companions alike; throwing paths and Kotlin `require`/`check` or JDK `Objects.requireNonNull` guards
  are silent. Go retains its existing Lint-compatible last-return-wins behavior, and those Go-only
  reports are pinned in goldens. Former FIR goldens run under `BadHostnameVerifier` with
  machine-checked Go lines. Apache lookalike enum entries, catch parameters and named-argument
  labels are silent. Old custom
  verifier baseline entries change issue ID or collapse into an existing `BadHostnameVerifier`
  finding; the playground baseline changes only `NetworkHelper.kt:35`, with no duplicate.
  Apache references require an explicit import or fully qualified name in Go; FIR resolves symbols.
  Unresolved names and lookalikes are silent.

### Fixed
- The `MutableStateInObject` Go fallback ignores nested class writes and same-named writes through unrelated
  receivers, except bare, `this.`-qualified, and shadowed-receiver finally writes, which it accepts as conservative resets.
  It also recognizes implicit-receiver and object-qualified lifecycle/finally resets, JUnit 5 `@BeforeAll` /
  `@AfterAll` hooks, and exempts never-written private state and additional Java concurrent collection types.
- `--fir` no longer gates test and androidTest files on a redeclaration that only exists because Krit
  compiles a project's scanned source sets together (#972). When that compile gates a file on
  `Redeclaration` or `Conflicting overloads`, its project is rechecked with each scanned source set
  compiled on its own, and those verdicts replace the merged ones for the project's gated files. Other projects keep the
  single compile: compiling every source set on its own (`KRIT_MODULE_SHARDS=isolate`) took clearly
  longer and several times the CPU time on the projects measured. `KRIT_MODULE_SHARDS=merged` turns the
  recheck off.
- `--output-types` no longer leaves a krit-fir daemon running under `--no-daemon` or `--no-fir-daemon`, and it
  compiles against the scan's classpath, model source roots, JVM target and Kotlin settings.
- `--max-cost` scans no longer delegate to a krit daemon, which ignored the filter and ran every rule.
- `VersionCatalogBuildSrcMismatch` names the conflicting buildSrc file relative to the project instead of by
  absolute path, so its message, and baseline entries for it, no longer depend on the checkout location.
  Baseline entries for this rule change once; regenerate the baseline to pick them up.
- The release workflow fails when the `krit-fir-<tag>.jar` asset or its checksum is missing. It creates the
  release as a draft, retries each asset upload, and publishes only after checking the uploaded assets.
- `ComposeRawTextLiteral` on files the compiler analyzes now counts letters outside the Basic Multilingual Plane
  (mathematical bold, some scripts), as the Go rule does. On files that fall back to Go, a `Text` call is reported
  only when it can be the AndroidX one: an `androidx.` qualified call, an AndroidX import of `Text`, or a wildcard
  import of an AndroidX material or text package. A design-system `Text` from a wildcard import or a qualified
  owner such as `Theme.Text(...)` is no longer reported there, matching the FIR checker.
- `ComposeSemanticsMissingRole` on files that fall back to Go now needs the file to import the foundation
  `clickable`, `toggleable` or `selectable` (by name or package wildcard), so a project modifier with the same name
  is not reported, matching the FIR checker.
- FIR checkers for five crypto and command rules miss fewer real cases. The Go fallback rules are unchanged:
  - `RsaNoPadding` reads a transformation held by a `const val`, a final Java constant field, or a local `val` set to a
    string literal.
  - `StaticIv` reports a zero-filled `ByteArray(size)` IV, and `toByteArray()` or a Base64 decode called directly on
    a String constant or a property set to a literal. A template over that property was already reported.
  - `WeakKeySize` follows `init`/`initialize` chained on `getInstance(...)` or called inside `apply`, `run`, `with`,
    `also` or `let` on the generator. It also reads an algorithm from a Java constant such as
    `KeyProperties.KEY_ALGORITHM_RSA`.
  - `PrngFromSystemTime` reports `java.util.Random.setSeed` with a clock seed (not `SecureRandom.setSeed`, which adds
    to the seed), a clock read stored in a local `val` first, and the Android `SystemClock` uptime clocks.
  - `RuntimeExecUnsafeShape` reports a command built with `format` or `plus` from a value that is not constant
    (`"ls %s".format(dir)`, `String.format("ls %s", dir)`), even when the literal holds no `$`.
- `InstanceOfCheckForException` (FIR) no longer reports an `is`, `!is` or `as` check on the caught exception when the
  checked type is not a subtype of the caught type, such as an interface check (`e is Retryable`) or an upcast
  (`e as Any`). No catch clause can replace such a check. This matches detekt 1.23.8. When either type is unresolved
  the check is still reported, as the Go fallback does.
- `TooGenericExceptionThrown` (FIR) reports on the `throw` expression, as detekt and the Go rule do. Before, it
  reported on the constructor call, so a throw split over lines, or a generic exception in an elvis or `if` on a later
  line, was reported on a different line.
- `PrintStackTrace` with `--fir` reports `Thread.dumpStack()` again, as Go and detekt do, and reports a caught
  exception's `printStackTrace()` in test sources, tooling paths and logging sinks, which the FIR checker skipped
  although Go reports them. Only the receivers Go does not report (a parameter, a local, `e.cause`) still skip
  those files. The Go rule no longer reports a receiver spelled like the caught variable that is another value (a
  shadowing local or lambda parameter, `this.e`), and it now reports an outer catch's variable inside a nested catch.
- `ObjectExtendsThrowable` with `--fir` reports on the object's name, the line Go reports, instead of the first
  annotation or modifier. The Go rule reads the superclass alone: it no longer reports an object that only mentions
  an exception type in an interface type argument, a constructor argument or a `by` delegate, or that extends a
  project class named like a Throwable, and it now checks companion objects and the same superclass names in every
  container.
- `ThrowingNewInstanceOfSameException` (Go rule) no longer reports a constructor argument that builds a new value
  from the caught exception (`e.message ?: ""`, `e.message + "!"`) or a caught name hidden by a lambda parameter or
  local, matching the FIR checker.
- `UnnecessaryFilter`'s FIR checker reports on the terminal call's name, where the Go rule reports, instead of the
  start of the chain. A chain split over several lines now gets the same line in both engines and keeps the Go
  autofix.
- `UseAnyOrNoneInsteadOfFind`'s FIR checker also reports a predicate passed as a function reference, a function value
  or an anonymous function (`list.find(::isValid) != null`), as detekt does.
- The Go fallback of `PreferToOverPairSyntax` no longer skips every `Pair(a, b)` when some project file declares a
  class named `Pair`; that class counts only when it is in the file's package or star-imported. A bare `Pair` in a
  file that imports `kotlin.Pair` under an alias is not reported, because the alias hides the default import.
- The Go fallback of `UnnecessaryLet` counts `it::member` and a parenthesized `(it)` as uses of the parameter, so it
  no longer suggests "a simple if" for them. A `?.let { }` that ends the lambda of `withTimeout`,
  `withTimeoutOrNull`, `withLock`, `withPermit`, `runInterruptible`, `measureTimedValue`, `firstNotNullOf(OrNull)` and
  a few other calls that return their lambda's result is now treated as used, as detekt does.
- `WakelockTimeout`, `SetWorldReadable` and `SetWorldWritable` now have FIR checkers, with the Go rules kept as the
  fallback. On files the compiler analyzes, the receiver is the resolved `PowerManager.WakeLock` or `java.io.File`, so
  an import alias, a type alias, a call result, `!!` or a smart cast is found, and `WakelockTimeout` also reports an
  `acquire()` on an implicit receiver. The folded `true`/`false` arguments of the two file rules are read as before.
- `OkHttpCallExecuteSync` now has a FIR checker, with the Go rule kept as the fallback. On files the compiler
  analyzes, the call must resolve to `execute()` on `okhttp3.Call` or a class that implements it, so a call held in a
  variable that is not named like a call is reported and an unrelated `execute()` on a `...Call`-named receiver is not.
  A call inside a lambda the compiler does not run in place (`Thread { }`, a stored callback) is not on the
  coroutine's thread and is no longer reported. The dispatcher, executor, Rx scheduler, cancellation-bridge and KDoc
  exemptions are unchanged.
- `UseCheckOrError` and `UseRequire` now have FIR checkers, with the Go rules kept as the fallback. On files the
  compiler analyzes, the thrown class is the resolved constructor of `IllegalStateException` or
  `IllegalArgumentException`, so an import alias or type alias of the exception is reported and a project class or
  function that only shares the name is not. The message must be a non-null `String` or absent, as before.
- `ThrowingNewInstanceOfSameException` and `ThrowingExceptionsWithoutMessageOrCause` now have FIR checkers, with the
  Go rules kept as the fallback. On files the compiler analyzes, the thrown or called constructor and the caught
  type are resolved classes (type aliases expanded), so a type alias or import alias of the exception is found, a
  nested class or function that only shares the simple name is not reported, and the caught variable is the catch
  parameter's symbol, so a lambda parameter that reuses the name no longer counts as the caught exception.
- `SuspendFunWithFlowReturnType` now has a FIR checker, with the Go rule kept as the fallback. On files the compiler
  analyzes, the return type is the resolved type (an import alias, a typealias or an inferred type of `Flow`,
  `StateFlow`, `SharedFlow`, `MutableStateFlow` or `MutableSharedFlow` counts, a project type named `Flow` does
  not), and the modifier is kept only when the body calls a function the compiler resolves as `suspend`, so a
  member named like a suspend overload (`first`, `toList`) no longer hides the finding and a suspend call inside
  an inline lambda counts.
- `SystemPermission` (Lint's `ProtectedPermissions`) now uses Lint's list of about 740 system-only permissions
  instead of 56, honours `android:maxSdkVersion` and the minimum SDK for permissions an ordinary app could hold on
  older releases, reports at the `android:name` attribute with Lint's message, and is on by default as in Lint.
- `UnusedPrivateFunction` and `UnusedPrivateClass` now have FIR checkers, with the Go rules kept as the
  fallback. On files the compiler analyzes, a private function or class is used only when a resolved reference
  (a call, callable reference, operator or delegate convention, type, constructor, qualifier or annotation)
  points at it, so a same-named member, local or overload elsewhere in the file no longer hides an unused
  declaration, and a `fun interface` is judged like any other class.
- `SharedPreferencesForSensitiveKey` and `LogOfSharedPreferenceRead` now have FIR checkers, with the Go rules
  kept as the fallback. On files the compiler analyzes, the editor or preferences are the resolved
  `SharedPreferences` members, so a receiver held in a property, returned by a call or left implicit counts, and
  an editor reached through `EncryptedSharedPreferences` is skipped. A logger is a resolved member of a class or
  object named `Log` or `Timber`, so an aliased or fully qualified `android.util.Log` and
  `Timber.tag(...).d(...)` count and a local value named `Log` does not.
- `SqliteCursorWithoutClose` now has a FIR checker, with the Go rule kept as the fallback. On files the
  compiler analyzes, the query is a resolved call to `SQLiteDatabase` or `SupportSQLiteDatabase`, so a
  receiver written as a property chain (`helper.readableDatabase.rawQuery(...)`) or left implicit counts, and
  only a resolved reference to the cursor, not any identifier of the same name, hands it over to other code.
- `AssertTrueOnComparison` now has a FIR checker, with the Go rule kept as the fallback. On files the compiler
  analyzes, the call must resolve to kotlin.test, JUnit 3, JUnit 4 or JUnit Jupiter, so a project helper that
  reuses the name `assertTrue` is no longer reported, and an import-aliased `assertTrue` is.
- `NullableToStringCall` now has a FIR checker, with the Go rule kept as the fallback. On files the compiler
  analyzes, the receiver or template part is nullable by the compiler's type, so a mutable property after a null
  check, an implicit lambda parameter, a safe-call result and a type parameter are reported, and a stable smart
  cast to non-null is not.
- `CastNullableToNonNullableType` now has a FIR checker, with the Go rule kept as the fallback. On files the
  compiler analyzes, the operand's nullability is the compiler's type (stable smart casts, platform types and
  typealiases to nullable types are read exactly), so a cast Go's source inference could not resolve is reported.
- `SwallowedException` now has a FIR checker, with the Go rule kept as the fallback. On files the compiler
  analyzes, only an access to the catch parameter's own symbol counts as a use, so a logger call such as
  `Timber.e(...)`, a lambda parameter or a local that reuses the name no longer hides a swallowed exception,
  and a typealias of an ignored exception type is exempt like the type itself.
- Source type inference no longer takes a call's return type from a same-name function that the call
  cannot reach: a private, local or private-class function in another file. When the reachable
  same-name declarations disagree, or one has no declared return type, the call's type is unknown
  instead of whichever file was indexed last. This removes false positives from
  `UselessElvisOnNonNull` and `UnnecessaryNotNullCheck` (and other rules reading call types) on
  calls to a nullable function that shares its name with a private non-null function elsewhere.
- `DataBindingWithoutKapt` resolves `alias(libs.plugins.<name>)` in a `plugins` block through
  `gradle/libs.versions.toml`, as Lint 32.4.1 does, for both the Kotlin Android and the kapt plugin
  in Groovy and Kotlin DSL. Like Lint, it skips a table entry without a version and does not resolve
  `kotlin("kapt")`.
- `InsecureBaseConfiguration` reports at the value of `cleartextTrafficPermitted`, and
  `CustomX509TrustManager` and `TrustAllX509TrustManager` follow a `typealias` declared in the same
  file to `X509TrustManager`, as Lint does.
- `IgnoredReturnValue` now has a FIR checker, with the Go rule kept as the fallback. On files the compiler
  analyzes, a discarded call is reported from the callee's must-use annotation or its resolved stdlib
  return type (Sequence, Flow, Stream, function value), and "discarded" is the compiler's own unused-value
  analysis, so a call that is not the last statement of a callback lambda is reported where Go's text
  heuristics missed it.
- `UnusedIds` now follows Lint 32.4.1: an id stays used only when source reads it or a resource that
  is itself used refers to it (a reference from an unused layout no longer counts), navigation graph
  ids are never reported, and the finding sits at the first `@+id/` mention, which can be a forward
  reference or an `android:id` below the element's first line. `UnusedResources` reports a file
  resource at its root element instead of line 1.
- `NotSibling` reports "Cannot be relative to self" only when the reference uses the same
  `@+id/` or `@id/` form as the declared id, as Lint does.
- `InconsistentLayout` now follows Lint's LayoutConsistencyDetector: it reports only ids that Kotlin or
  Java source reads through `R.id`, words an id found in fewer than half the configurations as
  "only present in", and drops an id when any declaration is suppressed with `tools:ignore`.
- The oracle JVMs (one-shot and persistent) now launch from `JAVA_HOME` first, like the FIR
  daemon and the preflight check, and Leyden AOT eligibility is decided from the JDK that
  actually runs. Before, a `JAVA_HOME` JDK 25+ with an older `java` on `PATH` never used the
  AOT cache, and the reverse passed AOT flags an older JDK rejects.
- One-shot oracle launches on a JDK 25+ no longer start an AOT profile recording that nothing
  finalizes. Each run used to leave an unfinished profile of 100+ MB and the build lock behind.
- `SerialVersionUIDInSerializableClass` (Go and FIR) now follows detekt 1.23.8: it fires only
  when a declared supertype entry's text is exactly `Serializable`. Qualified
  `java.io.Serializable`, `Externalizable`, and classes that are Serializable only
  through a parent class, a library class or `Throwable` are no longer reported.
- `StateFlowMutableLeak` no longer reports the single-line typed explicit backing
  field form `val s: StateFlow<T> field: MutableStateFlow<T> = ...`, which the
  grammar recovers as a property typed with the field type.
- Scans with the default cache could replay stale results after a Krit build
  changed rule logic: the Gradle profile cache (`librarymodel-cache`), the type
  index cache, the resource index cache and the XML parse cache were versioned
  by hand only, so a dev build read another build's entries and results could
  differ from a `--no-cache` scan. These caches now also record the build and
  are emptied when another build wrote them. Releases keep reusing them across runs of one binary.
- Findings caches ignored module SDK levels and Gradle library facts. Raising
  one module's `minSdk` in its build script left cached layout findings (for
  example `KeyboardInaccessibleWidget`) and per-file findings from SDK-aware
  rules unchanged. The SDK levels of every module and the Gradle library facts
  are now part of the Android phase and per-file analysis cache keys, and the
  findings bundle fingerprint covers the SDK levels.

- Cached manifest findings were keyed on the manifest's bytes alone. Two
  modules with byte-identical stub manifests shared one entry, so one module
  could replay the other's result (including its file path), so a cached scan
  could report fewer `RtlEnabledManifest` findings than `--no-cache`. The key now includes the manifest path and the module's `res/`
  files and build script that manifest rules read, and the project bundle
  covers them too.

### Added
- `--only-scope` and `--exclude-scope` select rules by scope: `file` for per-file rules and `project` for
  cross-file, module, Android and Gradle rules (individual dispatcher scopes also work). A build system can run
  the per-file rules per module and the project rules once. `--list-rules --format json` reports each rule's
  `scope`. See `docs/hermetic-builds.md`.
- `--reproducible` makes JSON reports byte-identical across runs, caches, worker counts and checkout paths:
  `durationMs` is 0, cache statistics and perf timings are omitted, and suggested-edit targets are relative.
- `SuspiciousIndentation` ports Android Lint's check: a statement indented as if it belonged to the
  previous statement or to an unbraced `if`, `else` or loop, an unbraced body at the column of its
  control statement, and an indentation string that differs from the line above. It reports at most
  one problem per block and skips lambda bodies and braced `when` branches, as Lint does.
- New list options, all empty by default, let a project name its own conventions:
  `RunBlockingInTest.realDispatcherMarkers` (strings that mark a test class as using
  real dispatchers), `StateFlowMutableLeak.testSupportClassNames` (class-name substrings
  that mark test support, in both the Go rule and the FIR checker), and
  `ComposeRawTextLiteral.nonProductionPathMarkers` and
  `ComposeTextFieldMissingLabel.nonProductionPathMarkers` (path substrings for sources
  that are not user-facing).
- `KotlinOptionsDsl` (supply-chain, on by default) flags the `kotlinOptions` Gradle
  DSL, which is an error since Kotlin 2.2.0 (every property since 2.3.0), on the
  `kotlinOptions` identifier. The severity follows the project's Kotlin version.
  The semantic fix rewrites a `kotlinOptions` block or statement in a Kotlin task
  configuration to `compilerOptions` when every statement has a direct
  equivalent. Sources are in `docs/kotlin-gradle-migration-rules.md`.
- `JvmDefaultFlag` (supply-chain, off by default) flags the `-Xjvm-default` compiler
  argument, deprecated in Kotlin 2.2.0 for `-jvm-default` and `jvmDefault`. It reports
  only when the project's Kotlin version is known to be 2.2.0 or newer. The semantic
  fix rewrites `all-compatibility`, `all` and `disable` to the documented new values.
- `ObsoleteKotlinVersionFlags` (supply-chain, off by default) flags Kotlin compiler flags
  that are stable or unsupported in the project's Kotlin version (`-Xwhen-guards`,
  `-Xcontext-parameters`, `-Xexplicit-backing-fields`, `-Xcontext-receivers`, ...),
  language versions 1.9 and below, and obsolete `kotlin.experimental.tryK2` and
  `kotlin.mpp.*` keys in `gradle.properties`. Each message states the Kotlin version
  that changed the construct. The semantic fix deletes redundant list items and removed
  properties.
- `KotlinAndroidPluginWithBuiltInKotlin` (supply-chain, on by default) flags the
  `org.jetbrains.kotlin.android` / `kotlin-android` plugin in a project on AGP 9.0 or
  newer, where built-in Kotlin makes applying it fail. It needs a known AGP version,
  honors `android.builtInKotlin=false`, and skips `apply false` declarations. The
  semantic fix deletes the plugin line when nothing else needs migrating first.
- `ComposeCompilerRemovedFlags` (supply-chain, on by default) flags the Compose compiler
  Gradle options and feature flags that Kotlin 2.4.0 turns into errors
  (`enableStrongSkippingMode`, `enableIntrinsicRemember`, `stabilityConfigurationFile`,
  `ComposeFeatureFlag.StrongSkipping`, ...) and the flags it deprecates. The severity
  follows the Kotlin version. The semantic fix deletes the `= true` lines that restate
  the defaults.
- `KaptUsage` (supply-chain, off by default) reports kapt plugins and `kapt(...)`
  configurations. It names the KSP variant for Room, Moshi codegen, Glide, Dagger and
  Hilt, points to `com.android.legacy-kapt` on AGP 9, and suggests
  `kapt.include.compile.classpath=false`. It has no autofix.
- Cached Gradle findings now depend on the `gradle.properties` files a build script can
  see, so editing one refreshes them.
- `ExplicitBackingFieldCandidate` (style, off by default) flags a private mutable
  backing property (`MutableStateFlow`, `MutableSharedFlow`, `MutableLiveData`,
  `MutableState`, mutable collections, `Channel`) whose only job is to back one
  read-only public property (`get() = _x`, `= _x`, `_x.asStateFlow()`,
  `_x.asSharedFlow()`), which a Kotlin 2.4 explicit backing field
  (`val x: StateFlow<T> field = MutableStateFlow(v)`) expresses in one declaration.
  It reports on the private property and is skipped when the project's Kotlin
  language version is below 2.4; an unknown version reports at low confidence.
  A FIR checker resolves the types and asks the compiler whether the feature is
  enabled. The Go rule offers a semantic autofix only for class members whose
  uses are all member accesses of the backing property (`_x.value = ...`); other
  shapes are report-only. `asStateFlow()` hides the mutable type from callers
  that downcast, which an explicit backing field does not.
- `docs/perf.md` lists every on-disk cache with what it holds, its key inputs
  and what invalidates it.
- `PreferToOverPairSyntax` (style, off by default) flags `Pair(a, b)` constructor
  calls that read better as `a to b`, like detekt. A FIR checker confirms the
  call resolves to `kotlin.Pair`. It has no autofix, as in detekt.
- Persisted caches are now keyed on the `krit-fir` jar contents, the oracle
  protocol and fact-schema versions, and the running build, so replacing the
  jar or upgrading Krit never serves facts or findings from an older oracle.
  This covers `types.json`, the oracle store, the FIR check cache, the
  per-file analysis cache, the Android and cross-file findings caches, the
  cross-file index, and the findings bundle.
- `--absolute-paths` reports finding file paths as absolute. Without it, every
  output format reports a file under `--base-path` (default: the first scan
  path) relative to it with forward slashes, and any other file as absolute.
  Previously Kotlin files kept the spelling of the scan path while XML,
  manifest and Gradle files were absolute, so output changed with `krit .`
  versus `krit /abs/path`. See `docs/integrations.md`.
- Detekt import: Krit reads an auto-discovered or explicit `detekt.yml`
  (`--detekt-config`, `--no-detekt-config`, `detekt:` in `krit.yml`) and
  honours its thresholds, rule switches and baseline. See
  `docs/detekt-migration.md`.
- Module sharding for whole-repository oracle compiles, enabled with
  `KRIT_MODULE_SHARDS=<n|auto>`. See `docs/module-sharding.md`.
- `findingsdiff` and `oraclediff` developer tools, and a parity script that
  compares sharded and whole-repository runs.

- Android lint parity audit against Lint 32.4.1, checked on synthetic projects
  with the Lint command line: `TooManyViews` and `TooDeepLayout` report the
  80th element and the first tenth-level element; `RtlHardcoded` and
  `RtlSymmetry` follow `RtlDetector` and treat RTL as on unless
  `supportsRtl="false"`; `WrongCase` flags only `Fragment`, `RequestFocus`,
  `Include` and `Merge`; `WrongFolder` flags a `<resources>` file in a layout
  folder; `WrongRegion`, `UseAlpha2` and `LocaleFolder` use Lint's locale
  tables; `UnknownIdInLayout`, `NotSibling`, `DisableBaselineAlignment`,
  `ProtectedPermissions`, `HardcodedDebugMode`, `ViewHolder` and
  `WrongViewCast` follow their detectors (`WrongViewCast` finally receives the
  layout index); the TV manifest rules apply only to TV apps and
  `UnsupportedTvHardware`, `ImpliedTouchscreenHardware` and `MissingTvBanner`
  are new; `IntentFilterExportRequired` and `MissingExportedFlag` answer to
  `IntentFilterExportedReceiver` and report a warning below targetSdk 31;
  `UnusedAttribute` checks every `android:` attribute against Lint's API
  database; `UnusedResources` is Lint's reachability check (the old naming
  heuristic is gone) and `UnusedIds` is new; `InsecureBaseConfigurationResource`
  and `AcceptsUserCertificates` read `res/xml` network security configs;
  `TextViewEditsResource` is Lint's layout check. Lint's tables are generated by
  `tools/lintdata`; see `tools/lintdata/README.md`.

### Changed
- `ScopeOnParameterizedClass` no longer reports javax/jakarta.inject `@Singleton`, `@Reusable`, Hilt, Metro,
  kotlin-inject, Guice or project scopes on generic classes: Dagger, Metro and Guice key scoped bindings
  by the full type (checked with Dagger 2.60.1, Metro 1.5.1 and Guice 5.1.0), so `Cache<String>` and
  `Cache<Integer>` get separate instances. The rule reports Koin `@Singleton`/`@Single`, EJB
  `@Singleton` and JSF managed-bean scopes, where the container keys by class, and CDI normal scopes
  (`@ApplicationScoped`, `@RequestScoped`, `@SessionScoped`, `@ConversationScoped`, faces
  `@ViewScoped`/`@FlowScoped` and `@TransactionScoped`) as definition errors on generic beans.
  FIR also resolves custom CDI normal scopes. javax/jakarta.inject `@Singleton` is no longer
  reported, even under CDI.
- The Go `AvoidReferentialEquality` rule trusts resolver-provided operand types,
  including through wildcard imports, and applies `forbiddenTypePatterns` to
  them. Adds guards for explicit `String` lookalikes and ambiguous top-level
  bindings under inherited or extension receivers. When neither operand has a
  resolved type, only string literals and templates supply evidence; unresolved
  identifiers stay silent. Adds a vararg correction limited to an immediately
  enclosing function's parameter with no potentially shadowing declaration.
- Krit finds `java` and `javac` under `JAVA_HOME` before `PATH` everywhere it launches a JVM or checks for one
  (jar download, Java facts helper, `--doctor`).
- On Linux, `-j` and other worker defaults follow `GOMAXPROCS`, which tracks the cgroup CPU quota, and memory
  sizing reads the cgroup memory limit instead of the host's. A scan in a memory-limited cgroup sets a Go soft
  memory limit of three quarters of it unless `GOMEMLIMIT` is set.
- `--no-fir-daemon` also runs the type oracle's cache misses one-shot, so an in-process scan
  (`--no-daemon --no-fir-daemon`) leaves no krit-fir daemon behind.
- Cold `--fir` scans start parsing, and building the cross-file code index, while the oracle
  JVM compiles, instead of after it. The phases take the background result only when their
  input is identical, so findings do not change.
- One-shot oracle compiles run on the parallel collector with a larger initial heap, code cache
  and metaspace threshold, which cuts JVM CPU time by roughly a tenth on a large Android project.
- RTL layout rules decide when they apply the way Lint 32.4.1 does, checked
  against its command line on synthetic projects (supportsRtl unset, true or
  false by targetSdk, minSdk, app or library). An app with `supportsRtl` unset
  counts as RTL on, so `RtlHardcodedResource`, `RtlSymmetryResource` and
  `RtlSuperscriptResource` report again; only `supportsRtl="false"`, a
  targetSdk below 17 (an unset targetSdk is the minSdk, an unset minSdk is 1) or
  a main manifest with no `<application>` turns them off. A library module is
  judged by the apps that include it, transitively and without test or
  annotation-processor configurations, so one including app that enables RTL is
  enough; a library no app includes reports nothing. `RtlHardcodedResource`
  gives Lint's "Consider adding" advice while the minSdk is below 17, even with
  RTL off, and "Consider replacing" from 17. `RtlCompatResource` uses the same
  per-module rule. `RtlEnabledManifest` now counts only `textAlignment` and
  start/end `gravity` as RTL attributes (not `paddingStart` and friends), checks
  library manifests too, and reports `supportsRtl="true"` below targetSdk 17
  by the including app's target. Unlike Lint's command line, which shares the
  "manifest sets supportsRtl" state between modules, a library is checked on its
  own, as in Gradle's per-module analysis.
- The embedded Kotlin compiler in `krit-fir` is now 2.4.21 (was 2.4.20), and
  `krit-rule-api` and the custom-rule plugin's `kotlin-compiler` dependency move
  with it. The vendored `FirProviderImpl` is unchanged between the two
  releases. The jar content changes, so persisted caches are rebuilt.
- `UnnecessaryLet` follows detekt 1.23's rule of the same name, checked on
  synthetic sources against detekt 1.23.8 with type resolution. It reports
  `let expression can be omitted` when the lambda parameter is unused, the body
  is a member chain on the parameter, or the call is not part of a call chain,
  and `let expression can be replaced with a simple if` for an unused `?.let`
  whose value is not used. A new FIR checker resolves the call to `kotlin.let`,
  counts parameter uses by symbol and reads Unit-returning lambdas; the Go rule
  stays as the syntactic fallback. The rule is still off by default.
- `UnnecessaryApply` follows detekt 1.23.8. It reports `receiver.apply { stmt }`
  when the lambda holds one assignment, `this`, reference or dot-qualified
  statement, exactly one name reference reads the apply receiver implicitly, and
  the apply value is not used. It no longer reports empty blocks or blocks that
  never read the receiver, and it has no autofix. A FIR checker resolves the
  implicit receivers; without it the Go rule reports only the subset where
  `this` is the single receiver reference. Messages are detekt's.
- Android Lint parity. These rules now follow the Lint 32.4.1 detectors, checked
  against the Lint CLI: `ShortAlarm` (`setRepeating` only, trigger below 5000
  and interval below 60000, reported on the argument), `TrulyRandom` (weak RNG
  on API 18 and lower, once per project), `GrantAllUris` (manifest check),
  `InvalidUsesTagAttribute` (Android Auto `<uses name>`), `GradleGetter`,
  `EasterEgg` (comment-end unicode escape), `CutPasteId` (repeated
  `findViewById`), and `RtlCompat` (`RtlCompatResource`, `textAlignment` and
  start/end attributes). `ExportedService`, `ExportedReceiver` and
  `ExportedContentProvider` are manifest checks
  (`ExportedServiceManifest`, `ExportedReceiverManifest`,
  `ExportedContentProviderManifest`).
- Renamed krit-only checks that borrowed Lint IDs for other behavior:
  `HardcodedRandomSeed` (was `TrulyRandom`), `InvalidUsesFeatureRequiredManifest`
  (was `InvalidUsesTagAttributeManifest`), `HiddenFeatureComment` (was
  `EasterEgg`), `IdTypePrefixMismatchResource` (was `CutPasteIdResource`),
  `GroovySetterInKotlinDsl` (was `GradleGetter`), `ApplyPluginInKotlinDsl` (was
  `GradleIdeError`). Config blocks and suppressions under retired IDs
  (`NestedScrolling`, `ExportedService`, `ExportedReceiver`,
  `ExportedContentProvider`, `RtlCompatManifest`, `AppIndexingErrorManifest`,
  `AppIndexingWarningManifest`, `GradleIdeError`, `CutPasteIdResource`,
  `InvalidUsesTagAttributeManifest`) keep working; config validation reports
  them as warnings. `UsesSdkManifest` is off by default because Lint 32 dropped
  `UsesMinSdkAttributes`.
- `tools:ignore` is handled by one module for the manifest, values, layout and
  `xml/` resources: IDs match ignoring case, `all` and the `AndroidLint` prefix
  work, a `<resources>` root covers its children, and the attribute covers
  later lines of a multi-line element.
- Style rules follow detekt 1.23.8 more closely (conditions, messages,
  anchors, config keys) and gain the `UntilInsteadOfRangeTo` rule. New options:
  `allowOperators`, `excludeAnnotatedClasses`, `ignoreUnwrappableLines`,
  `ignoreIdioms`, `reportMutableCollections`, `negativeFunctionNameParts`,
  `values`. See `docs/detekt-migration.md`.
- Finding spans for `!!`, `?.` and `?:` are exactly the operator token, not the
  rest of the call chain, so `UnnecessaryNotNullOperator`,
  `UnnecessarySafeCall` and `UnsafeCallOnNullableType` highlight two bytes.
- Findings for receiver-qualified calls now anchor on the called method name
  (`BitmapDecodeWithoutOptions`, `UpperLowerInvariantMisuse`,
  `CursorLoopWithColumnIndexInLoop`, `LocaleGetDefaultForFormatting`, and other
  call rules) instead of the receiver. Line numbers of multi-line chains are
  unchanged.
- `SafeCast` reports "Redundant cast after is-check" and also answers to the
  suppression names `RedundantCast` and `UselessCast`; its ID is unchanged.
  `ComposeSideEffectInComposition` calls an indexed write a "state map write",
  `IgnoredReturnValue` states that a pure stdlib function (not an annotation)
  is the basis, and `LayoutAutofillHintMismatch` labels its missing-hint report
  as such.
- `StringFormatTrivialResource` skips strings that carry a translator note (a
  preceding XML comment, or a `note` or `description` attribute) and describes
  the string as only a placeholder. It still has no source usage scan, so a
  string expanded with `expandTemplate` is only recognised through its name or
  note. The resource cache version is bumped.
- Directory scans skip documentation samples under `examples` and `references`
  directories that have no build file between them and the Git root and no `src`
  directory. See `docs/configuration.md`.
- With `-v`, the oracle module map lists the Kotlin files no Gradle project or
  settings module owns, with the usual causes.
- Cold runs are faster: a callable-name index, parallel file I/O and a
  persistent file-hash memo (`.krit/cache/hash-memo.gob`).
- Android layout rules now analyse every layout variant (`layout`,
  `layout-land`, `layout-sw600dp`, and same-named layouts in other modules)
  and report each variant's own file. Before, one variant per layout name was
  checked. The resource cache version is bumped, so the first run after
  upgrading rebuilds it.
- Compiler smart-cast verdicts that depend on a property declared in another
  module are dropped, using the exported Gradle model to map files to modules.
- Oracle type output is deterministic: dependencies and sources are indexed in
  a fixed order, and star projections render as `*`.
- Scan diagnostics: Kotlin compiler-settings notes print once, the
  missing-classpath warning says which entries need a build and which need a
  model re-export, and FIR verdict rows with no verdicts are omitted.

### Fixed
- `@Suppress` now applies in more shapes: classes and constructors the grammar
  splits from their annotation, property accessors, and annotated-expression
  parses of a class.
- Many rule false positives were fixed across the Kotlin, Android resource,
  manifest, Compose, DI and security rule sets, with the matching FIR checkers
  kept in agreement with the Go rules.
- `ElseCaseInsteadOfExhaustiveWhen` counts `object` variants and does not treat
  same-named leaves of a nested sealed type as covered by the outer type's
  branches. `PrintlnInProduction` honours a function's own KDoc in the FIR
  checker (the declaration source starts at the comment) and accepts
  `System.out` KDoc and `print`-named helpers.
- `@SuppressLint("Id")` now suppresses findings, including by Android lint
  ID, and `@Suppress`, `@SuppressWarnings` and `@SuppressLint` accept named and
  array arguments. A `tools:ignore` on the `<manifest>` element now suppresses
  file-level manifest findings, and `tools:ignore` is honoured in `values`,
  `xml` and `menu` resource files.
- Android lint rule behavior changes: `InconsistentLayout` compares view ids
  across layout configurations (as Android Lint does) instead of root types and
  view counts; `NfcTechWhitespace` checks `res/xml` tech lists, `Proguard`
  checks Gradle scripts, and `LibraryCustomView` checks library-module layouts,
  none of them Kotlin string literals any more; `MergeRootFrame` only fires for
  layouts pulled in with `<include>`; `MockLocationManifest` skips debug and
  test source-set manifests; `ResourceType` no longer reports `R.styleable`
  indices; `NewApi`, `ResourceAsColor`, `ResourceType` and `RtlAware` need a
  framework call target when the compiler oracle resolves the call; `Recycle`
  also checks inferred locals; `RemoteVersion` and `NewerVersionAvailable`
  report on the dependency line; `MissingPermission` accepts a `try` that
  catches `SecurityException`; `InsecureBaseConfigurationManifest` only reads
  `<base-config>`; `RequiredSizeResource` always skips `TableRow`.

### Changed
- `CyclomaticComplexMethod` counts like detekt: `else` entries are never
  counted, and with `ignoreSimpleWhenEntries` a `when` with no braced entry
  adds 1. The message names the mode. `IgnoredReturnValue` flags a discarded
  `Flow`, `Sequence`, `Stream` or function value only for stdlib and coroutines
  callees (or known side-effect-free names when the target is unresolved), and
  `RunTestWithDelay` only for a lone `delay` in a test that shares no test
  scheduler. `HardcodedDateFormat` reports month and day name patterns only
  when they are stored or exchanged, and always for week-based fields.
  `AssertNullableWithNotNullAssertion` now needs oracle diagnostics so it can
  skip a `!!` the compiler marks unnecessary.
- Round five: `StringFormatCountResource` counts only `%` specifiers (never
  `^1` tokens) and names mixed placeholder styles separately. `WrongImport`
  reports a framework id import only when a bare identifier resolves to it, and
  leaves unused imports to `UnusedImport`. The oracle keys sealed variants on
  the bare parent name, so generic subtypes such as `Parent<Nothing>` are all
  listed for `ElseCaseInsteadOfExhaustiveWhen`. `UnnecessaryNotNullOperator` and
  `UnnecessarySafeCall` ignore the compiler verdict for locals assigned in
  lambdas. `IgnoredReturnValue` never reports stdlib collection mutators and
  names its basis. `ClickableViewAccessibilityResource` skips views whose
  description source code sets. `LargeClass` excludes nested classes.
  `RtlEnabledManifest` skips harness manifests and non-application modules.
  `SwallowedException` skips catches marked intentional by name, function or
  comment.
- Finding positions now point at the flagged node: the real 1-based column for
  Kotlin, XML and Gradle findings (some used to report column 1), the
  declaration name instead of its first annotation, the `!!` operator, the
  call in a multi-line chain, and the attribute in a layout. A trailing
  `// krit:ignore` comment applies on the reported line, so it now goes on the
  declaration line, not the annotation line.
- Every finding now carries `startByte`/`endByte` (before, only a few did). See
  `docs/integrations.md` for what the span covers.
- Findings anchor on the flagged node more precisely: `SafeCast` on the
  redundant `as`, `SerialVersionUIDInSerializableClass` on the class name,
  `TooGenericExceptionThrown` on the constructor call, `RedundantSuspendModifier`
  on `suspend`, `OkHttpCallExecuteSync` on `execute`, the Compose
  clickable/role rules on the `.clickable` call name, and attribute-based
  layout rules (`SpUsageResource`, `PxUsageResource`, `RtlHardcodedResource`,
  `RtlSymmetryResource`, id rules and others) on the attribute. Resource cache
  version 9: cached layouts from older builds lacked attribute positions, so
  those rules fell back to the element start.
- `RtlHardcodedResource`, `RtlSymmetryResource` and `RtlSuperscriptResource`
  report only when an application manifest sets `android:supportsRtl="true"`,
  matching `RtlEnabledManifest`. A scan with no application manifest still
  reports.
- `Suspicious0dpResource` flags a `0dp` size along the parent's orientation
  axis (height in a vertical `LinearLayout`, width in a horizontal one) on a
  child without `layout_weight`. It no longer flags the other axis, fixed sizes
  or `Space`.
- `DuplicateIncludedIdsResource` resolves an include inside the including
  layout's qualifier set and names the variant (`in layout-land`). An
  `<include>`'s own `android:id` no longer hides the included root id, as in Lint.
  The FIR `SerialVersionUIDInSerializableClass` reports a backticked
  `serialVersionUID` like Go and detekt.
  `LayoutEditTextMissingImportance` fires only for an `EditText` with an
  `inputType` or `hint`. `WrongImport`, `RunBlockingInTest` and
  `UnreachableCatchBlock` messages are more accurate.
- JSON `confidence` is always written with a decimal point (`1.0`).
  `ElseCaseInsteadOfExhaustiveWhen` no longer offers an autofix, and `effort`
  is `trivial` or `local` for single-attribute resource, manifest and Gradle
  findings. Resource cache version 8.
- Opt-ins from any source set now apply to the whole-repo compile (#972),
  avoiding false "This API is experimental" errors. This can hide a missing
  opt-in in another source set.
- Incremental FIR checking is now on by default (#740). After the first
  whole-module compile, `--fir` scans compile only the edited files and the
  files that depend on them, for the FIR checkers and for the type oracle. Set
  `KRIT_FIR_INCREMENTAL=0` to turn it off. On a 182-file module, warm `--fir`
  daemon scans took about 2 s instead of 6–10 s, with the same findings. The
  first compile of a module also builds the binary snapshot, so it takes
  longer.

### Added
- The incremental FIR bundle (`.krit/fir-incremental/`) now also keeps the type
  oracle's per-file facts (bundle format version 2). A restarted daemon answers
  its first warm `--fir` scan from slice compiles instead of compiling the whole
  module for the oracle. Version 1 bundles are replaced on the first check.
- Incremental FIR checking persists its state, verdicts and binary
  snapshot to `.krit/fir-incremental/<compile key>/` (supplied by krit, bypassed by
  `--no-cache`), so a restarted daemon or a one-shot run serves its first check
  after a few edits by slice compile instead of a cold full check. Corrupt,
  truncated, version- or key-mismatched bundles fall back to the cold check.
- Linux fanotify watcher backend for the daemon: filesystem-wide kernel mark
  with FAN_REPORT_DFID_NAME + open_by_handle_at path resolution.
  Auto-selected on Linux when `CAP_SYS_ADMIN` + `CAP_DAC_READ_SEARCH` are
  available (`setcap cap_sys_admin,cap_dac_read_search+ep $(command -v krit)`),
  fsnotify otherwise. Skips the per-directory inotify walk on huge repos.
- `--watch-backend` flag on `krit serve`: `auto` (default), `fsnotify`, or
  `fanotify`.
- `docs/perf.md`: catalog of every performance-tuning option, with when to
  use each and what you give up.
- `make test-fanotify` Docker target + CI job to exercise the fanotify
  path on platforms / users without the required capabilities.

- Kotlin compiler settings from the Gradle model (`languageVersion`,
  `apiVersion`, opt-ins, semantic `-X` / `-XXLanguage:` flags, progressive mode)
  now reach the embedded compiler in both `krit-fir` and `krit-types`, so a
  project pinned to an older language level is no longer analyzed under the
  Kotlin 2.4 defaults. The Gradle model is now schema 2 (schema 1 is still
  read). Settings are applied from an allowlist; Krit-owned output, classpath,
  plugin, friend-path and fragment arguments stay protected, and anything not
  applied is reported. They are part of the oracle, FIR, daemon and findings
  bundle cache identities. See `docs/kotlin-compiler-settings.md`.
- `krit-fir` module compilations (`analyzeModules`) now apply only allowlisted
  semantic settings from a module's `compilerArgs` (language/API level, opt-ins,
  progressive mode and the flags listed in `docs/kotlin-compiler-settings.md`).
  Other extra compiler arguments that were previously passed through to the
  compiler are no longer applied; each is reported in the response `warning`.
  Krit-owned arguments were already dropped.

### Changed
- Verbose `--fir` and `--perf` output now explains why incremental FIR
  checking did not engage, including planner reasons, compiler-error gating,
  `KRIT_FIR_INCREMENTAL` opt-out, and `--no-cache` (#971).
- `krit daemon: ready` startup line now reports the resolved watcher
  backend (`watcher=fsnotify` / `watcher=fanotify` / `watcher=off`).
- Homebrew, Scoop, and Maven Central (`krit-rule-api`) publishing only
  runs for stable `vX.Y.Z` tags; nightly and release-candidate tags stop
  at the GitHub prerelease.
- The Gradle plugin's version is now the krit release it downloads by
  default, instead of a separately hardcoded tool version.
- winget manifests are generated on each stable release and submitted to
  microsoft/winget-pkgs when a `WINGET_TOKEN` secret is configured.

### Fixed
- `--no-cache` now also bypasses the `.krit/fir-cache` FIR check cache
  (no reads, no writes), in-process and daemon-delegated (#745).
- Gradle plugin binary download requested archives the release never
  published (`krit-<os>-<arch>.tar.gz`) and could not unpack the Windows
  zip. It now fetches `krit_<version>_<os>_<arch>` archives (including the
  musl build on Alpine), extracts `.tar.gz` and `.zip`, and requires a
  matching `checksums.txt` entry.
- VS Code extension download requested a raw `krit-lsp-<os>-<arch>` binary
  the release never published, and `krit.version: latest` built an invalid
  URL. It now resolves the release tag, downloads and verifies the release
  archive against `checksums.txt`, and extracts `krit-lsp`.

## [0.2.0] - 2026-05-11

### Added
- Broad lint-rule coverage across detekt-style, Android Lint-style, and Krit-specific checks
- LSP server for editor diagnostics and code intelligence
- MCP server for AI agent integration
- `--diff` flag: only report findings on lines changed since a git ref (perfect for PRs)
- `--init` flag: generate starter krit.yml with recommended defaults
- `--doctor` flag: check environment (Java, config, tools, krit-types)
- `--completions` flag: print shell completions (bash, zsh, fish)
- `--warnings-as-errors` flag: elevate warning severity to error
- Per-rule `excludes` glob patterns (detekt-compatible: `**/test/**`, `**/*Test.kt`)
- BracesOnIfStatements/When `consistent` mode
- UnderscoresInNumericLiterals `acceptableLength` config (default: 5 digits)
- Colored terminal output (auto-detected, respects NO_COLOR)
- Binary autofix for image assets (WebP conversion, PNG optimization, animated detection)
- Compiler diagnostic support through the Kotlin Analysis API oracle
- Compiled tree-sitter queries for performance
- Symbol-indexed dispatch (array lookup, no string hashing)
- Parser pooling via sync.Pool with incremental reparse in LSP
- Confidence field on findings, with rule-specific confidence carried through
  the v2 registry
- GitHub Action with SARIF upload, checksum verification, and `diff` input
- Gradle plugin (check/format/baseline, reports DSL, per-source-set tasks)
- VS Code extension with binary auto-download
- Neovim and IntelliJ editor configs, Cursor MCP config
- GoReleaser with GPG signing, SBOM, SLSA provenance
- Homebrew, Scoop, winget package manager support
- One-shot install scripts (bash with gum TUI + PowerShell)
- Pre-commit hook support (.pre-commit-hooks.yaml)
- MkDocs documentation site with migration guide
- Playground projects (Kotlin web service + Android app) for integration testing
- Tests, fixtures, regression checks, and benchmarks
- Shell completions for bash, zsh, fish (embedded via go:embed)
- Version injection via ldflags (GoReleaser + Makefile)

### Fixed
- Daemon startup hang (added 30s timeout to scanner.Scan)
- EmptyFunctionBlock false positive on expression bodies
- MagicNumber duplicate reporting for typed literals
- UtilityClass false positive on interfaces
- MissingSuperCall false positive on interface overrides
- PropertyUsedBeforeDeclaration moved to node-dispatched analysis on class bodies
- ClassNaming false positive on backtick-enclosed test class names
- ElseCaseInsteadOfExhaustiveWhen requires type resolver (0 FPs without)
- MatchingDeclarationName false positive on .kts script files
- JSON fixable flag only true when finding has actual fix data
- Exit code regression: always exit 1 on findings
- ReturnCount excludeReturnFromLambda uses AST ancestry check

### Architecture
- Node-dispatched, line-pass, aggregate, and project-scope rule pipelines
- Specialized pipelines for manifest, resource, Gradle, and icon analysis
- Source-level type inference plus optional JVM-backed Kotlin Analysis API/FIR helpers
- Module-aware dead code detection with bloom filter
- Cross-file reference indexing (Kotlin + Java + XML)
- Per-rule file exclusion via glob patterns in dispatcher
- Precision-oriented validation against large Kotlin/Android codebases
