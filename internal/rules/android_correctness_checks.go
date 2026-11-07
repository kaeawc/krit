package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
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
	// FQNs lists the classes the pattern may refer to; the file must import or mention one.
	// Empty means the pattern is a method name, matched as a member call.
	FQNs []string
}

var deprecatedApis = []deprecatedAPIEntry{{"AsyncTask", "AsyncTask is deprecated as of API 30. Use java.util.concurrent or Kotlin coroutines instead.", []string{"android.os.AsyncTask"}}, {"IntentService", "IntentService is deprecated as of API 30. Use WorkManager or JobIntentService instead.", []string{"android.app.IntentService"}}, {"PreferenceActivity", "PreferenceActivity is deprecated as of API 29. Use PreferenceFragmentCompat instead.", []string{"android.preference.PreferenceActivity"}}, {"CursorLoader", "CursorLoader is deprecated as of API 28. Use Room with LiveData or Flow instead.", []string{"android.content.CursorLoader"}}, {"LocalBroadcastManager", "LocalBroadcastManager is deprecated. Use LiveData, Flow, or other observable patterns instead.", []string{"androidx.localbroadcastmanager.content.LocalBroadcastManager", "android.support.v4.content.LocalBroadcastManager"}}, {"TabActivity", "TabActivity is deprecated as of API 13. Use tabs with Fragment/ViewPager instead.", []string{"android.app.TabActivity"}}, {"ActivityGroup", "ActivityGroup is deprecated as of API 13. Use Fragment-based navigation instead.", []string{"android.app.ActivityGroup"}}, {"getRunningTasks", "getRunningTasks is deprecated as of API 21. It returns only the caller's own tasks for privacy.", nil}, {"DefaultHttpClient", "DefaultHttpClient is deprecated as of API 22. Use HttpURLConnection or OkHttp instead.", []string{"org.apache.http.impl.client.DefaultHttpClient"}}, {"AndroidHttpClient", "AndroidHttpClient is deprecated as of API 22. Use HttpURLConnection or OkHttp instead.", []string{"android.net.http.AndroidHttpClient"}}}

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

// resourceTypeAlsoAccepted lists resource types a method takes besides its primary one in
// resourceMethodExpected: mipmaps and colors are valid drawables, and MenuInflater.inflate takes a menu.
func resourceTypeAlsoAccepted(method, actual string) bool {
	switch method {
	case "getDrawable", "setImageResource", "setImageDrawable":
		return actual == "mipmap" || actual == "color"
	case "inflate":
		return actual == "menu"
	}
	return false
}

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

// AccidentalOctalRule flags Gradle integer literals with a leading zero, which Groovy reads as octal.
// Lint only checks Gradle scripts: Java octal is intentional and Kotlin rejects leading zeros.
type AccidentalOctalRule struct {
	GradleBase
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

var knownLintIssueIDs = map[string]bool{"AaptCrash": true, "NewApi": true, "InlinedApi": true, "Override": true, "UnusedResources": true, "HardcodedText": true, "MissingTranslation": true, "ExtraTranslation": true, "MissingPermission": true, "ContentDescription": true, "ObsoleteLayoutParam": true, "ViewHolder": true, "LogConditional": true, "SdCardPath": true, "Wakelock": true, "SetJavaScriptEnabled": true, "ExportedService": true, "PackagedPrivateKey": true, "ValidFragment": true, "ViewConstructor": true, "WrongImport": true, "ServiceCast": true, "LayoutInflation": true, "ShowToast": true, "PackageManagerGetSignatures": true, "UseSparseArrays": true, "UseValueOf": true, "LongLogTag": true, "LogTagMismatch": true, "UnlocalizedSms": true, "ViewTag": true, "ShortAlarm": true, "UniqueConstants": true, "ShiftFlags": true, "AccidentalOctal": true, "AppCompatMethod": true, "CheckResult": true, "CommitPrefEdits": true, "CommitTransaction": true, "CustomViewStyleable": true, "CutPasteId": true, "DalvikOverride": true, "DefaultLocale": true, "Deprecated": true, "DeviceAdmin": true, "DuplicateActivity": true, "DuplicateIds": true, "DuplicateIncludedIds": true, "DuplicatePlatformClasses": true, "DuplicateUsesFeature": true, "ExtraText": true, "FullBackupContent": true, "GradleCompatible": true, "GradleDependency": true, "GradleDeprecated": true, "GradleDynamicVersion": true, "GradleGetter": true, "GradleIdeError": true, "GradleOverrides": true, "GradlePath": true, "IllegalResourceRef": true, "InconsistentArrays": true, "InconsistentLayout": true, "InnerclassSeparator": true, "InvalidResourceFolder": true, "ManifestOrder": true, "ManifestTypo": true, "MissingApplicationIcon": true, "MissingId": true, "MissingRegistered": true, "MissingVersion": true, "MockLocation": true, "MultipleUsesSdk": true, "NestedScrolling": true, "NotSibling": true, "UnknownId": true, "ObjectAnimatorBinding": true, "OnClick": true, "OverrideAbstract": true, "ParcelCreator": true, "PluralsCandidate": true, "PropertyEscape": true, "ProtectedPermissions": true, "Range": true, "ReferenceType": true, "Registered": true, "RequiredSize": true, "ResAuto": true, "ResourceAsColor": true, "ResourceType": true, "ScrollViewCount": true, "ScrollViewSize": true, "ServiceExported": true, "SetTextI18n": true, "SimpleDateFormat": true, "SpUsage": true, "StopShip": true, "StringFormatCount": true, "StringFormatInvalid": true, "StringFormatMatches": true, "SupportAnnotationUsage": true, "SwitchIntDef": true, "TextFields": true, "TextViewEdits": true, "UnpackedNativeCode": true, "UnusedAttribute": true, "UnusedNamespace": true, "UseCompoundDrawables": true, "UsesMinSdkAttributes": true, "WrongCall": true, "WrongCase": true, "WrongFolder": true, "WrongManifestParent": true, "WrongRegion": true, "WrongThread": true, "WrongViewCast": true, "Assert": true, "SQLiteString": true, "LocalSuppress": true, "AddJavascriptInterface": true, "GetInstance": true, "EasterEgg": true, "ExportedContentProvider": true, "ExportedReceiver": true, "ExportedPreferenceActivity": true, "GrantAllUris": true, "HardcodedDebugMode": true, "InsecureBaseConfiguration": true, "SecureRandom": true, "TrustedServer": true, "UnprotectedSMSBroadcastReceiver": true, "UnsafeProtectedBroadcastReceiver": true, "UseCheckPermission": true, "WorldReadableFiles": true, "WorldWriteableFiles": true, "DisableBaselineAlignment": true, "DrawAllocation": true, "FieldGetter": true, "FloatMath": true, "HandlerLeak": true, "InefficientWeight": true, "MergeRootFrame": true, "NestedWeights": true, "Overdraw": true, "Recycle": true, "TooDeepLayout": true, "UselessLeaf": true, "ClickableViewAccessibility": true, "LabelFor": true, "ByteOrderMark": true, "RelativeOverlap": true, "IconColors": true, "IconDensities": true, "IconDipSize": true, "IconDuplicates": true, "IconDuplicatesConfig": true, "IconExpectedSize": true, "IconExtension": true, "IconLauncherShape": true, "IconLocation": true, "IconMissingDensityFolder": true, "IconMixedNinePatch": true, "IconNoDpi": true, "IconXmlAndPng": true, "ConvertToWebp": true, "GifUsage": true, "AppCompatResource": true, "AppIndexingError": true, "AppIndexingWarning": true, "BackButton": true, "ButtonCase": true, "ButtonOrder": true, "ButtonStyle": true, "GoogleAppIndexingDeepLinkError": true, "GoogleAppIndexingWarning": true, "NegativeMargin": true, "RtlCompat": true, "RtlEnabled": true, "RtlHardcoded": true, "RtlSymmetry": true, "RtlSuperscript": true, "AllowBackup": true, "AlwaysShowAction": true, "InvalidUsesTagAttribute": true, "OldTargetApi": true, "ExpiredTargetSdkVersion": true, "PermissionImpliesUnsupportedHardware": true, "UnsupportedChromeOsHardware": true, "MissingLeanbackLauncher": true, "MissingLeanbackSupport": true, "Typos": true, "TypographyDashes": true, "TypographyEllipsis": true, "TypographyFractions": true, "TypographyOther": true, "TypographyQuotes": true, "UnusedIds": true, "Autofill": true, "RestrictedApi": true, "VisibleForTests": true, "MissingInflatedId": true, "NotificationPermission": true, "ObsoleteSdkInt": true, "NotifyDataSetChanged": true, "IntentFilterExportedReceiver": true, "AcceptsUserCertificates": true, "MissingTvBanner": true, "UnsupportedTvHardware": true, "ImpliedTouchscreenHardware": true, "all": true}

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

// classExtendsAndroidClass reports whether the class extends a framework class with this simple
// name. Android components are classes, so the supertype needs a constructor call, and an import
// of a same-named non-Android type or a local declaration rules the match out.
func classExtendsAndroidClass(file *scanner.File, idx uint32, simple string) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "class_declaration" {
		return false
	}
	matched := false
	for child := file.FlatFirstChild(idx); child != 0 && !matched; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		ctor, ok := file.FlatFindChild(child, "constructor_invocation")
		if !ok {
			continue
		}
		userType, _ := file.FlatFindChild(ctor, "user_type")
		if userType == 0 {
			continue
		}
		if ident := flatLastChildOfType(file, userType, "type_identifier"); ident != 0 && file.FlatNodeText(ident) == simple {
			// A qualified supertype must live in an Android package; my.pkg.Service is someone else's class.
			if qualified := strings.ReplaceAll(strings.TrimSpace(file.FlatNodeText(userType)), " ", ""); strings.Contains(qualified, ".") &&
				!strings.HasPrefix(qualified, "android.") && !strings.HasPrefix(qualified, "androidx.") &&
				!strings.HasPrefix(qualified, "com.google.android.") {
				continue
			}
			matched = true
		}
	}
	if !matched || fileDeclaresSimpleTypeNameLocal(file, simple) {
		return false
	}
	conflict := false
	file.FlatWalkNodes(0, "import_header", func(node uint32) {
		path := cleanImportHeaderTextWithAlias(file.FlatNodeText(node))
		if strings.Contains(path, " as ") || !strings.HasSuffix(path, "."+simple) {
			return
		}
		if !strings.HasPrefix(path, "android.") && !strings.HasPrefix(path, "androidx.") &&
			!strings.HasPrefix(path, "com.google.android.") {
			conflict = true
		}
	})
	return !conflict
}

// nearestKnownLintIssueID returns the known issue ID within two edits of id, or "" when none is close.
func nearestKnownLintIssueID(id string) string {
	if len(id) < 6 {
		return ""
	}
	best, bestDist := "", 3
	for known := range knownLintIssueIDs {
		if known == "all" {
			continue
		}
		if d := levenshtein(strings.ToLower(id), strings.ToLower(known)); d < bestDist || (d == bestDist && known < best) {
			best, bestDist = known, d
		}
	}
	return best
}
