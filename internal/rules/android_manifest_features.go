package rules

import (
	"fmt"
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

// ---------------------------------------------------------------------------
// Rule: RtlEnabledManifestRule
// ---------------------------------------------------------------------------

// RtlEnabledManifestRule detects missing android:supportsRtl="true" on the
// <application> element. Supporting RTL layouts is recommended for
// internationalization.
type RtlEnabledManifestRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android manifest features rule. Detection flags required/optional
// feature declarations and version constraints via attribute presence
// checks. Classified per roadmap/17.
func (r *RtlEnabledManifestRule) Confidence() float64 { return 0.75 }

func (r *RtlEnabledManifestRule) check(ctx *v2.Context) {
	m, _ := ctx.Manifest.(*Manifest)
	if m.Application == nil {
		return
	}
	if isNonMainManifestPath(m.Path) {
		return
	}
	if isLibraryOrTestModuleManifest(m.Path) {
		return
	}
	app := m.Application
	if app.SupportsRtl == nil || !*app.SupportsRtl {
		ctx.Emit(manifestFinding(m.Path, app.Line, r.BaseRule,
			"Missing `android:supportsRtl=\"true\"` on <application>. "+
				"Add RTL support to ensure proper layout mirroring for RTL languages."))
		return
	}
	return
}

// ---------------------------------------------------------------------------
// Rule: RtlCompatManifestRule
// ---------------------------------------------------------------------------

// RtlCompatManifestRule detects targetSdkVersion >= 17 without
// supportsRtl="true" on the application element. API 17 introduced native
// RTL layout support; apps targeting 17+ should enable it.
type RtlCompatManifestRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android manifest features rule. Detection flags required/optional
// feature declarations and version constraints via attribute presence
// checks. Classified per roadmap/17.
func (r *RtlCompatManifestRule) Confidence() float64 { return 0.75 }

func (r *RtlCompatManifestRule) check(ctx *v2.Context) {
	m, _ := ctx.Manifest.(*Manifest)
	if m.Application == nil {
		return
	}
	if m.TargetSDK > 0 && m.TargetSDK < 17 {
		return
	}
	// If TargetSDK is 0 (not set), we don't flag — can't confirm API level
	if m.TargetSDK == 0 {
		return
	}
	if m.Application.SupportsRtl == nil || !*m.Application.SupportsRtl {
		ctx.Emit(manifestFinding(m.Path, m.Application.Line, r.BaseRule,
			fmt.Sprintf("targetSdkVersion is %d (>= 17) but `android:supportsRtl` is not set to true. "+
				"Enable RTL support with `android:supportsRtl=\"true\"` for proper right-to-left layout mirroring.",
				m.TargetSDK)))
		return
	}
	return
}

// ---------------------------------------------------------------------------
// Rule: AppIndexingErrorManifestRule
// ---------------------------------------------------------------------------

// AppIndexingErrorManifestRule detects activities with a VIEW intent filter
// that are missing HTTP or HTTPS data schemes. Activities handling VIEW actions
// should typically support deep linking via http/https.
type AppIndexingErrorManifestRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android manifest features rule. Detection flags required/optional
// feature declarations and version constraints via attribute presence
// checks. Classified per roadmap/17.
func (r *AppIndexingErrorManifestRule) Confidence() float64 { return 0.75 }

func (r *AppIndexingErrorManifestRule) check(ctx *v2.Context) {
	m, _ := ctx.Manifest.(*Manifest)
	if m.Application == nil {
		return
	}
	if isNonMainManifestPath(m.Path) {
		return
	}
	if isLibraryOrTestModuleManifest(m.Path) {
		return
	}
	for _, act := range m.Application.Activities {
		hasViewAction := false
		for _, action := range act.IntentFilterActions {
			if action == "android.intent.action.VIEW" {
				hasViewAction = true
				break
			}
		}
		if !hasViewAction {
			continue
		}
		// Only flag when the activity has EXACTLY ONE of http/https but not
		// both — that's a partial configuration that misses half of web deep
		// links. Activities using only custom schemes (tsdevice, sms, tel,
		// etc.) are legitimate and not a concern here.
		hasHTTP := false
		hasHTTPS := false
		for _, scheme := range act.IntentFilterDataSchemes {
			if scheme == "http" {
				hasHTTP = true
			} else if scheme == "https" {
				hasHTTPS = true
			}
		}
		if (hasHTTP && !hasHTTPS) || (hasHTTPS && !hasHTTP) {
			missing := "https"
			if hasHTTPS {
				missing = "http"
			}
			ctx.Emit(manifestFinding(m.Path, act.Line, r.BaseRule,
				fmt.Sprintf("Activity `%s` has a web deep link but is missing the `%s` scheme. "+
					"Add `<data android:scheme=\"%s\" />` to handle both http and https URLs.",
					act.Name, missing, missing)))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: AppIndexingWarningManifestRule
// ---------------------------------------------------------------------------

// AppIndexingWarningManifestRule detects activities with browsable intent
// filters that are missing a VIEW action. Activities with the BROWSABLE
// category should typically also declare a VIEW action for proper deep link
// handling.
type AppIndexingWarningManifestRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android manifest features rule. Detection flags required/optional
// feature declarations and version constraints via attribute presence
// checks. Classified per roadmap/17.
func (r *AppIndexingWarningManifestRule) Confidence() float64 { return 0.75 }

func (r *AppIndexingWarningManifestRule) check(ctx *v2.Context) {
	m, _ := ctx.Manifest.(*Manifest)
	if m.Application == nil {
		return
	}
	for _, act := range m.Application.Activities {
		hasBrowsable := false
		for _, cat := range act.IntentFilterCategories {
			if cat == "android.intent.category.BROWSABLE" {
				hasBrowsable = true
				break
			}
		}
		if !hasBrowsable {
			continue
		}
		hasViewAction := false
		for _, action := range act.IntentFilterActions {
			if action == "android.intent.action.VIEW" {
				hasViewAction = true
				break
			}
		}
		if !hasViewAction {
			ctx.Emit(manifestFinding(m.Path, act.Line, r.BaseRule,
				fmt.Sprintf("Activity `%s` has a BROWSABLE intent filter but no VIEW action. "+
					"Add `<action android:name=\"android.intent.action.VIEW\" />` to handle deep links properly.",
					act.Name)))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: GoogleAppIndexingDeepLinkErrorManifestRule
// ---------------------------------------------------------------------------

// GoogleAppIndexingDeepLinkErrorManifestRule detects deep link data elements
// with malformed URIs — specifically, data elements that declare a scheme but
// no host. A URI with a scheme but no host is malformed for deep linking.
type GoogleAppIndexingDeepLinkErrorManifestRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android manifest features rule. Detection flags required/optional
// feature declarations and version constraints via attribute presence
// checks. Classified per roadmap/17.
func (r *GoogleAppIndexingDeepLinkErrorManifestRule) Confidence() float64 { return 0.75 }

func (r *GoogleAppIndexingDeepLinkErrorManifestRule) check(ctx *v2.Context) {
	m, _ := ctx.Manifest.(*Manifest)
	if m.Application == nil {
		return
	}
	if isNonMainManifestPath(m.Path) {
		return
	}
	for _, act := range m.Application.Activities {
		hasViewAction := false
		for _, action := range act.IntentFilterActions {
			if action == "android.intent.action.VIEW" {
				hasViewAction = true
				break
			}
		}
		if !hasViewAction {
			continue
		}
		// Custom schemes (not http/https) legitimately don't require a host.
		// Only flag when http or https is used without a host — those are
		// real web URLs that need a host to be well-formed.
		hasWebScheme := false
		for _, scheme := range act.IntentFilterDataSchemes {
			if scheme == "http" || scheme == "https" {
				hasWebScheme = true
				break
			}
		}
		if hasWebScheme && len(act.IntentFilterDataHosts) == 0 {
			ctx.Emit(manifestFinding(m.Path, act.Line, r.BaseRule,
				fmt.Sprintf("Activity `%s` has an http/https deep link with no host. "+
					"A web URI with a scheme but no host is malformed. "+
					"Add `<data android:host=\"...\" />` to the intent filter.",
					act.Name)))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: GoogleAppIndexingWarningManifestRule
// ---------------------------------------------------------------------------

// GoogleAppIndexingWarningManifestRule detects apps that lack any deep link
// support. If no activity in the manifest declares a VIEW intent filter with
// an http or https data scheme, the app cannot be indexed by Google Search.
type GoogleAppIndexingWarningManifestRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android manifest features rule. Detection flags required/optional
// feature declarations and version constraints via attribute presence
// checks. Classified per roadmap/17.
func (r *GoogleAppIndexingWarningManifestRule) Confidence() float64 { return 0.75 }

func (r *GoogleAppIndexingWarningManifestRule) check(ctx *v2.Context) {
	m, _ := ctx.Manifest.(*Manifest)
	if m.Application == nil {
		return
	}
	// Skip variant manifests and test fixtures — app indexing is only relevant
	// to production manifests.
	if isTestManifestPath(m.Path) || isLibraryOrTestModuleManifest(m.Path) {
		return
	}
	for _, act := range m.Application.Activities {
		hasViewAction := false
		for _, action := range act.IntentFilterActions {
			if action == "android.intent.action.VIEW" {
				hasViewAction = true
				break
			}
		}
		if !hasViewAction {
			continue
		}
		for _, scheme := range act.IntentFilterDataSchemes {
			if scheme == "http" || scheme == "https" {
				return // At least one activity supports deep links
			}
		}
	}
	ctx.Emit(manifestFinding(m.Path, 1, r.BaseRule,
		"No activity with a VIEW intent filter and http/https data scheme found. "+
			"Add deep link support to enable Google App Indexing. "+
			"https://developer.android.com/training/app-indexing"))
	return
}

// ---------------------------------------------------------------------------
// Rule: MissingLeanbackLauncherManifestRule
// ---------------------------------------------------------------------------

// MissingLeanbackLauncherManifestRule detects apps that declare the
// android.software.leanback feature but have no activity with a
// LEANBACK_LAUNCHER category in its intent filter. Without a leanback launcher
// activity, the app won't appear on Android TV.
type MissingLeanbackLauncherManifestRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android manifest features rule. Detection flags required/optional
// feature declarations and version constraints via attribute presence
// checks. Classified per roadmap/17.
func (r *MissingLeanbackLauncherManifestRule) Confidence() float64 { return 0.75 }

func (r *MissingLeanbackLauncherManifestRule) check(ctx *v2.Context) {
	m, _ := ctx.Manifest.(*Manifest)
	hasLeanback := false
	for _, f := range m.UsesFeatures {
		if f.Name == "android.software.leanback" {
			hasLeanback = true
			break
		}
	}
	if !hasLeanback {
		return
	}
	if m.Application == nil {
		ctx.Emit(manifestFinding(m.Path, 1, r.BaseRule,
			"App declares `android.software.leanback` feature but has no activity with "+
				"LEANBACK_LAUNCHER category. Add an activity with "+
				"`<category android:name=\"android.intent.category.LEANBACK_LAUNCHER\" />`."))
		return
	}
	for _, act := range m.Application.Activities {
		for _, cat := range act.IntentFilterCategories {
			if cat == "android.intent.category.LEANBACK_LAUNCHER" {
				return
			}
		}
	}
	ctx.Emit(manifestFinding(m.Path, 1, r.BaseRule,
		"App declares `android.software.leanback` feature but has no activity with "+
			"LEANBACK_LAUNCHER category. Add an activity with "+
			"`<category android:name=\"android.intent.category.LEANBACK_LAUNCHER\" />`."))
	return
}

// ---------------------------------------------------------------------------
// Rule: MissingLeanbackSupportManifestRule
// ---------------------------------------------------------------------------

// MissingLeanbackSupportManifestRule detects apps that declare the
// android.software.leanback feature but do not opt out of the touchscreen
// requirement. TV apps must declare
// <uses-feature android:name="android.hardware.touchscreen" android:required="false" />
// because TV devices do not have touchscreens.
type MissingLeanbackSupportManifestRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android manifest features rule. Detection flags required/optional
// feature declarations and version constraints via attribute presence
// checks. Classified per roadmap/17.
func (r *MissingLeanbackSupportManifestRule) Confidence() float64 { return 0.75 }

func (r *MissingLeanbackSupportManifestRule) check(ctx *v2.Context) {
	m, _ := ctx.Manifest.(*Manifest)
	hasLeanback := false
	for _, f := range m.UsesFeatures {
		if f.Name == "android.software.leanback" {
			hasLeanback = true
			break
		}
	}
	if !hasLeanback {
		return
	}
	for _, f := range m.UsesFeatures {
		if f.Name == "android.hardware.touchscreen" && strings.EqualFold(f.Required, "false") {
			return
		}
	}
	ctx.Emit(manifestFinding(m.Path, 1, r.BaseRule,
		"App declares `android.software.leanback` feature but does not opt out of touchscreen. "+
			"TV apps must declare `<uses-feature android:name=\"android.hardware.touchscreen\" "+
			"android:required=\"false\" />` because TV devices do not have touchscreens."))
	return
}

// ---------------------------------------------------------------------------
// Rule: PermissionImpliesUnsupportedHardwareManifestRule
// ---------------------------------------------------------------------------

// PermissionImpliesUnsupportedHardwareManifestRule detects permissions that
// imply hardware features (e.g., CAMERA implies android.hardware.camera) when
// those features are not declared with android:required="false". This can
// prevent the app from being installed on devices without that hardware.
type PermissionImpliesUnsupportedHardwareManifestRule struct {
	ManifestBase
	AndroidRule
}

// permissionToFeature maps permissions to the hardware features they imply.
var permissionToFeature = map[string]string{
	"android.permission.CAMERA":                 "android.hardware.camera",
	"android.permission.BLUETOOTH":              "android.hardware.bluetooth",
	"android.permission.BLUETOOTH_ADMIN":        "android.hardware.bluetooth",
	"android.permission.BLUETOOTH_CONNECT":      "android.hardware.bluetooth",
	"android.permission.BLUETOOTH_SCAN":         "android.hardware.bluetooth",
	"android.permission.ACCESS_FINE_LOCATION":   "android.hardware.location.gps",
	"android.permission.ACCESS_COARSE_LOCATION": "android.hardware.location",
	"android.permission.CALL_PHONE":             "android.hardware.telephony",
	"android.permission.READ_PHONE_STATE":       "android.hardware.telephony",
	"android.permission.READ_SMS":               "android.hardware.telephony",
	"android.permission.SEND_SMS":               "android.hardware.telephony",
	"android.permission.RECORD_AUDIO":           "android.hardware.microphone",
}

// Confidence reports a tier-2 (medium) base confidence. Android manifest features rule. Detection flags required/optional
// feature declarations and version constraints via attribute presence
// checks. Classified per roadmap/17.
func (r *PermissionImpliesUnsupportedHardwareManifestRule) Confidence() float64 { return 0.75 }

func (r *PermissionImpliesUnsupportedHardwareManifestRule) check(ctx *v2.Context) {
	m, _ := ctx.Manifest.(*Manifest)
	// Build a set of features declared with required="false"
	optionalFeatures := make(map[string]bool)
	for _, f := range m.UsesFeatures {
		if strings.EqualFold(f.Required, "false") {
			optionalFeatures[f.Name] = true
		}
	}

	// Collect all missing permission/feature pairs first. Emitting one
	// finding per pair at the same (file, line, col) collided on the
	// finding key downstream — combine them into a single finding
	// listing every offender.
	type permFeature struct{ perm, feature string }
	var missing []permFeature
	seen := make(map[string]bool)
	for _, perm := range m.UsesPermissions {
		feature, ok := permissionToFeature[perm]
		if !ok {
			continue
		}
		if seen[feature] {
			continue
		}
		seen[feature] = true
		if !optionalFeatures[feature] {
			missing = append(missing, permFeature{perm: perm, feature: feature})
		}
	}
	if len(missing) == 0 {
		return
	}
	if len(missing) == 1 {
		only := missing[0]
		ctx.Emit(manifestFinding(m.Path, 1, r.BaseRule,
			fmt.Sprintf("Permission `%s` implies hardware feature `%s`. "+
				"Declare `<uses-feature android:name=\"%s\" android:required=\"false\" />` "+
				"to allow installation on devices without this hardware.",
				only.perm, only.feature, only.feature)))
		return
	}
	var parts []string
	for _, mf := range missing {
		parts = append(parts, fmt.Sprintf("`%s` -> `%s`", mf.perm, mf.feature))
	}
	ctx.Emit(manifestFinding(m.Path, 1, r.BaseRule,
		fmt.Sprintf("Permissions imply hardware features that are not declared as optional: %s. "+
			"Declare each with `<uses-feature android:name=\"…\" android:required=\"false\" />` "+
			"to allow installation on devices without this hardware.",
			strings.Join(parts, ", "))))
	return
}

// ---------------------------------------------------------------------------
// Rule: UnsupportedChromeOsHardwareManifestRule
// ---------------------------------------------------------------------------

// UnsupportedChromeOsHardwareManifestRule detects <uses-feature> declarations
// for hardware features unsupported on Chrome OS (e.g., telephony, camera)
// that are not marked with android:required="false". This prevents the app
// from being available on Chromebooks.
type UnsupportedChromeOsHardwareManifestRule struct {
	ManifestBase
	AndroidRule
}

// chromeOsUnsupportedFeatures lists features unavailable on most Chrome OS devices.
var chromeOsUnsupportedFeatures = map[string]bool{
	"android.hardware.telephony":            true,
	"android.hardware.camera":               true,
	"android.hardware.camera.autofocus":     true,
	"android.hardware.camera.flash":         true,
	"android.hardware.nfc":                  true,
	"android.hardware.sensor.accelerometer": true,
	"android.hardware.sensor.barometer":     true,
	"android.hardware.sensor.compass":       true,
	"android.hardware.sensor.gyroscope":     true,
}

// Confidence reports a tier-2 (medium) base confidence. Android manifest features rule. Detection flags required/optional
// feature declarations and version constraints via attribute presence
// checks. Classified per roadmap/17.
func (r *UnsupportedChromeOsHardwareManifestRule) Confidence() float64 { return 0.75 }

func (r *UnsupportedChromeOsHardwareManifestRule) check(ctx *v2.Context) {
	m, _ := ctx.Manifest.(*Manifest)
	for _, f := range m.UsesFeatures {
		if !chromeOsUnsupportedFeatures[f.Name] {
			continue
		}
		if strings.EqualFold(f.Required, "false") {
			continue
		}
		ctx.Emit(manifestFinding(m.Path, f.Line, r.BaseRule,
			fmt.Sprintf("`<uses-feature android:name=\"%s\">` is not available on most Chrome OS devices. "+
				"Set `android:required=\"false\"` to allow installation on Chromebooks.",
				f.Name)))
	}
}

// ---------------------------------------------------------------------------
// Rule: DeviceAdminManifestRule
// ---------------------------------------------------------------------------

// DeviceAdminManifestRule detects <receiver> elements with the
// android.app.admin.DEVICE_ADMIN action but missing the required
// <meta-data android:resource="@xml/device_admin"/> element.
type DeviceAdminManifestRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android manifest features rule. Detection flags required/optional
// feature declarations and version constraints via attribute presence
// checks. Classified per roadmap/17.
func (r *DeviceAdminManifestRule) Confidence() float64 { return 0.75 }

func (r *DeviceAdminManifestRule) check(ctx *v2.Context) {
	m, _ := ctx.Manifest.(*Manifest)
	if m.Application == nil {
		return
	}
	for _, recv := range m.Application.Receivers {
		hasDeviceAdminAction := false
		for _, action := range recv.IntentFilterActions {
			if action == "android.app.action.DEVICE_ADMIN_ENABLED" {
				hasDeviceAdminAction = true
				break
			}
		}
		if !hasDeviceAdminAction {
			continue
		}
		// Check for required <meta-data android:resource="@xml/device_admin"/>
		hasDeviceAdminMeta := false
		for _, md := range recv.MetaDataEntries {
			if md.Name == "android.app.device_admin" && md.Resource != "" {
				hasDeviceAdminMeta = true
				break
			}
		}
		if !hasDeviceAdminMeta {
			ctx.Emit(manifestFinding(m.Path, recv.Line, r.BaseRule,
				fmt.Sprintf("Receiver `%s` handles DEVICE_ADMIN_ENABLED but is missing "+
					"`<meta-data android:name=\"android.app.device_admin\" android:resource=\"@xml/device_admin\"/>`.",
					recv.Name)))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: FullBackupContentManifestRule
// ---------------------------------------------------------------------------

// FullBackupContentManifestRule detects issues with the fullBackupContent
// attribute. When allowBackup is true and targetSdk >= 23, the application
// should declare fullBackupContent to control what gets backed up.
type FullBackupContentManifestRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. Android manifest features rule. Detection flags required/optional
// feature declarations and version constraints via attribute presence
// checks. Classified per roadmap/17.
func (r *FullBackupContentManifestRule) Confidence() float64 { return 0.75 }

func (r *FullBackupContentManifestRule) check(ctx *v2.Context) {
	m, _ := ctx.Manifest.(*Manifest)
	if m.Application == nil {
		return
	}
	app := m.Application

	// Check if allowBackup is explicitly false — no issue
	if app.AllowBackup != nil && !*app.AllowBackup {
		return
	}

	// When allowBackup is true (or default) and targetSdk >= 23, missing fullBackupContent is an issue
	if m.TargetSDK >= 23 && app.FullBackupContent == "" {
		ctx.Emit(manifestFinding(m.Path, app.Line, r.BaseRule,
			"Missing `android:fullBackupContent` attribute on <application>. "+
				"When allowBackup is true and targetSdkVersion >= 23, specify fullBackupContent "+
				"to control which files are backed up. "+
				"https://developer.android.com/guide/topics/data/autobackup"))
		return
	}

	return
}

// ---------------------------------------------------------------------------
// Rule: MissingRegisteredManifestRule
// ---------------------------------------------------------------------------

// MissingRegisteredManifestRule detects component class names in the manifest
// that look invalid — empty name, name starting with a digit, or other
// obviously malformed class references.
type MissingRegisteredManifestRule struct {
	ManifestBase
	AndroidRule
}

func isInvalidComponentName(name string) (bool, string) {
	if name == "" {
		return true, "empty component name"
	}
	// Strip leading dot (relative class name)
	checkName := name
	if strings.HasPrefix(checkName, ".") {
		checkName = checkName[1:]
	}
	if checkName == "" {
		return true, "component name is just a dot"
	}
	// Check if name starts with a digit
	if checkName[0] >= '0' && checkName[0] <= '9' {
		return true, "component name starts with a digit"
	}
	// Check for invalid characters (only letters, digits, dots, underscores, $ allowed)
	for _, c := range checkName {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '$') {
			return true, fmt.Sprintf("component name contains invalid character '%c'", c)
		}
	}
	return false, ""
}

// Confidence reports a tier-2 (medium) base confidence. Android manifest features rule. Detection flags required/optional
// feature declarations and version constraints via attribute presence
// checks. Classified per roadmap/17.
func (r *MissingRegisteredManifestRule) Confidence() float64 { return 0.75 }

func (r *MissingRegisteredManifestRule) check(ctx *v2.Context) {
	m, _ := ctx.Manifest.(*Manifest)
	if m.Application == nil {
		return
	}
	for _, c := range allComponents(m.Application) {
		invalid, reason := isInvalidComponentName(c.Name)
		if invalid {
			ctx.Emit(manifestFinding(m.Path, c.Line, r.BaseRule,
				fmt.Sprintf("Invalid component name `%s` in <%s>: %s.",
					c.Name, c.Tag, reason)))
		}
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
