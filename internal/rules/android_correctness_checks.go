package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

type OverrideAbstractRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *OverrideAbstractRule) Confidence() float64 { return api.ConfidenceMedium }

var abstractClassRequirements = map[string][]string{"Service": {"onBind"}, "BroadcastReceiver": {"onReceive"}, "ContentProvider": {"onCreate", "query", "insert", "update", "delete", "getType"}}

type ParcelCreatorRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ParcelCreatorRule) Confidence() float64 { return api.ConfidenceMedium }

type SwitchIntDefRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *SwitchIntDefRule) Confidence() float64 { return api.ConfidenceMedium }

type TextViewEditsRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *TextViewEditsRule) Confidence() float64 { return api.ConfidenceMedium }

type DeprecatedRule struct {
	FlatDispatchBase
	AndroidRule
}

type deprecatedAPIEntry struct {
	Pattern string
	Message string
}

var deprecatedApis = []deprecatedAPIEntry{{"AsyncTask", "AsyncTask is deprecated as of API 30. Use java.util.concurrent or Kotlin coroutines instead."}, {"IntentService", "IntentService is deprecated as of API 30. Use WorkManager or JobIntentService instead."}, {"PreferenceActivity", "PreferenceActivity is deprecated as of API 29. Use PreferenceFragmentCompat instead."}, {"CursorLoader", "CursorLoader is deprecated as of API 28. Use Room with LiveData or Flow instead."}, {"LocalBroadcastManager", "LocalBroadcastManager is deprecated. Use LiveData, Flow, or other observable patterns instead."}, {"TabActivity", "TabActivity is deprecated as of API 13. Use tabs with Fragment/ViewPager instead."}, {"ActivityGroup", "ActivityGroup is deprecated as of API 13. Use Fragment-based navigation instead."}, {"getRunningTasks", "getRunningTasks is deprecated as of API 21. It returns only the caller's own tasks for privacy."}, {"DefaultHttpClient", "DefaultHttpClient is deprecated as of API 22. Use HttpURLConnection or OkHttp instead."}, {"AndroidHttpClient", "AndroidHttpClient is deprecated as of API 22. Use HttpURLConnection or OkHttp instead."}}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *DeprecatedRule) Confidence() float64 { return api.ConfidenceMedium }

type ResourceTypeRule struct {
	FlatDispatchBase
	AndroidRule
}

var resourceMethodExpected = map[string]string{"getString": "string", "getText": "string", "getQuantityString": "plurals", "getStringArray": "array", "getIntArray": "array", "getDrawable": "drawable", "setImageResource": "drawable", "setImageDrawable": "drawable", "setContentView": "layout", "inflate": "layout", "getColor": "color", "getColorStateList": "color", "getDimension": "dimen", "getDimensionPixelSize": "dimen", "getDimensionPixelOffset": "dimen", "getBoolean": "bool", "getInteger": "integer", "getAnimation": "anim", "getLayout": "layout"}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ResourceTypeRule) Confidence() float64 { return api.ConfidenceMedium }

type ResourceAsColorRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ResourceAsColorRule) Confidence() float64 { return api.ConfidenceMedium }

// ioCallNames is the set of known IO/network type and method names whose
// presence in a @MainThread function indicates a likely thread violation.
var ioCallNames = map[string]bool{
	"HttpURLConnection": true, "OkHttpClient": true,
	"FileInputStream": true, "FileOutputStream": true,
	"BufferedReader": true, "BufferedWriter": true,
	"URLConnection": true, "HttpClient": true,
	"Retrofit": true, "FileReader": true, "FileWriter": true,
	"RandomAccessFile": true, "openConnection": true,
	"Socket": true, "ServerSocket": true, "DatagramSocket": true,
}

type SupportAnnotationUsageRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *SupportAnnotationUsageRule) Confidence() float64 { return api.ConfidenceMedium }

type AccidentalOctalRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *AccidentalOctalRule) Confidence() float64 { return api.ConfidenceMedium }

type AppCompatMethodRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *AppCompatMethodRule) Confidence() float64 { return api.ConfidenceMedium }

type CustomViewStyleableRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *CustomViewStyleableRule) Confidence() float64 { return api.ConfidenceHigher }

type InnerclassSeparatorRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *InnerclassSeparatorRule) Confidence() float64 { return api.ConfidenceMedium }

type OnClickRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *OnClickRule) Confidence() float64 { return api.ConfidenceMedium }

type PropertyEscapeRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *PropertyEscapeRule) Confidence() float64 { return api.ConfidenceMedium }

type ShortAlarmRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ShortAlarmRule) Confidence() float64 { return api.ConfidenceMedium }

type LocalSuppressRule struct {
	FlatDispatchBase
	AndroidRule
}

var knownLintIssueIDs = map[string]bool{"AaptCrash": true, "NewApi": true, "InlinedApi": true, "Override": true, "UnusedResources": true, "HardcodedText": true, "MissingTranslation": true, "ExtraTranslation": true, "MissingPermission": true, "ContentDescription": true, "ObsoleteLayoutParam": true, "ViewHolder": true, "LogConditional": true, "SdCardPath": true, "Wakelock": true, "SetJavaScriptEnabled": true, "ExportedService": true, "PackagedPrivateKey": true, "ValidFragment": true, "ViewConstructor": true, "WrongImport": true, "ServiceCast": true, "LayoutInflation": true, "ShowToast": true, "PackageManagerGetSignatures": true, "UseSparseArrays": true, "UseValueOf": true, "LongLogTag": true, "LogTagMismatch": true, "UnlocalizedSms": true, "ViewTag": true, "ShortAlarm": true, "UniqueConstants": true, "ShiftFlags": true, "AccidentalOctal": true, "AppCompatMethod": true, "CheckResult": true, "CommitPrefEdits": true, "CommitTransaction": true, "CustomViewStyleable": true, "CutPasteId": true, "DalvikOverride": true, "DefaultLocale": true, "Deprecated": true, "DeviceAdmin": true, "DuplicateActivity": true, "DuplicateIds": true, "DuplicateIncludedIds": true, "DuplicatePlatformClasses": true, "DuplicateUsesFeature": true, "ExtraText": true, "FullBackupContent": true, "GradleCompatible": true, "GradleDependency": true, "GradleDeprecated": true, "GradleDynamicVersion": true, "GradleGetter": true, "GradleIdeError": true, "GradleOverrides": true, "GradlePath": true, "IllegalResourceRef": true, "InconsistentArrays": true, "InconsistentLayout": true, "InnerclassSeparator": true, "InvalidResourceFolder": true, "ManifestOrder": true, "ManifestTypo": true, "MissingApplicationIcon": true, "MissingId": true, "MissingRegistered": true, "MissingVersion": true, "MockLocation": true, "MultipleUsesSdk": true, "NestedScrolling": true, "NotSibling": true, "UnknownId": true, "ObjectAnimatorBinding": true, "OnClick": true, "OverrideAbstract": true, "ParcelCreator": true, "PluralsCandidate": true, "PropertyEscape": true, "ProtectedPermissions": true, "Range": true, "ReferenceType": true, "Registered": true, "RequiredSize": true, "ResAuto": true, "ResourceAsColor": true, "ResourceType": true, "ScrollViewCount": true, "ScrollViewSize": true, "ServiceExported": true, "SetTextI18n": true, "SimpleDateFormat": true, "SpUsage": true, "StopShip": true, "StringFormatCount": true, "StringFormatInvalid": true, "StringFormatMatches": true, "SupportAnnotationUsage": true, "SwitchIntDef": true, "TextFields": true, "TextViewEdits": true, "UnpackedNativeCode": true, "UnusedAttribute": true, "UnusedNamespace": true, "UseCompoundDrawables": true, "UsesMinSdkAttributes": true, "WrongCall": true, "WrongCase": true, "WrongFolder": true, "WrongManifestParent": true, "WrongRegion": true, "WrongThread": true, "WrongViewCast": true, "Assert": true, "SQLiteString": true, "LocalSuppress": true, "AddJavascriptInterface": true, "GetInstance": true, "EasterEgg": true, "ExportedContentProvider": true, "ExportedReceiver": true, "ExportedPreferenceActivity": true, "GrantAllUris": true, "HardcodedDebugMode": true, "InsecureBaseConfiguration": true, "SecureRandom": true, "TrustedServer": true, "UnprotectedSMSBroadcastReceiver": true, "UnsafeProtectedBroadcastReceiver": true, "UseCheckPermission": true, "WorldReadableFiles": true, "WorldWriteableFiles": true, "DisableBaselineAlignment": true, "DrawAllocation": true, "FieldGetter": true, "FloatMath": true, "HandlerLeak": true, "InefficientWeight": true, "MergeRootFrame": true, "NestedWeights": true, "Overdraw": true, "Recycle": true, "TooDeepLayout": true, "UselessLeaf": true, "ClickableViewAccessibility": true, "LabelFor": true, "ByteOrderMark": true, "RelativeOverlap": true, "IconColors": true, "IconDensities": true, "IconDipSize": true, "IconDuplicates": true, "IconDuplicatesConfig": true, "IconExpectedSize": true, "IconExtension": true, "IconLauncherShape": true, "IconLocation": true, "IconMissingDensityFolder": true, "IconMixedNinePatch": true, "IconNoDpi": true, "IconXmlAndPng": true, "ConvertToWebp": true, "GifUsage": true, "AppCompatResource": true, "AppIndexingError": true, "AppIndexingWarning": true, "BackButton": true, "ButtonCase": true, "ButtonOrder": true, "ButtonStyle": true, "GoogleAppIndexingDeepLinkError": true, "GoogleAppIndexingWarning": true, "NegativeMargin": true, "RtlCompat": true, "RtlEnabled": true, "RtlHardcoded": true, "RtlSymmetry": true, "RtlSuperscript": true, "AllowBackup": true, "AlwaysShowAction": true, "InvalidUsesTagAttribute": true, "OldTargetApi": true, "ExpiredTargetSdkVersion": true, "PermissionImpliesUnsupportedHardware": true, "UnsupportedChromeOsHardware": true, "MissingLeanbackLauncher": true, "MissingLeanbackSupport": true, "Typos": true, "TypographyDashes": true, "TypographyEllipsis": true, "TypographyFractions": true, "TypographyOther": true, "TypographyQuotes": true, "UnusedIds": true, "Autofill": true, "RestrictedApi": true, "VisibleForTests": true, "MissingInflatedId": true, "NotificationPermission": true, "ObsoleteSdkInt": true, "all": true}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *LocalSuppressRule) Confidence() float64 { return api.ConfidenceMedium }

// pluralsCountNames is the set of identifiers whose presence as a when-subject
// suggests manual pluralization.
var pluralsCountNames = map[string]bool{
	"count": true, "num": true, "size": true,
	"quantity": true, "amount": true, "number": true,
}

// pluralsStringCalls is the set of call names that suggest string-resource usage.
var pluralsStringCalls = map[string]bool{
	"getString": true, "getQuantityString": true, "format": true,
	"getText": true, "getResources": true,
}

type PluralsCandidateRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *PluralsCandidateRule) Confidence() float64 { return api.ConfidenceMedium }
