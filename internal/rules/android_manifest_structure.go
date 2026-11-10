package rules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
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
	app := manifestApplicationNode(manifestRoot(m))
	if app == nil {
		return
	}
	seen := make(map[string]bool)
	for _, act := range xmlChildrenNamed(app, "activity") {
		name, ok := xmlAttrOf(act, "android:name")
		if !ok || name.Value == "" {
			continue
		}
		// `.A`, `A` and `com.t.A` name the same class once the manifest package is applied.
		key := qualifyManifestClass(m.Package, name.Value)
		if seen[key] {
			ctx.Emit(baseFindingAt(m.Path, name.Line, name.Col, r.BaseRule,
				fmt.Sprintf("Duplicate registration for activity `%s`", key)))
		}
		seen[key] = true
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

// manifestLevelTags must be direct children of <manifest>, like Lint's WrongManifestParent.
var manifestLevelTags = map[string]bool{
	"uses-sdk": true, "uses-permission": true, "permission": true, "permission-group": true,
	"permission-tree": true, "uses-feature": true, "supports-screens": true,
	"compatible-screens": true, "supports-gl-texture": true, "uses-configuration": true,
	"application": true,
}

// applicationLevelTags must be direct children of <application>; <provider> may also sit in <queries>.
var applicationLevelTags = map[string]bool{
	"activity": true, "service": true, "receiver": true, "provider": true, "uses-library": true,
}

func (r *WrongManifestParentManifestRule) check(ctx *api.Context) {
	root := manifestRoot(ctx.Manifest)
	if root == nil {
		return
	}
	var walk func(node *android.XMLNode)
	walk = func(node *android.XMLNode) {
		for _, child := range node.Children {
			r.checkParent(ctx, node, child)
			walk(child)
		}
	}
	walk(root)
}

func (r *WrongManifestParentManifestRule) checkParent(ctx *api.Context, parent, node *android.XMLNode) {
	var msg string
	switch {
	case manifestLevelTags[node.Tag] && parent.Tag != "manifest":
		msg = fmt.Sprintf("The <%s> element must be a direct child of the <manifest> root element", node.Tag)
	case node.Tag == "provider" && parent.Tag != "application" && parent.Tag != "queries":
		msg = "The <provider> element must be a direct child of the <application> element or the <queries> element"
	case applicationLevelTags[node.Tag] && node.Tag != "provider" && parent.Tag != "application":
		msg = fmt.Sprintf("The <%s> element must be a direct child of the <application> element", node.Tag)
	default:
		return
	}
	ctx.Emit(baseFindingAt(ctx.Manifest.Path, node.Line, node.Col+1, r.BaseRule, msg))
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
	if isNonMainManifestPath(m.Path) {
		return
	}
	// A manifest value is only overridden when the owning module's build script sets it.
	gi := lookupManifestGradleInfo(m.Path)
	if !gi.found {
		return
	}
	root := manifestRoot(m)
	if root == nil {
		return
	}
	report := func(a android.XMLAttribute, gradleValue string) {
		if gradleValue == "" {
			gradleValue = "build.gradle"
		}
		ctx.Emit(baseFindingAt(m.Path, a.Line, a.Col, r.BaseRule,
			fmt.Sprintf("This `%s` value (`%s`) is not used; it is always overridden by the value specified in the Gradle build script (`%s`)",
				strings.TrimPrefix(a.Name, "android:"), a.Value, gradleValue)))
	}
	itoa := func(n int) string {
		if n <= 0 {
			return ""
		}
		return strconv.Itoa(n)
	}
	for _, sdk := range xmlChildrenNamed(root, "uses-sdk") {
		if a, ok := xmlAttrOf(sdk, "android:minSdkVersion"); ok && gi.hasMinSdk && xmlAttrValue(sdk, "tools:overrideLibrary") == "" {
			report(a, itoa(gi.minSdk))
		}
		if a, ok := xmlAttrOf(sdk, "android:targetSdkVersion"); ok && gi.hasTargetSdk {
			report(a, itoa(gi.targetSdk))
		}
	}
	if a, ok := xmlAttrOf(root, "android:versionCode"); ok && gi.hasVersionCode {
		report(a, gi.versionCode)
	}
	if a, ok := xmlAttrOf(root, "android:versionName"); ok && gi.hasVersionName {
		report(a, gi.versionName)
	}
}

// ---------------------------------------------------------------------------
// Rule: UsesSdkManifest
// ---------------------------------------------------------------------------

// UsesSdkManifestRule checks for a missing <uses-sdk> element. The manifest
// should contain a <uses-sdk> element defining the minimum and target API levels.
//
// Off by default: Android Lint 32 dropped its UsesMinSdkAttributes check (AGP
// moved minSdk and targetSdk to Gradle), so a manifest without <uses-sdk> is
// normal. The rule remains for projects that keep SDK levels in the manifest.
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
	root := manifestRoot(m)
	if root == nil {
		return
	}
	// Lint only reports when the module restricts densities with resConfigs.
	if !lookupManifestGradleInfo(m.Path).hasDensityResConfig {
		return
	}
	for _, app := range xmlChildrenNamed(root, "application") {
		r.checkIcon(ctx, app)
		for _, activity := range xmlChildrenNamed(app, "activity") {
			if isLaunchableActivityNode(activity) {
				r.checkIcon(ctx, activity)
			}
		}
	}
}

func (r *MipmapLauncherRule) checkIcon(ctx *api.Context, element *android.XMLNode) {
	icon, ok := xmlAttrOf(element, "android:icon")
	if !ok || !strings.HasPrefix(icon.Value, "@drawable/") {
		return
	}
	ctx.Emit(baseFindingAt(ctx.Manifest.Path, icon.Line, icon.Col, r.BaseRule,
		"Should use `@mipmap` instead of `@drawable` for launcher icons"))
}

// isLaunchableActivityNode reports whether an activity declares an intent filter with the LAUNCHER category.
func isLaunchableActivityNode(activity *android.XMLNode) bool {
	for _, filter := range xmlChildrenNamed(activity, "intent-filter") {
		for _, category := range xmlChildrenNamed(filter, "category") {
			if xmlAttrValue(category, "android:name") == "android.intent.category.LAUNCHER" {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Rule: UniquePermissionRule
// ---------------------------------------------------------------------------

// UniquePermissionRule reports <permission> and <permission-group> elements whose unqualified
// names collide, because aapt generates one Manifest.permission field per unqualified name.
type UniquePermissionRule struct {
	ManifestBase
	AndroidRule
}

func (r *UniquePermissionRule) check(ctx *api.Context) {
	m := ctx.Manifest
	root := manifestRoot(m)
	if root == nil {
		return
	}
	pkg := xmlAttrValue(root, "package")
	r.checkUniqueNames(ctx, root, pkg, "permission", "Permission")
	r.checkUniqueNames(ctx, root, pkg, "permission-group", "Permission group")
}

func (r *UniquePermissionRule) checkUniqueNames(ctx *api.Context, root *android.XMLNode, pkg, tag, label string) {
	seen := make(map[string]string)
	for _, element := range xmlChildrenNamed(root, tag) {
		if node := xmlAttrValue(element, "tools:node"); node == android.ToolsNodeRemove || node == android.ToolsNodeRemoveAll {
			continue
		}
		name, ok := xmlAttrOf(element, "android:name")
		if !ok {
			continue
		}
		qualified := strings.ReplaceAll(name.Value, "${applicationId}", pkg)
		if strings.Contains(qualified, "${") {
			continue
		}
		unqualified := qualified[strings.LastIndex(qualified, ".")+1:]
		previous, dup := seen[unqualified]
		if !dup {
			seen[unqualified] = qualified
			continue
		}
		if previous == qualified {
			continue
		}
		ctx.Emit(baseFindingAt(ctx.Manifest.Path, name.Line, name.Col, r.BaseRule,
			fmt.Sprintf("%s name `%s` is not unique (appears in both `%s` and `%s`)", label, unqualified, previous, qualified)))
	}
}

// ---------------------------------------------------------------------------
// Rule: SystemPermissionRule
// ---------------------------------------------------------------------------

// SystemPermissionRule flags uses-permission requests for permissions that
// only system apps can hold, which an ordinary app is never granted.
type SystemPermissionRule struct {
	ManifestBase
	AndroidRule
}

// systemOnlyPermissions lists permissions granted only to apps signed with the
// platform key or installed in the system image (AOSP SystemPermissionsDetector).
var systemOnlyPermissions = map[string]bool{
	"android.permission.ACCESS_CHECKIN_PROPERTIES":      true,
	"android.permission.ACCESS_SURFACE_FLINGER":         true,
	"android.permission.BACKUP":                         true,
	"android.permission.BIND_APPWIDGET":                 true,
	"android.permission.BIND_DEVICE_ADMIN":              true,
	"android.permission.BIND_INPUT_METHOD":              true,
	"android.permission.BIND_REMOTEVIEWS":               true,
	"android.permission.BIND_TEXT_SERVICE":              true,
	"android.permission.BIND_VPN_SERVICE":               true,
	"android.permission.BIND_WALLPAPER":                 true,
	"android.permission.BRICK":                          true,
	"android.permission.BROADCAST_PACKAGE_REMOVED":      true,
	"android.permission.BROADCAST_SMS":                  true,
	"android.permission.BROADCAST_WAP_PUSH":             true,
	"android.permission.CALL_PRIVILEGED":                true,
	"android.permission.CHANGE_COMPONENT_ENABLED_STATE": true,
	"android.permission.CLEAR_APP_USER_DATA":            true,
	"android.permission.CONFIRM_FULL_BACKUP":            true,
	"android.permission.CONTROL_LOCATION_UPDATES":       true,
	"android.permission.DELETE_CACHE_FILES":             true,
	"android.permission.DELETE_PACKAGES":                true,
	"android.permission.DEVICE_POWER":                   true,
	"android.permission.DIAGNOSTIC":                     true,
	"android.permission.DUMP":                           true,
	"android.permission.FACTORY_TEST":                   true,
	"android.permission.FORCE_BACK":                     true,
	"android.permission.GLOBAL_SEARCH":                  true,
	"android.permission.HARDWARE_TEST":                  true,
	"android.permission.INJECT_EVENTS":                  true,
	"android.permission.INSTALL_LOCATION_PROVIDER":      true,
	"android.permission.INSTALL_PACKAGES":               true,
	"android.permission.INTERNAL_SYSTEM_WINDOW":         true,
	"android.permission.MANAGE_APP_TOKENS":              true,
	"android.permission.MASTER_CLEAR":                   true,
	"android.permission.MODIFY_PHONE_STATE":             true,
	"android.permission.MOUNT_FORMAT_FILESYSTEMS":       true,
	"android.permission.MOUNT_UNMOUNT_FILESYSTEMS":      true,
	"android.permission.READ_FRAME_BUFFER":              true,
	"android.permission.READ_INPUT_STATE":               true,
	"android.permission.READ_LOGS":                      true,
	"android.permission.REBOOT":                         true,
	"android.permission.SET_ACTIVITY_WATCHER":           true,
	"android.permission.SET_ORIENTATION":                true,
	"android.permission.SET_POINTER_SPEED":              true,
	"android.permission.SET_PROCESS_LIMIT":              true,
	"android.permission.SET_TIME":                       true,
	"android.permission.SHUTDOWN":                       true,
	"android.permission.STATUS_BAR":                     true,
	"android.permission.UPDATE_DEVICE_STATS":            true,
	"android.permission.WRITE_APN_SETTINGS":             true,
	"android.permission.WRITE_GSERVICES":                true,
	"android.permission.WRITE_SECURE_SETTINGS":          true,
}

func (r *SystemPermissionRule) check(ctx *api.Context) {
	m := ctx.Manifest
	for _, perm := range m.UsesPermissions {
		if systemOnlyPermissions[perm] {
			ctx.Emit(baseFinding(m.Path, permissionLine(m.UsesPermissionLines, perm), r.BaseRule,
				fmt.Sprintf("Permission `%s` is only granted to system apps.", perm)))
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
				fmt.Sprintf("Misspelled tag `<%s>`: Did you mean `%s`?",
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

// IntentFilterExportRequiredRule is Lint's IntentFilterExportedReceiver: from targetSdk 31 an activity,
// activity-alias, service or receiver with an intent filter must set android:exported.
type IntentFilterExportRequiredRule struct {
	ManifestBase
	AndroidRule
}

func (r *IntentFilterExportRequiredRule) check(ctx *api.Context) { checkExportedFlag(ctx, r.BaseRule) }

const (
	exportedFlagMessage  = "As of Android 12, `android:exported` must be set; use `true` to make the activity available to other apps, and `false` otherwise."
	exportedLauncherNote = " For launcher activities, this should be set to true."
	exportedLaunchableFn = "A launchable activity must be exported as of Android 12, which also makes it available to other apps."
)

// checkExportedFlag mirrors ExportedFlagDetector, which both exported-flag rules share. A missing flag is
// an error from targetSdk 31 and a warning below it.
func checkExportedFlag(ctx *api.Context, rule BaseRule) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	missingSeverity := "warning"
	if manifestTargetSDK(m) >= 31 {
		missingSeverity = "error"
	}
	var components []ManifestComponent
	components = append(components, m.Application.Activities...)
	components = append(components, m.Application.Services...)
	components = append(components, m.Application.Receivers...)
	for _, c := range components {
		exportedSet := c.Exported != nil || c.ExportedRaw != ""
		switch {
		case !exportedSet && (c.HasIntentFilter || c.HasNavGraph):
			msg := exportedFlagMessage
			if c.FirstFilterAction == "android.intent.action.MAIN" {
				msg += exportedLauncherNote
			}
			f := baseFinding(m.Path, c.Line, rule, msg)
			f.Severity = missingSeverity
			ctx.Emit(f)
		case c.ExportedRaw == "false" && c.FirstFilterAction == "android.intent.action.MAIN" &&
			c.FirstFilterCategory == "android.intent.category.LAUNCHER":
			ctx.Emit(baseFinding(m.Path, c.Line, rule, exportedLaunchableFn))
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
	root := manifestRoot(m)
	if root == nil {
		return
	}
	seen := make(map[string]bool)
	for _, f := range xmlChildrenNamed(root, "uses-feature") {
		name, ok := xmlAttrOf(f, "android:name")
		if !ok || name.Value == "" {
			continue
		}
		if seen[name.Value] {
			ctx.Emit(baseFindingAt(m.Path, name.Line, name.Col, r.BaseRule,
				fmt.Sprintf("Duplicate declaration of uses-feature `%s`", name.Value)))
		}
		seen[name.Value] = true
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
			"There should only be a single `<uses-sdk>` element in the manifest: merge these together"))
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

// manifestOrderTags are the root children Lint expects before <application>.
// Lint reports only the first misplaced one per manifest, even when it is suppressed.
var manifestOrderTags = map[string]bool{
	"uses-permission": true, "permission": true, "permission-tree": true, "permission-group": true,
	"uses-sdk": true, "uses-configuration": true, "uses-feature": true, "supports-screens": true,
	"compatible-screens": true, "supports-gl-texture": true,
}

func (r *ManifestOrderManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	seenApplication := false
	for _, elem := range m.Elements {
		switch {
		case elem.Tag == "application":
			seenApplication = true
		case seenApplication && manifestOrderTags[elem.Tag] && elem.ParentTag == "manifest":
			ctx.Emit(baseFinding(m.Path, elem.Line, r.BaseRule,
				fmt.Sprintf("`<%s>` tag appears after `<application>` tag", elem.Tag)))
			return
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
	// Skip variant manifests: they merge into main and inherit version info from build.gradle or the main manifest.
	if isNonMainManifestPath(m.Path) {
		return
	}
	// Lint reports this only for non-Gradle projects, where the manifest is the only place for the version.
	if gi := lookupManifestGradleInfo(m.Path); gi.found {
		return
	}
	// Library manifests never carry a version. Without Gradle info, a manifest with no components is taken as one.
	if m.Application == nil ||
		(len(m.Application.Activities) == 0 && len(m.Application.Services) == 0 &&
			len(m.Application.Receivers) == 0 && len(m.Application.Providers) == 0) {
		return
	}
	line := 1
	for _, elem := range m.Elements {
		if elem.Tag == "manifest" {
			line = elem.Line
			break
		}
	}
	if m.VersionCode == "" {
		ctx.Emit(baseFinding(m.Path, line, r.BaseRule,
			"Should set `android:versionCode` to specify the application version"))
	}
	if m.VersionName == "" {
		ctx.Emit(baseFinding(m.Path, line, r.BaseRule,
			"Should set `android:versionName` to specify the application version"))
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
	if m.IsDebugManifest || isDebugManifestPath(m.Path) {
		return
	}
	for _, perm := range m.UsesPermissions {
		if perm == "android.permission.ACCESS_MOCK_LOCATION" {
			ctx.Emit(baseFinding(m.Path, permissionLine(m.UsesPermissionLines, perm), r.BaseRule,
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
// Rule: InvalidUsesFeatureRequiredManifestRule
// ---------------------------------------------------------------------------

// InvalidUsesFeatureRequiredManifestRule detects <uses-feature> elements where
// android:required is set to a value other than "true" or "false". This is a
// krit-only check. It was named InvalidUsesTagAttributeManifest, which is the
// id Android Lint uses for the automotive <uses name> check (see
// InvalidUsesTagAttributeRule); the old id stays a suppression alias.
type InvalidUsesFeatureRequiredManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *InvalidUsesFeatureRequiredManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	for _, f := range m.UsesFeatures {
		if f.Required == "" {
			continue // not set is OK
		}
		// A resource reference or manifest placeholder resolves at build time.
		if strings.HasPrefix(f.Required, "@") || strings.HasPrefix(f.Required, "?") || strings.Contains(f.Required, "${") {
			continue
		}
		if !strings.EqualFold(f.Required, "true") && !strings.EqualFold(f.Required, "false") {
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

// permissionLine returns the recorded element line for a permission, or 1 when unknown.
func permissionLine(lines map[string]int, name string) int {
	if line := lines[name]; line > 0 {
		return line
	}
	return 1
}
