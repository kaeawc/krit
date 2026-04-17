# Android Lint — Manifest Sub-Cluster

Rules that analyze `AndroidManifest.xml`. Implemented via the `ManifestRule` interface in the manifest pipeline (not the Kotlin AST dispatcher).

**Status: 46 shipped, 3 planned**

---

## Shipped Rules

### Security (14)

| Rule ID | Brief |
|---|---|
| AllowBackup | Missing or true allowBackup attribute |
| BackupRules | Missing backup configuration |
| CleartextTraffic | usesCleartextTraffic enabled |
| ExportedPreferenceActivity | Exported PreferenceActivity vulnerable to fragment injection |
| ExportedService | Exported service without required permission |
| ExportedWithoutPermission | Exported component without required permission |
| HardcodedDebugMode | Hardcoded value of android:debuggable in manifest |
| InsecureBaseConfiguration | Missing networkSecurityConfig on API 28+ |
| IntentFilterExportRequired | Component with intent-filter missing android:exported (API 31+) |
| MissingExportedFlag | Component with intent-filter missing android:exported (API 31+) |
| ProtectedPermissions | Using system app permission |
| ServiceExported | Exported service does not require permission |
| UniquePermission | Custom permission collides with system permission |
| UnprotectedSMSBroadcastReceiver | SMS receiver without permission protection |
| UnsafeProtectedBroadcastReceiver | Exported receiver for protected broadcast without permission |
| UseCheckPermission | Exported service with sensitive action but no permission |
| SystemPermission | Requesting dangerous system permission |

### Correctness (16)

| Rule ID | Brief |
|---|---|
| DuplicateActivity | Activity registered more than once |
| DuplicateUsesFeature | Duplicate `<uses-feature>` declaration |
| GradleOverrides | SDK version in manifest overridden by Gradle |
| InvalidUsesTagAttribute | Invalid android:required value on `<uses-feature>` |
| ManifestOrder | `<application>` appears before `<uses-permission>` or `<uses-sdk>` |
| ManifestTypo | Typos in manifest element tags |
| MissingRegistered | Missing registered class |
| MissingVersion | Missing versionCode or versionName on `<manifest>` |
| MockLocation | ACCESS_MOCK_LOCATION in non-debug manifest |
| MultipleUsesSdk | More than one `<uses-sdk>` element |
| OldTargetApi | Target SDK version is too old |
| UnpackedNativeCode | Missing extractNativeLibs=false with native libraries |
| UsesMinSdkAttributes | Missing `<uses-sdk>` element in manifest |
| WrongManifestParent | Element declared under wrong parent in manifest |

### Usability / Features (15)

| Rule ID | Brief |
|---|---|
| AppIndexingError | VIEW intent filter missing http/https data scheme |
| AppIndexingWarning | Browsable intent filter missing VIEW action |
| DeviceAdmin | Malformed Device Admin |
| FullBackupContent | Invalid fullBackupContent or dataExtractionRules |
| GoogleAppIndexingDeepLinkError | Deep link data element with scheme but no host |
| GoogleAppIndexingWarning | No activity with deep link support |
| LocaleConfigMissing | Missing locale config reference |
| MissingApplicationIcon | Missing android:icon on `<application>` |
| MissingLeanbackLauncher | Leanback feature without LEANBACK_LAUNCHER activity |
| MissingLeanbackSupport | Leanback feature without touchscreen opt-out |
| MipmapIcons | Launcher icon should use @mipmap/ not @drawable/ |
| PermissionImpliesUnsupportedHardware | Permission implies hardware feature not declared optional |
| RtlCompat | Missing supportsRtl with targetSdkVersion >= 17 |
| RtlEnabled | Missing supportsRtl=true on `<application>` |
| UnsupportedChromeOsHardware | Hardware feature unsupported on Chrome OS not marked optional |

---

## Planned Rules (3)

These three require improvements to the manifest XML parser before they can be implemented.

| Rule ID | AOSP Detector | Description |
|---|---|---|
| Mipmap | ManifestDetector.MIPMAP | Flags `@drawable/` references to launcher icons that should use `@mipmap/` (broader variant of MipmapIcons that catches all reference sites) |
| UniquePermission (full) | ManifestDetector.UNIQUE_PERMISSION | Detects custom permission name collisions across library manifests during manifest merge (requires merged manifest view) |
| SystemPermissions | SystemPermissionsDetector | Detects usage of permissions that are only grantable to system apps, requires updated permission taxonomy |

All three require parser improvements: either cross-manifest merging support or an expanded permission allowlist derived from the AOSP platform manifest.
