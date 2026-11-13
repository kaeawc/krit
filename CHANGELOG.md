# Changelog

All notable changes to Krit will be documented in this file.

## [Unreleased]

### Fixed
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
