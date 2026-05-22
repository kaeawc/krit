package rules

import (
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// ---------------------------------------------------------------------------
// Rule: DuplicateActivityManifest
// ---------------------------------------------------------------------------

// DuplicateActivityManifestRule checks for activities registered more than once.
// Duplicate registrations cause subtle bugs because attribute declarations from
// the two elements are not merged.
type DuplicateActivityManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *DuplicateActivityManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	seen := make(map[string]int) // name -> first line
	for _, act := range m.Application.Activities {
		if act.Name == "" {
			continue
		}
		if firstLine, ok := seen[act.Name]; ok {
			ctx.Emit(baseFinding(m.Path, act.Line, r.BaseRule,
				fmt.Sprintf("Activity `%s` is registered more than once (first at line %d). "+
					"Duplicate activity declarations cause subtle bugs.",
					act.Name, firstLine)))
		} else {
			seen[act.Name] = act.Line
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: WrongManifestParentManifest
// ---------------------------------------------------------------------------

// WrongManifestParentManifestRule checks for elements declared under the wrong
// parent. For example, <activity> must be under <application>, <uses-sdk> must
// be under <manifest>, <uses-library> must be under <application>.
type WrongManifestParentManifestRule struct {
	ManifestBase
	AndroidRule
}

// expectedParent maps element tags to their required parent tag.
var expectedParent = map[string]string{
	"activity":        "application",
	"activity-alias":  "application",
	"service":         "application",
	"receiver":        "application",
	"provider":        "application",
	"uses-library":    "application",
	"uses-sdk":        "manifest",
	"uses-permission": "manifest",
	"uses-feature":    "manifest",
	"permission":      "manifest",
	"application":     "manifest",
	"instrumentation": "manifest",
}

func (r *WrongManifestParentManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	for _, elem := range m.Elements {
		expected, ok := expectedParent[elem.Tag]
		if !ok {
			continue
		}
		if elem.ParentTag != expected {
			ctx.Emit(baseFinding(m.Path, elem.Line, r.BaseRule,
				fmt.Sprintf("<%s> should be a child of <%s>, not <%s>.",
					elem.Tag, expected, elem.ParentTag)))
		}
	}
	// Also check components inside application
	if m.Application != nil {
		for _, c := range allComponents(m.Application) {
			expected, ok := expectedParent[c.Tag]
			if !ok {
				continue
			}
			if c.ParentTag != expected {
				ctx.Emit(baseFinding(m.Path, c.Line, r.BaseRule,
					fmt.Sprintf("<%s> should be a child of <%s>, not <%s>.",
						c.Tag, expected, c.ParentTag)))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: GradleOverridesManifest
// ---------------------------------------------------------------------------

// GradleOverridesManifestRule checks for minSdkVersion/targetSdkVersion
// declared in the manifest. These values should be specified in build.gradle
// instead, as Gradle overrides manifest values.
type GradleOverridesManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *GradleOverridesManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.UsesSdk == nil {
		return
	}
	if m.MinSDK > 0 {
		ctx.Emit(baseFinding(m.Path, m.UsesSdk.Line, r.BaseRule,
			"This `minSdkVersion` value in the manifest is overridden by the value in "+
				"build.gradle. Remove the value from AndroidManifest.xml."))
	}
	if m.TargetSDK > 0 {
		ctx.Emit(baseFinding(m.Path, m.UsesSdk.Line, r.BaseRule,
			"This `targetSdkVersion` value in the manifest is overridden by the value in "+
				"build.gradle. Remove the value from AndroidManifest.xml."))
	}
}

// ---------------------------------------------------------------------------
// Rule: UsesSdkManifest
// ---------------------------------------------------------------------------

// UsesSdkManifestRule checks for a missing <uses-sdk> element. The manifest
// should contain a <uses-sdk> element defining the minimum and target API levels.
type UsesSdkManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *UsesSdkManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	// Skip variant manifests — they merge into main and don't need uses-sdk.
	if isNonMainManifestPath(m.Path) {
		return
	}
	// Skip library/test module stubs with no <application> element —
	// these modules get min/target SDK from their build.gradle, not the
	// manifest.
	if m.Application == nil {
		return
	}
	gi := lookupManifestGradleInfo(m.Path)
	if gi.found {
		// Skip when build.gradle sets minSdk directly — modern AGP
		// projects define the SDK levels in gradle and the manifest
		// does not need a redundant <uses-sdk> element.
		if gi.hasMinSdk {
			return
		}
		// Skip dedicated Android test modules (`com.android.test`) —
		// they never require <uses-sdk>.
		if gi.isTest {
			return
		}
		// Skip any module whose build.gradle marks it as an Android
		// application (including convention-plugin sample apps) —
		// SDK levels come from the convention plugin, parent project,
		// or catalog, not the manifest.
		if gi.isApplication || gi.isLibrary {
			return
		}
	}
	if m.UsesSdk == nil {
		ctx.Emit(baseFinding(m.Path, 1, r.BaseRule,
			"Manifest is missing a `<uses-sdk>` element. Add `<uses-sdk>` with "+
				"android:minSdkVersion and android:targetSdkVersion attributes."))
		return
	}
}

// ---------------------------------------------------------------------------
// Rule: MipmapLauncherRule
// ---------------------------------------------------------------------------

// MipmapLauncherRule checks that the launcher icon references @mipmap/ not @drawable/.
// Launcher icons should use mipmap resources for proper density handling.
type MipmapLauncherRule struct {
	ManifestBase
	AndroidRule
}

func (r *MipmapLauncherRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	icon := m.Application.Icon
	if icon == "" {
		return // MissingApplicationIcon handles this case
	}
	if strings.HasPrefix(icon, "@drawable/") {
		ctx.Emit(baseFinding(m.Path, m.Application.Line, r.BaseRule,
			fmt.Sprintf("Launcher icon `%s` uses @drawable/ instead of @mipmap/. "+
				"Use @mipmap/ resources for launcher icons to ensure proper density handling.",
				icon)))
		return
	}
}

// ---------------------------------------------------------------------------
// Rule: UniquePermissionRule
// ---------------------------------------------------------------------------

// UniquePermissionRule checks that custom permission names don't collide with
// well-known system permission names.
type UniquePermissionRule struct {
	ManifestBase
	AndroidRule
}

// systemPermissions is a set of well-known Android system permission names.
var systemPermissions = map[string]bool{
	"android.permission.ACCESS_COARSE_LOCATION":     true,
	"android.permission.ACCESS_FINE_LOCATION":       true,
	"android.permission.ACCESS_NETWORK_STATE":       true,
	"android.permission.ACCESS_WIFI_STATE":          true,
	"android.permission.BLUETOOTH":                  true,
	"android.permission.BLUETOOTH_ADMIN":            true,
	"android.permission.BLUETOOTH_CONNECT":          true,
	"android.permission.BLUETOOTH_SCAN":             true,
	"android.permission.CAMERA":                     true,
	"android.permission.CALL_PHONE":                 true,
	"android.permission.INTERNET":                   true,
	"android.permission.NFC":                        true,
	"android.permission.READ_CALENDAR":              true,
	"android.permission.READ_CONTACTS":              true,
	"android.permission.READ_EXTERNAL_STORAGE":      true,
	"android.permission.READ_MEDIA_AUDIO":           true,
	"android.permission.READ_MEDIA_IMAGES":          true,
	"android.permission.READ_MEDIA_VIDEO":           true,
	"android.permission.READ_PHONE_STATE":           true,
	"android.permission.READ_SMS":                   true,
	"android.permission.RECEIVE_BOOT_COMPLETED":     true,
	"android.permission.RECORD_AUDIO":               true,
	"android.permission.SEND_SMS":                   true,
	"android.permission.VIBRATE":                    true,
	"android.permission.WAKE_LOCK":                  true,
	"android.permission.WRITE_CALENDAR":             true,
	"android.permission.WRITE_CONTACTS":             true,
	"android.permission.WRITE_EXTERNAL_STORAGE":     true,
	"android.permission.FOREGROUND_SERVICE":         true,
	"android.permission.POST_NOTIFICATIONS":         true,
	"android.permission.BODY_SENSORS":               true,
	"android.permission.ACTIVITY_RECOGNITION":       true,
	"android.permission.ACCESS_BACKGROUND_LOCATION": true,
}

func (r *UniquePermissionRule) check(ctx *api.Context) {
	m := ctx.Manifest
	for _, perm := range m.Permissions {
		if systemPermissions[perm] {
			ctx.Emit(baseFinding(m.Path, 1, r.BaseRule,
				fmt.Sprintf("Custom permission `%s` collides with a system permission. "+
					"Use a unique name prefixed with your application package.",
					perm)))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: SystemPermissionRule
// ---------------------------------------------------------------------------

// SystemPermissionRule flags requests for dangerous system permissions that
// require runtime permission grants and careful justification.
type SystemPermissionRule struct {
	ManifestBase
	AndroidRule
}

// dangerousPermissions lists permissions in the "dangerous" protection level
// that require runtime user approval.
var dangerousPermissions = map[string]bool{
	"android.permission.CAMERA":                     true,
	"android.permission.RECORD_AUDIO":               true,
	"android.permission.ACCESS_FINE_LOCATION":       true,
	"android.permission.ACCESS_COARSE_LOCATION":     true,
	"android.permission.ACCESS_BACKGROUND_LOCATION": true,
	"android.permission.READ_CONTACTS":              true,
	"android.permission.WRITE_CONTACTS":             true,
	"android.permission.READ_CALENDAR":              true,
	"android.permission.WRITE_CALENDAR":             true,
	"android.permission.READ_EXTERNAL_STORAGE":      true,
	"android.permission.WRITE_EXTERNAL_STORAGE":     true,
	"android.permission.READ_PHONE_STATE":           true,
	"android.permission.CALL_PHONE":                 true,
	"android.permission.READ_SMS":                   true,
	"android.permission.SEND_SMS":                   true,
	"android.permission.BODY_SENSORS":               true,
	"android.permission.ACTIVITY_RECOGNITION":       true,
	"android.permission.READ_MEDIA_AUDIO":           true,
	"android.permission.READ_MEDIA_IMAGES":          true,
	"android.permission.READ_MEDIA_VIDEO":           true,
	"android.permission.POST_NOTIFICATIONS":         true,
	"android.permission.BLUETOOTH_CONNECT":          true,
	"android.permission.BLUETOOTH_SCAN":             true,
}

func (r *SystemPermissionRule) check(ctx *api.Context) {
	m := ctx.Manifest
	for _, perm := range m.UsesPermissions {
		if dangerousPermissions[perm] {
			// Extract the short name after the last dot
			short := perm
			if idx := strings.LastIndex(perm, "."); idx >= 0 {
				short = perm[idx+1:]
			}
			ctx.Emit(baseFinding(m.Path, 1, r.BaseRule,
				fmt.Sprintf("Requesting dangerous permission `%s`. "+
					"Ensure this permission is necessary and that runtime permission handling is implemented.",
					short)))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: ManifestTypoRule
// ---------------------------------------------------------------------------

// ManifestTypoRule detects common typos in manifest element names.
type ManifestTypoRule struct {
	ManifestBase
	AndroidRule
}

// manifestTypos maps common typos to their correct form.
var manifestTypos = map[string]string{
	"aplication":      "application",
	"applicaton":      "application",
	"applcation":      "application",
	"applicaiton":     "application",
	"activty":         "activity",
	"acitivity":       "activity",
	"activiti":        "activity",
	"reciver":         "receiver",
	"reciever":        "receiver",
	"receiever":       "receiver",
	"sevice":          "service",
	"servce":          "service",
	"serivce":         "service",
	"provder":         "provider",
	"proivder":        "provider",
	"providr":         "provider",
	"uses-permision":  "uses-permission",
	"uses-permssion":  "uses-permission",
	"user-permission": "uses-permission",
	"use-permission":  "uses-permission",
	"intent-fliter":   "intent-filter",
	"intent-fitler":   "intent-filter",
	"meta-dat":        "meta-data",
	"meatdata":        "meta-data",
	"uses-fetaure":    "uses-feature",
	"uses-featrue":    "uses-feature",
}

func (r *ManifestTypoRule) check(ctx *api.Context) {
	m := ctx.Manifest
	for _, elem := range m.Elements {
		if correct, ok := manifestTypos[elem.Tag]; ok {
			ctx.Emit(baseFinding(m.Path, elem.Line, r.BaseRule,
				fmt.Sprintf("Probable typo: `<%s>` should be `<%s>`.",
					elem.Tag, correct)))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: MissingApplicationIconRule
// ---------------------------------------------------------------------------

// MissingApplicationIconRule checks that the <application> element has android:icon set.
type MissingApplicationIconRule struct {
	ManifestBase
	AndroidRule
}

func (r *MissingApplicationIconRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	if isNonMainManifestPath(m.Path) {
		return
	}
	if isLibraryOrTestModuleManifest(m.Path) {
		return
	}
	if m.Application.Icon == "" {
		ctx.Emit(baseFinding(m.Path, m.Application.Line, r.BaseRule,
			"Missing `android:icon` attribute on <application>. "+
				"Add an icon to ensure the app has a visible launcher icon."))
		return
	}
}

// ---------------------------------------------------------------------------
// Rule: TargetNewerRule
// ---------------------------------------------------------------------------

// TargetNewerRule checks that targetSdkVersion is reasonably recent (>= 33).
type TargetNewerRule struct {
	ManifestBase
	AndroidRule
}

const minRecommendedTargetSDK = 33

func (r *TargetNewerRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.TargetSDK == 0 {
		return // Not set in manifest; Gradle likely controls this
	}
	if m.TargetSDK < minRecommendedTargetSDK {
		ctx.Emit(baseFinding(m.Path, 1, r.BaseRule,
			fmt.Sprintf("targetSdkVersion %d is outdated. "+
				"Target at least API %d for latest security and behavior changes.",
				m.TargetSDK, minRecommendedTargetSDK)))
		return
	}
}

// ---------------------------------------------------------------------------
// Rule: IntentFilterExportRequiredRule
// ---------------------------------------------------------------------------

// IntentFilterExportRequiredRule checks that components with intent-filters
// explicitly declare android:exported. Starting with API 31, this is required.
// This is stricter than MissingExportedFlag: it flags all component types.
type IntentFilterExportRequiredRule struct {
	ManifestBase
	AndroidRule
}

func (r *IntentFilterExportRequiredRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	// Only enforce for API 31+
	if m.TargetSDK > 0 && m.TargetSDK < 31 {
		return
	}
	components := allComponents(m.Application)
	for _, c := range components {
		if c.HasIntentFilter && c.Exported == nil {
			ctx.Emit(baseFinding(m.Path, c.Line, r.BaseRule,
				fmt.Sprintf("%s `%s` declares an intent-filter but is missing android:exported. "+
					"API 31+ requires android:exported on all components with intent-filters.",
					cases.Title(language.Und).String(c.Tag), c.Name)))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: DuplicateUsesFeatureManifestRule
// ---------------------------------------------------------------------------

// DuplicateUsesFeatureManifestRule detects <uses-feature> elements with
// duplicate android:name values.
type DuplicateUsesFeatureManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *DuplicateUsesFeatureManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if len(m.UsesFeatures) == 0 {
		return
	}
	seen := make(map[string]int) // name -> first line
	for _, f := range m.UsesFeatures {
		if f.Name == "" {
			continue
		}
		if firstLine, ok := seen[f.Name]; ok {
			ctx.Emit(baseFinding(m.Path, f.Line, r.BaseRule,
				fmt.Sprintf("Duplicate `<uses-feature android:name=\"%s\">` (first at line %d). "+
					"Remove the duplicate declaration.",
					f.Name, firstLine)))
		} else {
			seen[f.Name] = f.Line
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: MultipleUsesSdkManifestRule
// ---------------------------------------------------------------------------

// MultipleUsesSdkManifestRule detects more than one <uses-sdk> element in
// the manifest. Only one <uses-sdk> element is allowed.
type MultipleUsesSdkManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *MultipleUsesSdkManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	count := 0
	var secondLine int
	for _, elem := range m.Elements {
		if elem.Tag == "uses-sdk" {
			count++
			if count == 2 {
				secondLine = elem.Line
			}
		}
	}
	if count > 1 {
		ctx.Emit(baseFinding(m.Path, secondLine, r.BaseRule,
			fmt.Sprintf("Found %d `<uses-sdk>` elements. Only one is allowed per manifest.", count)))
		return
	}
}

// ---------------------------------------------------------------------------
// Rule: ManifestOrderManifestRule
// ---------------------------------------------------------------------------

// ManifestOrderManifestRule detects <application> appearing before
// <uses-permission> or <uses-sdk> in the manifest. Conventional ordering
// places <uses-permission> and <uses-sdk> before <application>.
type ManifestOrderManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *ManifestOrderManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	// Find the line of the <application> element
	appLine := 0
	for _, elem := range m.Elements {
		if elem.Tag == "application" {
			appLine = elem.Line
			break
		}
	}
	if appLine == 0 {
		return
	}

	for _, elem := range m.Elements {
		if (elem.Tag == "uses-permission" || elem.Tag == "uses-sdk") && elem.Line > appLine {
			ctx.Emit(baseFinding(m.Path, elem.Line, r.BaseRule,
				fmt.Sprintf("`<%s>` appears after `<application>` (line %d). "+
					"Move `<%s>` before `<application>` for conventional manifest ordering.",
					elem.Tag, appLine, elem.Tag)))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: MissingVersionManifestRule
// ---------------------------------------------------------------------------

// MissingVersionManifestRule detects missing android:versionCode or
// android:versionName on the <manifest> element.
type MissingVersionManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *MissingVersionManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	// Skip variant manifests — they merge into main and inherit version info
	// from build.gradle or the main manifest.
	if isNonMainManifestPath(m.Path) {
		return
	}
	// Skip library module manifests — Android library modules never set
	// versionCode/versionName (only app modules do). Heuristic: if the
	// manifest has no <application> element, no activities/services/providers,
	// it's a library stub.
	if m.Application == nil {
		return
	}
	if len(m.Application.Activities) == 0 &&
		len(m.Application.Services) == 0 &&
		len(m.Application.Receivers) == 0 &&
		len(m.Application.Providers) == 0 {
		// Minimal application element with no components — likely a library.
		return
	}
	// Modern AGP projects set versionCode/versionName in build.gradle's
	// defaultConfig, not the manifest. Skip if the build.gradle
	// provides them, or if the module is clearly an Android
	// application driven by a convention plugin.
	if gi := lookupManifestGradleInfo(m.Path); gi.found {
		if gi.hasVersionCode && gi.hasVersionName {
			return
		}
		// Library and test modules never need versionCode/versionName.
		if gi.isLibrary || gi.isTest {
			return
		}
		// Sample apps and convention-plugin driven modules typically
		// have versionCode/versionName supplied by the plugin itself,
		// not the module build.gradle. Treat any gradle file that
		// marks the module as an Android application as "versions
		// managed elsewhere".
		if gi.isApplication {
			return
		}
	}
	missingCode := m.VersionCode == ""
	missingName := m.VersionName == ""
	if !missingCode && !missingName {
		return
	}
	// Emit a single combined finding when both are missing. Emitting two
	// findings at the same (file, line, col) was producing duplicate
	// keys in downstream reporting.
	switch {
	case missingCode && missingName:
		ctx.Emit(baseFinding(m.Path, 1, r.BaseRule,
			"Missing `android:versionCode` and `android:versionName` on <manifest>. "+
				"Set a version code and name for release builds."))
	case missingCode:
		ctx.Emit(baseFinding(m.Path, 1, r.BaseRule,
			"Missing `android:versionCode` on <manifest>. "+
				"Set a version code for release builds."))
	default: // missingName
		ctx.Emit(baseFinding(m.Path, 1, r.BaseRule,
			"Missing `android:versionName` on <manifest>. "+
				"Set a version name for release builds."))
	}
}

// ---------------------------------------------------------------------------
// Rule: MockLocationManifestRule
// ---------------------------------------------------------------------------

// MockLocationManifestRule detects the ACCESS_MOCK_LOCATION permission in a
// non-debug manifest. This permission should only be present in debug builds.
type MockLocationManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *MockLocationManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.IsDebugManifest {
		return
	}
	for _, perm := range m.UsesPermissions {
		if perm == "android.permission.ACCESS_MOCK_LOCATION" {
			ctx.Emit(baseFinding(m.Path, 1, r.BaseRule,
				"`android.permission.ACCESS_MOCK_LOCATION` should only be declared in a debug-specific "+
					"manifest, not in the main AndroidManifest.xml."))
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: UnpackedNativeCodeManifestRule
// ---------------------------------------------------------------------------

// UnpackedNativeCodeManifestRule detects missing android:extractNativeLibs="false"
// when native libraries exist. On API 23+ setting this to false reduces APK size
// and install footprint.
type UnpackedNativeCodeManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *UnpackedNativeCodeManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if !m.HasNativeLibs {
		return
	}
	if m.Application == nil {
		return
	}
	app := m.Application
	if app.ExtractNativeLibs == nil || *app.ExtractNativeLibs {
		ctx.Emit(baseFinding(m.Path, app.Line, r.BaseRule,
			"Project uses native libraries but `android:extractNativeLibs` is not set to false. "+
				"Set `android:extractNativeLibs=\"false\"` on <application> to reduce APK size."))
		return
	}
}

// ---------------------------------------------------------------------------
// Rule: InvalidUsesTagAttributeManifestRule
// ---------------------------------------------------------------------------

// InvalidUsesTagAttributeManifestRule detects <uses-feature> elements where
// android:required is set to a value other than "true" or "false".
type InvalidUsesTagAttributeManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *InvalidUsesTagAttributeManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	for _, f := range m.UsesFeatures {
		if f.Required == "" {
			continue // not set is OK
		}
		if f.Required != "true" && f.Required != "false" {
			ctx.Emit(baseFinding(m.Path, f.Line, r.BaseRule,
				fmt.Sprintf("`<uses-feature android:name=\"%s\">` has android:required=\"%s\". "+
					"Value must be \"true\" or \"false\".",
					f.Name, f.Required)))
		}
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
