# Remaining v1 Registrations

Inventory of rule files in `internal/rules/` that still invoke the v1
`Register` / `RegisterManifest` / `RegisterResource` / `RegisterGradle` helpers
at package `init()` time. The per-file counts below count only actual rule
registrations (i.e. top-level `Register*(&RuleStruct{...})` calls), excluding
the helper function bodies themselves (which internally call `Register(r)`).

## Executive summary

- **Total files with v1 Register calls:** 27
- **Total v1 Register calls (including 5 duplicate re-registrations):** 183
- **Unique rule structs still on v1:** 178

### By Register call

| Call | Count |
|---|---|
| `Register` | 38 |
| `RegisterManifest` | 46 |
| `RegisterResource` | 79 |
| `RegisterGradle` | 20 |

### By rule family (unique rule structs)

| Family | Unique rules | Total Register calls |
|---|---|---|
| ManifestRule | 46 | 46 |
| ResourceRule | 79 | 80 |
| GradleRule | 16 | 20 |
| ModuleAwareRule | 5 | 5 |
| CrossFileRule | 7 | 7 |
| FlatDispatchRule | 1 | 1 |
| LineRule | 12 | 12 |
| Legacy(Check) | 12 | 12 |

### Duplicate registrations

Some rules are registered more than once (same struct, different helpers or
multiple init blocks). These count separately in total calls but once in unique:

- `AppCompatResourceRule` — 2 registrations
- `DynamicVersionRule` — 2 registrations
- `GradlePluginCompatibilityRule` — 2 registrations
- `NewerVersionAvailableRule` — 2 registrations
- `StringIntegerRule` — 2 registrations

---

## Per-file breakdown

### `android_correctness.go`

- **Total v1 calls:** 1
- **Unique rules:** 1

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `Register` | `ScrollViewCountRule` | Legacy(Check) | ConfidenceProvider |  |

### `android_correctness_checks.go`

- **Total v1 calls:** 2
- **Unique rules:** 2

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `Register` | `DalvikOverrideRule` | Legacy(Check) | ConfidenceProvider |  |
| `Register` | `OnClickRule` | Legacy(Check) | ConfidenceProvider |  |

### `android_gradle.go`

- **Total v1 calls:** 19
- **Unique rules:** 15

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `RegisterGradle` | `GradlePluginCompatibilityRule` | GradleRule | ConfidenceProvider | 2x |
| `RegisterGradle` | `StringIntegerRule` | GradleRule | ConfidenceProvider | 2x |
| `RegisterGradle` | `RemoteVersionRule` | GradleRule | ConfidenceProvider |  |
| `RegisterGradle` | `DynamicVersionRule` | GradleRule | ConfidenceProvider | 2x |
| `RegisterGradle` | `GradleOldTargetApiRule` | GradleRule | ConfidenceProvider |  |
| `RegisterGradle` | `DeprecatedDependencyRule` | GradleRule | ConfidenceProvider |  |
| `RegisterGradle` | `MavenLocalRule` | GradleRule | ConfidenceProvider |  |
| `RegisterGradle` | `MinSdkTooLowRule` | GradleRule | ConfidenceProvider |  |
| `RegisterGradle` | `GradleDeprecatedRule` | GradleRule | ConfidenceProvider |  |
| `RegisterGradle` | `GradleGetterRule` | GradleRule | ConfidenceProvider |  |
| `RegisterGradle` | `GradlePathRule` | GradleRule | ConfidenceProvider |  |
| `RegisterGradle` | `GradleOverridesRule` | GradleRule | ConfidenceProvider |  |
| `RegisterGradle` | `GradleIdeErrorRule` | GradleRule | ConfidenceProvider |  |
| `RegisterGradle` | `AndroidGradlePluginVersionRule` | GradleRule | ConfidenceProvider |  |
| `RegisterGradle` | `NewerVersionAvailableRule` | GradleRule | ConfidenceProvider | 2x |

### `android_icons.go`

- **Total v1 calls:** 9
- **Unique rules:** 9

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `Register` | `IconDensitiesRule` | Legacy(Check) | ConfidenceProvider |  |
| `Register` | `IconDipSizeRule` | Legacy(Check) | ConfidenceProvider |  |
| `Register` | `IconDuplicatesRule` | Legacy(Check) | ConfidenceProvider |  |
| `Register` | `GifUsageRule` | Legacy(Check) | ConfidenceProvider |  |
| `Register` | `ConvertToWebpRule` | Legacy(Check) | ConfidenceProvider |  |
| `Register` | `IconMissingDensityFolderRule` | Legacy(Check) | ConfidenceProvider |  |
| `Register` | `IconExpectedSizeRule` | Legacy(Check) | ConfidenceProvider |  |
| `Register` | `IconNoDpiRule` | Legacy(Check) | ConfidenceProvider |  |
| `Register` | `IconDuplicatesConfigRule` | Legacy(Check) | ConfidenceProvider |  |

### `android_manifest_features.go`

- **Total v1 calls:** 13
- **Unique rules:** 13

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `RegisterManifest` | `RtlEnabledManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `RtlCompatManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `AppIndexingErrorManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `AppIndexingWarningManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `GoogleAppIndexingDeepLinkErrorManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `GoogleAppIndexingWarningManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `MissingLeanbackLauncherManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `MissingLeanbackSupportManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `PermissionImpliesUnsupportedHardwareManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `UnsupportedChromeOsHardwareManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `DeviceAdminManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `FullBackupContentManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `MissingRegisteredManifestRule` | ManifestRule | ConfidenceProvider |  |

### `android_manifest_i18n.go`

- **Total v1 calls:** 1
- **Unique rules:** 1

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `RegisterManifest` | `LocaleConfigMissingRule` | ManifestRule | ConfidenceProvider |  |

### `android_manifest_security.go`

- **Total v1 calls:** 14
- **Unique rules:** 14

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `RegisterManifest` | `AllowBackupManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `DebuggableManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `ExportedWithoutPermissionRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `MissingExportedFlagRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `ExportedServiceManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `ExportedPreferenceActivityManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `CleartextTrafficRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `BackupRulesRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `InsecureBaseConfigurationManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `UnprotectedSMSBroadcastReceiverManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `UnsafeProtectedBroadcastReceiverManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `UseCheckPermissionManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `ProtectedPermissionsManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `ServiceExportedManifestRule` | ManifestRule | ConfidenceProvider |  |

### `android_manifest_structure.go`

- **Total v1 calls:** 18
- **Unique rules:** 18

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `RegisterManifest` | `DuplicateActivityManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `WrongManifestParentManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `GradleOverridesManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `UsesSdkManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `MipmapLauncherRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `UniquePermissionRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `SystemPermissionRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `ManifestTypoRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `MissingApplicationIconRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `TargetNewerRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `IntentFilterExportRequiredRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `DuplicateUsesFeatureManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `MultipleUsesSdkManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `ManifestOrderManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `MissingVersionManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `MockLocationManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `UnpackedNativeCodeManifestRule` | ManifestRule | ConfidenceProvider |  |
| `RegisterManifest` | `InvalidUsesTagAttributeManifestRule` | ManifestRule | ConfidenceProvider |  |

### `android_resource_a11y.go`

- **Total v1 calls:** 16
- **Unique rules:** 16

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `RegisterResource` | `HardcodedValuesResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `MissingContentDescriptionResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `LabelForResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `ClickableViewAccessibilityResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `BackButtonResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `ButtonCaseResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `ButtonOrderResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `ButtonStyleResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `LayoutClickableWithoutMinSizeRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `LayoutEditTextMissingImportanceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `LayoutImportantForAccessibilityNoRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `LayoutAutofillHintMismatchRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `LayoutMinTouchTargetInButtonRowRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `StringNotSelectableRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `StringRepeatedInContentDescriptionRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `StringSpanInContentDescriptionRule` | ResourceRule | ConfidenceProvider |  |

### `android_resource_ids.go`

- **Total v1 calls:** 14
- **Unique rules:** 14

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `RegisterResource` | `DuplicateIdsResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `InvalidIdResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `MissingIdResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `CutPasteIdResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `DuplicateIncludedIdsResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `MissingPrefixResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `NamespaceTypoResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `ResAutoResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `UnusedNamespaceResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `IllegalResourceRefResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `WrongCaseResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `WrongFolderResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `InvalidResourceFolderResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `AppCompatResourceRule` | ResourceRule | ConfidenceProvider |  |

### `android_resource_layout.go`

- **Total v1 calls:** 13
- **Unique rules:** 13

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `RegisterResource` | `TooManyViewsResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `TooDeepLayoutResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `UselessParentResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `UselessLeafResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `NestedScrollingResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `ScrollViewCountResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `ScrollViewSizeResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `RequiredSizeResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `OrientationResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `AdapterViewChildrenResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `IncludeLayoutParamResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `UseCompoundDrawablesResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `InconsistentLayoutResourceRule` | ResourceRule | ConfidenceProvider |  |

### `android_resource_rtl.go`

- **Total v1 calls:** 5
- **Unique rules:** 5

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `RegisterResource` | `RtlHardcodedResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `RtlSymmetryResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `RtlSuperscriptResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `RelativeOverlapResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `NotSiblingResourceRule` | ResourceRule | ConfidenceProvider |  |

### `android_resource_style.go`

- **Total v1 calls:** 14
- **Unique rules:** 14

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `RegisterResource` | `PxUsageResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `SpUsageResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `SmallSpResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `InOrMmUsageResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `NegativeMarginResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `Suspicious0dpResourceRule` | ResourceRule | — |  |
| `RegisterResource` | `DisableBaselineAlignmentResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `InefficientWeightResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `NestedWeightsResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `ObsoleteLayoutParamsResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `MergeRootFrameResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `OverdrawResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `AlwaysShowActionResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `StateListReachableResourceRule` | ResourceRule | ConfidenceProvider |  |

### `android_resource_values.go`

- **Total v1 calls:** 17
- **Unique rules:** 17

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `RegisterResource` | `WebViewInScrollViewResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `OnClickResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `TextFieldsResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `UnusedAttributeResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `WrongRegionResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `LocaleConfigStaleResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `MissingQuantityResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `UnusedQuantityResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `ImpliedQuantityResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `StringFormatInvalidResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `StringFormatCountResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `StringFormatMatchesResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `StringFormatTrivialResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `StringNotLocalizableResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `GoogleApiKeyInResourcesRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `InconsistentArraysResourceRule` | ResourceRule | ConfidenceProvider |  |
| `RegisterResource` | `ExtraTextResourceRule` | ResourceRule | ConfidenceProvider |  |

### `android_source.go`

- **Total v1 calls:** 2
- **Unique rules:** 2

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `Register` | `GetSignaturesRule` | LineRule | ConfidenceProvider |  |
| `Register` | `NonInternationalizedSmsRule` | LineRule | ConfidenceProvider |  |

### `android_source_extra.go`

- **Total v1 calls:** 10
- **Unique rules:** 10

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `Register` | `WrongImportRule` | LineRule | ConfidenceProvider |  |
| `Register` | `LayoutInflationRule` | LineRule | ConfidenceProvider |  |
| `Register` | `ViewTagRule` | LineRule | ConfidenceProvider |  |
| `Register` | `TrulyRandomRule` | LineRule | ConfidenceProvider |  |
| `Register` | `MissingPermissionRule` | LineRule | ConfidenceProvider |  |
| `Register` | `WrongConstantRule` | LineRule | ConfidenceProvider |  |
| `Register` | `LocaleFolderRule` | LineRule | ConfidenceProvider |  |
| `Register` | `UseAlpha2Rule` | LineRule | — |  |
| `Register` | `MangledCRLFRule` | LineRule | ConfidenceProvider |  |
| `Register` | `ProguardSplitRule` | LineRule | ConfidenceProvider |  |

### `android_usability.go`

- **Total v1 calls:** 1
- **Unique rules:** 1

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `Register` | `AppCompatResourceRule` | ResourceRule | ConfidenceProvider |  |

### `coroutines.go`

- **Total v1 calls:** 1
- **Unique rules:** 1

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `Register` | `MainDispatcherInLibraryCodeRule` | ModuleAwareRule | ConfidenceProvider, FixableRule, (SetModuleIndex) |  |

### `deadcode.go`

- **Total v1 calls:** 1
- **Unique rules:** 1

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `Register` | `DeadCodeRule` | CrossFileRule | ConfidenceProvider, FixLevelRule, FixableRule |  |

### `deadcode_module.go`

- **Total v1 calls:** 1
- **Unique rules:** 1

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `Register` | `ModuleDeadCodeRule` | ModuleAwareRule | ConfidenceProvider, FixableRule, (SetModuleIndex) |  |

### `di_hygiene.go`

- **Total v1 calls:** 1
- **Unique rules:** 1

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `Register` | `AnvilMergeComponentEmptyScopeRule` | CrossFileRule | ConfidenceProvider, OracleFilterProvider |  |

### `hotspot.go`

- **Total v1 calls:** 1
- **Unique rules:** 1

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `Register` | `FanInFanOutHotspotRule` | CrossFileRule | ConfidenceProvider |  |

### `licensing.go`

- **Total v1 calls:** 1
- **Unique rules:** 1

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `RegisterGradle` | `DependencyLicenseUnknownRule` | GradleRule | ConfidenceProvider |  |

### `package_dependency_cycle.go`

- **Total v1 calls:** 1
- **Unique rules:** 1

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `Register` | `PackageDependencyCycleRule` | ModuleAwareRule | ConfidenceProvider, (SetModuleIndex) |  |

### `package_naming_convention_drift.go`

- **Total v1 calls:** 1
- **Unique rules:** 1

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `Register` | `PackageNamingConventionDriftRule` | FlatDispatchRule | ConfidenceProvider |  |

### `release_engineering.go`

- **Total v1 calls:** 5
- **Unique rules:** 5

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `Register` | `ConventionPluginDeadCodeRule` | ModuleAwareRule | ConfidenceProvider, (SetModuleIndex) |  |
| `Register` | `VisibleForTestingCallerInNonTestRule` | CrossFileRule | ConfidenceProvider |  |
| `Register` | `OpenForTestingCallerInNonTestRule` | CrossFileRule | ConfidenceProvider |  |
| `Register` | `TestFixtureAccessedFromProductionRule` | CrossFileRule | ConfidenceProvider |  |
| `Register` | `TimberTreeNotPlantedRule` | CrossFileRule | ConfidenceProvider |  |

### `supply_chain.go`

- **Total v1 calls:** 1
- **Unique rules:** 1

| Register helper | Rule struct | Family | Mixins | Dup |
|---|---|---|---|---|
| `Register` | `CompileSdkMismatchAcrossModulesRule` | ModuleAwareRule | ConfidenceProvider, (SetModuleIndex) |  |
