# Roadmap clusters

This directory breaks the cluster overview docs (`roadmap/49`–`64` and
the architecture/SDLC survey) into one file per individual concept so
each rule or feature can be tracked, discussed, and scheduled
independently.

The numbered parent docs in `roadmap/49-*.md` through `roadmap/64-*.md`
remain the authoritative per-cluster summaries. Each cluster folder
here contains:

- `README.md` — cluster index with a one-line description of every
  concept.
- Per-concept files — name, dispatch node, positive and negative
  Kotlin example, infra reuse notes, and links.

## Historical references

- [`roadmap/regressions/`](../regressions/README.md) — regression files
  from the retired roadmap loop. Kept as lessons learned on integration
  testing thresholds, noise floors, and automation limits.

## Clusters

### Security

- [`security/syntactic/`](security/syntactic/) — parent: [`49`](../49-security-rules-syntactic.md)
- [`security/call-shape/`](security/call-shape/) — parent: [`50`](../50-security-rules-call-shape.md)
- [`security/taint/`](security/taint/) — parent: [`51`](../51-security-rules-taint.md) (deferred)

### Correctness / quality

- [`accessibility/`](accessibility/) — parent: [`52`](../52-accessibility-rules.md)
- [`i18n/`](i18n/) — parent: [`53`](../53-i18n-l10n-rules.md)
- [`compose/`](compose/) — parent: [`54`](../54-compose-correctness-rules.md)
- [`di-hygiene/`](di-hygiene/) — parent: [`55`](../55-di-hygiene-rules.md)
- [`concurrency/`](concurrency/) — parent: [`56`](../56-concurrency-coroutines-rules.md)
- [`database/`](database/) — parent: [`57`](../57-database-room-rules.md)
- [`release-engineering/`](release-engineering/) — parent: [`58`](../58-release-engineering-rules.md)
- [`testing-quality/`](testing-quality/) — parent: [`59`](../59-testing-quality-rules.md)

### Small-new-infra

- [`privacy/`](privacy/) — parent: [`60`](../60-privacy-data-handling-rules.md)
- [`observability/`](observability/) — parent: [`61`](../61-observability-rules.md)
- [`resource-cost/`](resource-cost/) — parent: [`62`](../62-resource-cost-rules.md)
- [`supply-chain/`](supply-chain/) — parent: [`63`](../63-supply-chain-hygiene-rules.md)
- [`licensing/`](licensing/) — parent: [`64`](../64-licensing-legal-rules.md)

### Android Lint coverage

- [`android-lint/`](android-lint/) — AOSP Android Lint rule coverage:
  181 shipped across 5 pipelines (source, manifest, resource, gradle,
  icon), 52 planned, fixture gap tracking. Supersedes roadmap items
  14, 22, and 24.

### Core infrastructure (greenfield targets)

- [`core-infra/`](core-infra/) — architectural changes to the scanner,
  dispatcher, registry, pipeline, and caching substrate; no parent
  overview doc yet.

### Beyond per-file rules (new)

- [`architecture/`](architecture/) — module-graph and package-level
  enforcement; no parent overview doc yet.
- [`di-graph/`](di-graph/) — whole-graph DI binding validation,
  orthogonal to the per-file `di-hygiene` cluster.
- [`sdlc/`](sdlc/) — PR diff mode, codemods, documentation
  generators, metrics, LSP integration. Split into sub-clusters
  per subsystem.
- [`build-integration/`](build-integration/) — exposing krit's
  symbol-level analysis to external build tools (ABI hashing,
  used-symbol extraction, cross-module dead code, symbol-level
  impact, query daemon); no parent overview doc yet.
- [`fir-checkers/`](fir-checkers/) — delivery vehicle for Kotlin
  FIR-based checkers; JVM runner first (krit as CLI driver), then
  dual-packaged as a kotlinc compiler plugin. Uses
  [ZacSweers/metro](https://github.com/ZacSweers/metro) as the
  reference implementation. No parent overview doc yet.

## Concept file template

Every per-concept file follows the same shape:

```markdown
# <ConceptName>

**Cluster:** [<cluster>](../README.md) · **Status:** planned ·
**Severity:** warning · **Default:** active

## What it catches

<1–3 sentences.>

## Example — triggers

```kotlin
// positive case
```

## Example — does not trigger

```kotlin
// negative case
```

## Implementation notes

- Dispatch: `<tree-sitter node type>`
- Infra reuse: `<file paths in internal/rules/>`
- Related: `<other concepts or rules>`

## Links

- Parent overview: [`../../XX-<name>.md`](../../XX-<name>.md)
- Related: `<links>`
```

A concept is "planned" by default. Promote to "in progress" when an
owner picks it up, "shipped" when it lands in `internal/rules/`, and
"declined" with a reason if we choose not to build it.

---

# Scoreboard (updated 2026-04-16)

Progress across every concept file in this directory. Cross-checked
against registered rules in `internal/rules/` (rules whose concept
name matches a `Register(&X{BaseRule: BaseRule{"X", ...}})` call
are counted as shipped regardless of the concept file's status marker).

| Cluster | Total | ✅ Shipped | 🟡 In-progress | ⏳ Planned | ⏸️ Deferred |
|---|---:|---:|---:|---:|---:|
| [accessibility](./accessibility/) | 16 | 16 | 0 | 0 | 0 |
| [android-lint](./android-lint/) | 225 | 170 | 3 | 52 | 0 |
| [architecture](./architecture/) | 8 | 8 | 0 | 0 | 0 |
| [build-integration](./build-integration/) | 6 | 0 | 1 | 5 | 0 |
| [compose](./compose/) | 19 | 19 | 0 | 0 | 0 |
| [concurrency](./concurrency/) | 20 | 20 | 0 | 0 | 0 |
| [core-infra](./core-infra/) | 13 | 4 | 0 | 9 | 0 |
| [database](./database/) | 21 | 3 | 3 | 15 | 0 |
| [di-graph](./di-graph/) | 5 | 1 | 0 | 4 | 0 |
| [di-hygiene](./di-hygiene/) | 22 | 4 | 3 | 15 | 0 |
| [i18n](./i18n/) | 18 | 3 | 3 | 12 | 0 |
| [licensing](./licensing/) | 15 | 3 | 3 | 9 | 0 |
| [observability](./observability/) | 18 | 3 | 4 | 11 | 0 |
| [onboarding](./onboarding/) | 16 | 16 | 0 | 0 | 0 |
| [performance-infra](./performance-infra/) | 12 | 9 | 0 | 2 | 1 |
| [privacy](./privacy/) | 14 | 13 | 0 | 1 | 0 |
| [release-engineering](./release-engineering/) | 18 | 18 | 0 | 0 | 0 |
| [resource-cost](./resource-cost/) | 21 | 21 | 0 | 0 | 0 |
| [rule-quality](./rule-quality/) | 27 | 0 | 2 | 25 | 0 |
| [sdlc](./sdlc/) | 33 | 5 | 1 | 27 | 0 |
| [security](./security/) | 58 | 5 | 1 | 39 | 13 |
| [supply-chain](./supply-chain/) | 21 | 2 | 4 | 15 | 0 |
| [testing-quality](./testing-quality/) | 19 | 18 | 1 | 0 | 0 |
| **Total** | **638** | **361** | **28** | **235** | **14** |

Progress: **361/638 shipped** (57%), 28 in-progress, 235 planned, 14 deferred.

Notes:
- `android-lint/` scoreboard counts concept-file items (225), not registered rules (181) — sub-clusters include 44 planned AOSP rules without concept files yet.
- `onboarding/` TUI shipped (`krit init` via bubbletea) but concept files not yet updated.
- `coroutines.go` has 29 registered rules; 9 predate the cluster concept system.
- `core-infra/` and `rule-quality/` expanded 2026-04-16 with vibe-detector findings.

## Per-cluster breakdown

### `accessibility/`  (16 concepts: 16✅ 0🟡 0⏳ 0⏸️)

**Shipped:** `AnimatorDurationIgnoresScale`, `ComposeClickableWithoutMinTouchTarget`, `ComposeDecorativeImageContentDescription`, `ComposeIconButtonMissingContentDescription`, `ComposeRawTextLiteral`, `ComposeSemanticsMissingRole`, `ComposeTextFieldMissingLabel`, `LayoutAutofillHintMismatch`, `LayoutClickableWithoutMinSize`, `LayoutEditTextMissingImportance`, `LayoutImportantForAccessibilityNo`, `LayoutMinTouchTargetInButtonRow`, `StringNotSelectable`, `StringRepeatedInContentDescription`, `StringSpanInContentDescription`, `ToastForAccessibilityAnnouncement`

### `architecture/`  (8 concepts: 0✅ 0🟡 8⏳ 0⏸️)

**Planned:** `FanInFanOutHotspot`, `GodClassOrModule`, `LayerDependencyViolation`, `ModuleDependencyCycle`, `PackageDependencyCycle`, `PackageNamingConventionDrift`, `PublicApiSurfaceSnapshot`, `PublicToInternalLeakyAbstraction`

### `compose/`  (19 concepts: 19✅ 0🟡 0⏳ 0⏸️)

**Shipped:** `ComposeColumnRowInScrollable`, `ComposeDerivedStateMisuse`, `ComposeDisposableEffectMissingDispose`, `ComposeLambdaCapturesUnstableState`, `ComposeLaunchedEffectWithoutKeys`, `ComposeModifierBackgroundAfterClip`, `ComposeModifierClickableBeforePadding`, `ComposeModifierFillAfterSize`, `ComposeModifierPassedThenChained`, `ComposeMutableDefaultArgument`, `ComposeMutableStateInComposition`, `ComposePreviewAnnotationMissing`, `ComposePreviewWithBackingState`, `ComposeRememberSaveableNonParcelable`, `ComposeRememberWithoutKey`, `ComposeSideEffectInComposition`, `ComposeStatefulDefaultParameter`, `ComposeStringResourceInsideLambda`, `ComposeUnstableParameter`

### `concurrency/`  (20 concepts: 20✅ 0🟡 0⏳ 0⏸️)

**Shipped:** `ChannelReceiveWithoutClose`, `CollectInOnCreateWithoutLifecycle`, `CollectionsSynchronizedListIteration`, `ConcurrentModificationIteration`, `CoroutineScopeCreatedButNeverCancelled`, `DeferredAwaitInFinally`, `FlowWithoutFlowOn`, `GlobalScopeLaunchInViewModel`, `LaunchWithoutCoroutineExceptionHandler`, `MainDispatcherInLibraryCode`, `MutableStateInObject`, `SharedFlowWithoutReplay`, `StateFlowCompareByReference`, `StateFlowMutableLeak`, `SupervisorScopeInEventHandler`, `SynchronizedOnBoxedPrimitive`, `SynchronizedOnNonFinal`, `SynchronizedOnString`, `VolatileMissingOnDcl`, `WithContextInSuspendFunctionNoop`

Note: `coroutines.go` has 29 registered rules total; 9 predate the cluster concept system.

### `database/`  (21 concepts: 3✅ 3🟡 15⏳ 0⏸️)

**Shipped:** `DaoNotInterface`, `DaoWithoutAnnotations`, `JdbcPreparedStatementNotClosed`

**In progress:** `EntityPrimaryKeyNotStable`, `ForeignKeyWithoutOnDelete`, `JdbcResultSetLeakedFromFunction`

**Planned:** `EntityMutableColumn`, `RoomConflictStrategyReplaceOnFk`, `RoomDatabaseVersionNotBumped`, `RoomEntityChangedMigrationMissing`, `RoomExportSchemaDisabled`, `RoomFallbackToDestructiveMigration`, `RoomMigrationUsesExecSqlWithInterpolation`, `RoomMultipleWritesMissingTransaction`, `RoomQueryMissingWhereForUpdate`, `RoomQueryWithLikeMissingEscape`, `RoomRelationWithoutIndex`, `RoomReturnTypeFlowWithoutDistinct`, `RoomSelectStarWithoutLimit`, `RoomSuspendQueryInTransaction`, `SqliteCursorWithoutClose`

### `di-graph/`  (5 concepts: 1✅ 0🟡 4⏳ 0⏸️)

**Shipped:** `CrossModuleScopeConsistency`

**Planned:** `DeadBindings`, `DiCycleDetection`, `DiGraphExport`, `WholeGraphBindingCompleteness`

### `di-hygiene/`  (22 concepts: 4✅ 3🟡 15⏳ 0⏸️)

**Shipped:** `AnvilContributesBindingWithoutScope`, `AnvilMergeComponentEmptyScope`, `BindsMismatchedArity`, `HiltEntryPointOnNonInterface`

**In progress:** `BindsInsteadOfProvides`, `BindsReturnTypeMatchesParam`, `ComponentMissingModule`

**Planned:** `HiltInstallInMismatch`, `HiltSingletonWithActivityDep`, `InjectOnAbstractClass`, `IntoMapDuplicateKey`, `IntoMapMissingKey`, `IntoSetDuplicateType`, `IntoSetOnNonSetReturn`, `LazyInsteadOfDirect`, `MetroGraphFactoryMissingAbstract`, `MissingJvmSuppressWildcards`, `ModuleWithNonStaticProvides`, `ProviderInsteadOfLazy`, `ScopeOnParameterizedClass`, `SingletonOnMutableClass`, `SubcomponentNotInstalled`

### `i18n/`  (18 concepts: 3✅ 3🟡 12⏳ 0⏸️)

**Shipped:** `LocaleConfigMissing`, `LocaleConfigStale`, `LocaleDefaultForCurrency`

**In progress:** `HardcodedNumberFormat`, `LocaleGetDefaultForFormatting`, `PluralsBuiltWithIfElse`

**Planned:** `HardcodedDateFormat`, `PluralsMissingZero`, `StringConcatForTranslation`, `StringContainsHtmlWithoutCDATA`, `StringResourceMissingPositional`, `StringResourcePlaceholderOrder`, `StringTemplateForTranslation`, `StringTrailingWhitespace`, `TextDirectionLiteralInString`, `TranslatableMarkupMismatch`, `UnicodeNormalizationMissing`, `UpperLowerInvariantMisuse`

### `licensing/`  (15 concepts: 4✅ 2🟡 9⏳ 0⏸️)

**Shipped:** `CopyrightYearOutdated`, `DependencyLicenseIncompatible`, `DependencyLicenseUnknown`, `MissingSpdxIdentifier`

**In progress:** `LgplStaticLinkingInApk`, `OptInMarkerExposedPublicly`

**Planned:** `NoticeFileOutOfDate`, `OptInMarkerNotRecognised`, `OptInWithoutJustification`, `OssLicensesNotIncludedInAndroid`, `RequiresOptInWithoutLevel`, `RequiresOptInWithoutMessage`, `SpdxIdentifierInvalid`, `SpdxIdentifierMismatchWithProject`, `SuppressedWarningWithoutJustification`

### `observability/`  (18 concepts: 3✅ 4🟡 11⏳ 0⏸️)

**Shipped:** `LogLevelGuardMissing`, `LogWithoutCorrelationId`, `LoggerWithoutLoggerField`

**In progress:** `LoggerInterpolatedMessage`, `LoggerStringConcat`, `MdcAcrossCoroutineBoundary`, `MdcPutNoRemove`

**Planned:** `MetricCounterNotMonotonic`, `MetricNameMissingUnit`, `MetricTagHighCardinality`, `MetricTimerOutsideBlock`, `NullableStructuredField`, `SpanAttributeWithHighCardinality`, `SpanStartWithoutFinish`, `StructuredLogKeyMixedCase`, `TraceIdLoggedAsPlainMessage`, `UnstructuredErrorLog`, `WithContextWithoutTracingContext`

### `onboarding/`  (16 concepts: 0✅ 0🟡 16⏳ 0⏸️)

**Planned:** `CascadeMap`, `ControversialRulesRegistry`, `GumAutofixPass`, `GumBaseline`, `GumComparisonTable`, `GumControversialRules`, `GumIntegrationTest`, `GumProfileScan`, `GumProfileSelection`, `GumWriteConfig`, `ProfileTemplates`, `TuiArchitecture`, `TuiLiveCodePreview`, `TuiRealtimeFindingCount`, `TuiSplitPaneExplorer`, `TuiThresholdSliders`

### `performance-infra/`  (9 concepts: 8✅ 0🟡 0⏳ 1⏸️)

**Shipped:** `ColumnarFindingStorage`, `FlatNodeRepresentation`, `OracleCrashResilience`, `PerFileArenaAllocation`, `PrecompiledQueryExpansion`, `StringInterning`, `WorkerPinnedParallelScan`, `ZeroCopyNodeText`

**Deferred:** `DispatcherFlatTreeMigration`

### `privacy/`  (14 concepts: 13✅ 0🟡 1⏳ 0⏸️)

**Shipped:** `AdMobInitializedBeforeConsent`, `AnalyticsCallWithoutConsentGate`, `AnalyticsEventWithPiiParamName`, `AnalyticsUserIdFromPii`, `BiometricAuthNotFallingBackToDeviceCredential`, `ClipboardOnSensitiveInputType`, `ContactsAccessWithoutPermissionUi`, `CrashlyticsCustomKeyWithPii`, `FirebaseRemoteConfigDefaultsWithPii`, `LocationBackgroundWithoutRationale`, `LogOfSharedPreferenceRead`, `PlainFileWriteOfSensitive`, `SharedPreferencesForSensitiveKey`

**Planned:** `ScreenshotNotBlockedOnLoginScreen`

### `release-engineering/`  (18 concepts: 18✅ 0🟡 0⏳ 0⏸️)

**Shipped:** `BuildConfigDebugInLibrary`, `BuildConfigDebugInverted`, `CommentedOutCodeBlock`, `CommentedOutImport`, `DebugToastInProduction`, `GradleBuildContainsTodo`, `HardcodedEnvironmentName`, `HardcodedLocalhostUrl`, `HardcodedLogTag`, `MergeConflictMarkerLeftover`, `NonAsciiIdentifier`, `OpenForTestingCallerInNonTest`, `PrintStackTraceInProduction`, `PrintlnInProduction`, `TestFixtureAccessedFromProduction`, `TestOnlyImportInProduction`, `TimberTreeNotPlanted`, `VisibleForTestingCallerInNonTest`

### `resource-cost/`  (21 concepts: 21✅ 0🟡 0⏳ 0⏸️)

**Shipped:** `BitmapDecodeWithoutOptions`, `BufferedReadWithoutBuffer`, `ComposePainterResourceInLoop`, `ComposeRememberInList`, `CursorLoopWithColumnIndexInLoop`, `DatabaseInstanceRecreated`, `DatabaseQueryOnMainThread`, `HttpClientNotReused`, `ImageLoadedAtFullSizeInList`, `ImageLoaderNoMemoryCache`, `LazyColumnInsideColumn`, `OkHttpCallExecuteSync`, `OkHttpClientCreatedPerCall`, `PeriodicWorkRequestLessThan15Min`, `RecyclerAdapterStableIdsDefault`, `RecyclerAdapterWithoutDiffUtil`, `RecyclerViewInLazyColumn`, `RetrofitCreateInHotPath`, `RoomLoadsAllWhereFirstUsed`, `WorkManagerNoBackoff`, `WorkManagerUniquePolicyKeepButReplaceIntended`

### `rule-quality/`  (21 concepts: 0✅ 0🟡 21⏳ 0⏸️)

**Planned:** `AstRewriteAlsoCouldBeApply`, `AstRewriteAndroidCorrectnessBatch`, `AstRewriteAndroidSourceExtraBatch`, `AstRewriteAudit`, `AstRewriteAvoidReferentialEquality`, `AstRewriteCastNullable`, `AstRewriteCharArrayToStringCall`, `AstRewriteCheckResult`, `AstRewriteCollapsibleIfStatements`, `AstRewriteCommitPrefEdits`, `AstRewriteCommitTransaction`, `AstRewriteDataClassImmutable`, `AstRewriteDoubleMutability`, `AstRewriteErrorUsageWithThrowable`, `AstRewriteExitOutsideMain`, `AstRewriteExplicitCollectionElementAccessMethod`, `AstRewriteForbiddenAnnotation`, `AstRewriteForbiddenComment`, `ConfigurableTestPaths`, `TestPathConfigSchema`, `TestPathMigration`

### `sdlc/`  (31 concepts: 5✅ 0🟡 26⏳ 0⏸️)

**Shipped:** `BaselineDrift`, `ConventionPluginDeadCode`, `DeadCodeBatchRemoval`, `FixtureHarvesting`, `RenameRefactoring`

**Planned:** `EditorconfigRealityDrift`, `ModuleTemplateConformance`, `CodebaseWalkthroughGenerator`, `DependencyGraphExport`, `KdocLinkValidation`, `ModuleReadmeGeneration`, `SampleAnnotationFreshness`, `GoToTest`, `HoverShowsRuleDocs`, `InlineCodelens`, `QuickFixPreviewsWithDiff`, `CodebaseHealthScore`, `PerModuleScorecards`, `RuleLevelTimeSeries`, `SloRules`, `ApiMigrationAssist`, `CodemodCommand`, `BlastRadiusScoring`, `ChurnComplexityRiskMap`, `DiffModeReporting`, `FailOnNewMode`, `ReviewerRouting`, `MockInventory`, `TestSelection`, `TestToCodeMapping`, `UntestedPublicApi`

### `security/`  (58 concepts: 5✅ 1🟡 39⏳ 13⏸️)

**Shipped:** `ContentProviderQueryWithSelectionInterpolation`, `FileFromUntrustedPath`, `GoogleApiKeyInResources`, `HardcodedBearerToken`, `HardcodedGcpServiceAccount`

**In progress:** `HardcodedAwsAccessKey`

**Planned:** `HardcodedJwt`, `HardcodedSlackWebhook`, `JdbcStatementExecute`, `LogPii`, `PrintStackTraceInRelease`, `ProcessBuilderShellArg`, `RoomRawQueryStringConcat`, `RuntimeExecUnsafeShape`, `SqlInjectionRawQuery`, `TempFileWorldReadable`, `UnprotectedDynamicReceiver`, `ZipSlipUnchecked`, `AllowAllHostnameVerifier`, `BroadcastReceiverExportedFlagMissing`, `DeepLinkMissingAutoVerify`, `DisableCertificatePinning`, `GsonPolymorphicFromJson`, `HardcodedHttpUrl`, `HardcodedSecretKey`, `ImplicitPendingIntent`, `InsecureTrustManager`, `JacksonDefaultTyping`, `JavaObjectInputStream`, `NetworkSecurityConfigDebugOverrides`, `OkHttpDisableSslValidation`, `PrngFromSystemTime`, `RsaNoPadding`, `StartActivityWithUntrustedIntent`, `StaticIv`, `WeakKeySize`, `WeakMacAlgorithm`, `WeakMessageDigest`, `WebViewAllowContentAccess`, `WebViewAllowFileAccess`, `WebViewDebuggingEnabled`, `WebViewFileAccessFromFileUrls`, `WebViewMixedContentAllowAll`, `WebViewUniversalAccessFromFileUrls`, `XmlExternalEntity`

**Deferred:** `CommandInjection`, `EncryptWithoutAuthentication`, `IntentRedirection`, `JsonPolymorphicUnsafeType`, `LdapInjection`, `LogInjection`, `OpenRedirect`, `PathTraversal`, `SignatureVerificationBypass`, `SqlInjection`, `UnsafeDeserialization`, `UnsafeIntentLaunch`, `XpathInjection`

### `supply-chain/`  (21 concepts: 2✅ 4🟡 15⏳ 0⏸️)

**Shipped:** `AllProjectsBlock`, `CompileSdkMismatchAcrossModules`

**In progress:** `ApplyPluginTwice`, `ConfigurationsAllSideEffect`, `ConventionPluginAppliedToWrongTarget`, `DependenciesInRootProject`

**Planned:** `DependencyDynamicVersion`, `DependencyFromBintray`, `DependencyFromHttp`, `DependencyFromJcenter`, `DependencySnapshotInRelease`, `DependencyVerificationDisabled`, `DependencyWithoutGroup`, `GradleWrapperValidationAction`, `JvmTargetMismatch`, `KotlinVersionMismatchAcrossModules`, `MissingGradleChecksums`, `VersionCatalogBuildSrcMismatch`, `VersionCatalogDuplicateVersion`, `VersionCatalogRawVersionInBuild`, `VersionCatalogUnused`

### `testing-quality/`  (19 concepts: 18✅ 1🟡 0⏳ 0⏸️)

**Shipped:** `AssertEqualsArgumentOrder`, `AssertNullableWithNotNullAssertion`, `AssertTrueOnComparison`, `MixedAssertionLibraries`, `MockWithoutVerify`, `RelaxedMockUsedForValueClass`, `RunBlockingInTest`, `RunTestWithDelay`, `RunTestWithThreadSleep`, `SharedMutableStateInObject`, `SpyOnDataClass`, `TestDispatcherNotInjected`, `TestFunctionReturnValue`, `TestInheritanceDepth`, `TestNameContainsUnderscore`, `TestWithOnlyTodo`, `TestWithoutAssertion`, `VerifyWithoutMock`

**In progress:** `MockFinalClassWithoutMockMaker`

