package rules

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/experiment"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// ---------------------------------------------------------------------------
// Rule: ExportedWithoutPermission
// ---------------------------------------------------------------------------

// ExportedWithoutPermissionRule checks for components that are explicitly
// exported (android:exported="true") but do not declare a permission.
type ExportedWithoutPermissionRule struct {
	ManifestBase
	AndroidRule
}

func (r *ExportedWithoutPermissionRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	// Skip test/benchmark/debug manifests — these legitimately export
	// components for testing infrastructure and aren't shipped to production.
	if isTestManifestPath(m.Path) || isInstrumentationManifest(m) || isTestSupportModuleManifestPath(m.Path) {
		return
	}
	skipSystemActions := experiment.Enabled("exported-without-permission-skip-system-actions")
	components := allComponents(m.Application)
	for _, c := range components {
		// Services, receivers and providers are covered by the Lint-aligned ExportedService,
		// ExportedReceiver and ExportedContentProvider rules.
		if c.Tag != "activity" && c.Tag != "activity-alias" {
			continue
		}
		if c.Exported == nil || !*c.Exported || c.Permission != "" {
			continue
		}
		// Skip components with LAUNCHER intent-filter — launcher activities
		// must be exported and don't need a custom permission.
		if hasIntentFilterCategory(c, "android.intent.category.LAUNCHER") {
			continue
		}
		// An activity-alias with a LAUNCHER filter makes its target a launcher entry point.
		if c.Tag == "activity" && isLauncherAliasTarget(m, c.Name) {
			continue
		}
		// A filterless exported activity in a library or playground module is a test/demo entry
		// point; only application modules ship it as a real external surface.
		if c.Tag == "activity" && !c.HasIntentFilter && len(c.IntentFilterActions) == 0 &&
			(isLibraryOrTestModuleManifest(m.Path) || isPlaygroundManifestPath(m.Path)) {
			continue
		}
		// Skip components with BROWSABLE intent-filter — deep link entry points
		// must be exported and cannot require custom permissions (the browser
		// invoking them wouldn't hold the permission).
		if hasIntentFilterCategory(c, "android.intent.category.BROWSABLE") {
			continue
		}
		// An activity that answers share, view, settings or default-category intents is an entry point;
		// a permission would break the callers that rely on it.
		if (c.Tag == "activity" || c.Tag == "activity-alias") && activityIsExternalEntryPoint(c) {
			continue
		}
		if skipSystemActions && componentHasPublicSystemAction(c) {
			// Public system actions (ACTION_SEND share targets, sync
			// adapters, account authenticators, job services, etc.) must
			// remain exported without a custom permission — requiring one
			// would break the contract with the calling system service.
			continue
		}
		ctx.Emit(baseFinding(m.Path, c.Line, r.BaseRule,
			fmt.Sprintf("Exported %s `%s` does not require a permission. "+
				"Add android:permission to restrict access.",
				c.Tag, c.Name)))
	}
}

// activityEntryPointActions are intent actions whose activities must stay reachable from other apps and the system.
var activityEntryPointActions = map[string]bool{
	"android.intent.action.SEND":                     true,
	"android.intent.action.SEND_MULTIPLE":            true,
	"android.intent.action.VIEW":                     true,
	"android.intent.action.MAIN":                     true,
	"android.intent.action.NOTIFICATION_PREFERENCES": true,
	"android.intent.action.APPLICATION_PREFERENCES":  true,
}

// activityIsExternalEntryPoint reports an activity with an entry-point action or a DEFAULT/BROWSABLE category.
func activityIsExternalEntryPoint(c ManifestComponent) bool {
	for _, a := range c.IntentFilterActions {
		if activityEntryPointActions[a] {
			return true
		}
	}
	return hasIntentFilterCategory(c, "android.intent.category.DEFAULT") ||
		hasIntentFilterCategory(c, "android.intent.category.BROWSABLE")
}

// isTestSupportModuleManifestPath reports a manifest inside a test-support module such as test-support.
func isTestSupportModuleManifestPath(path string) bool {
	segments := strings.Split(filepath.ToSlash(path), "/")
	module := ""
	for i := len(segments) - 1; i > 0; i-- {
		if segments[i] == "src" {
			module = strings.ToLower(segments[i-1])
			break
		}
	}
	switch module {
	case "":
		return false
	case "test", "tests", "testing":
		return true
	}
	return strings.HasPrefix(module, "test-") || strings.HasSuffix(module, "-test") ||
		strings.HasSuffix(module, "-testing") || strings.HasSuffix(module, "-tests")
}

// isPlaygroundManifestPath reports whether the manifest sits under a playground or sample directory.
func isPlaygroundManifestPath(path string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(path), "/") {
		switch strings.ToLower(segment) {
		case "playground", "playgrounds", "sample", "samples":
			return true
		}
	}
	return false
}

// publicSystemActions lists intent-filter action names that identify a
// component as a public entry point for the Android system: share targets,
// sync/account plumbing, notification listeners, job services, etc. These
// components must remain exported without custom permissions — adding one
// would prevent the system/other apps from invoking them.
var publicSystemActions = map[string]bool{
	"android.intent.action.SEND":                               true,
	"android.intent.action.SEND_MULTIPLE":                      true,
	"android.intent.action.SENDTO":                             true,
	"android.intent.action.MAIN":                               true,
	"android.intent.action.CREATE_SHORTCUT":                    true,
	"android.intent.action.INSERT":                             true,
	"android.intent.action.INSERT_OR_EDIT":                     true,
	"android.intent.action.PICK":                               true,
	"android.intent.action.CHOOSER":                            true,
	"android.intent.action.GET_CONTENT":                        true,
	"android.intent.action.ASSIST":                             true,
	"android.intent.action.MEDIA_BUTTON":                       true,
	"android.intent.action.DIAL":                               true,
	"android.intent.action.CALL":                               true,
	"android.intent.action.CALL_BUTTON":                        true,
	"android.content.SyncAdapter":                              true,
	"android.accounts.AccountAuthenticator":                    true,
	"android.service.notification.NotificationListenerService": true,
	"android.service.chooser.ChooserTargetService":             true,
	"android.service.quicksettings.action.QS_TILE":             true,
	"android.telecom.ConnectionService":                        true,
	"android.media.browse.MediaBrowserService":                 true,
	"androidx.work.impl.background.systemjob.SystemJobService": true,
	"androidx.work.impl.foreground.SystemForegroundService":    true,
	"android.app.job.JobService":                               true,
	"com.google.firebase.MESSAGING_EVENT":                      true,
}

// isInstrumentationManifest reports whether the manifest declares <instrumentation>,
// which marks a test-only APK manifest.
func isInstrumentationManifest(m *Manifest) bool {
	for _, e := range m.Elements {
		if e.Tag == "instrumentation" {
			return true
		}
	}
	return false
}

// isLauncherAliasTarget reports whether an <activity-alias> with a LAUNCHER filter targets name.
func isLauncherAliasTarget(m *Manifest, name string) bool {
	if m.Application == nil {
		return false
	}
	for _, target := range m.Application.LauncherAliasTargets {
		if qualifyManifestClass(m.Package, target) == qualifyManifestClass(m.Package, name) {
			return true
		}
	}
	return false
}

// qualifyManifestClass expands a manifest class name relative to the package.
func qualifyManifestClass(pkg, name string) string {
	switch {
	case strings.HasPrefix(name, "."):
		return pkg + name
	case !strings.Contains(name, "."):
		return pkg + "." + name
	}
	return name
}

func componentHasPublicSystemAction(c ManifestComponent) bool {
	for _, a := range c.IntentFilterActions {
		if publicSystemActions[a] {
			return true
		}
	}
	return false
}

// hasIntentFilterCategory returns true if any of the component's intent-filters
// contains the given category name.
func hasIntentFilterCategory(c ManifestComponent, category string) bool {
	for _, cat := range c.IntentFilterCategories {
		if cat == category {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Rule: MissingExportedFlag
// ---------------------------------------------------------------------------

// MissingExportedFlagRule checks for components with intent-filters that do
// not explicitly declare android:exported. Starting with API 31 (Android 12),
// android:exported is required for any component that declares intent-filters.
type MissingExportedFlagRule struct {
	ManifestBase
	AndroidRule
}

func (r *MissingExportedFlagRule) check(ctx *api.Context) { checkExportedFlag(ctx, r.BaseRule) }

// ---------------------------------------------------------------------------
// Rule: ExportedServiceManifestRule
// ---------------------------------------------------------------------------

// ExportedServiceManifestRule mirrors Lint's ExportedService check; its logic lives in
// android_manifest_lint_exports.go.
type ExportedServiceManifestRule struct {
	ManifestBase
	AndroidRule
}

// ---------------------------------------------------------------------------
// Rule: ExportedPreferenceActivityManifestRule
// ---------------------------------------------------------------------------

// ExportedPreferenceActivityManifestRule detects activities that extend
// PreferenceActivity (by source superclass chain) and are exported. Exported PreferenceActivity subclasses
// can be exploited to load arbitrary fragment classes.
type ExportedPreferenceActivityManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *ExportedPreferenceActivityManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	for _, act := range m.Application.Activities {
		if !activityExtendsPreferenceActivity(m, act.Name) {
			continue
		}
		// An explicit android:exported wins; filters only export by default when it is unset.
		exported := act.HasIntentFilter
		if act.Exported != nil {
			exported = *act.Exported
		}
		if exported {
			msg := fmt.Sprintf("`PreferenceActivity` subclass %s should not be exported", qualifyManifestClass(m.Package, act.Name))
			if act.Exported == nil {
				msg += " in the manifest"
			}
			ctx.Emit(baseFinding(m.Path, act.Line, r.BaseRule, msg))
		}
	}
}

// activityExtendsPreferenceActivity uses the source superclass chain when the class is declared
// in project sources, and falls back to the simple-name suffix for classes it cannot see.
func activityExtendsPreferenceActivity(m *Manifest, name string) bool {
	if extends, known := m.ActivityExtendsPreferenceActivity[manifestClassFQN(m.Package, name)]; known {
		return extends
	}
	return isLikelyPreferenceActivity(name)
}

// manifestClassFQN expands a relative android:name against the manifest package.
func manifestClassFQN(pkg, name string) string {
	name = strings.ReplaceAll(name, "$", ".")
	switch {
	case strings.HasPrefix(name, "."):
		return pkg + name
	case !strings.Contains(name, ".") && pkg != "":
		return pkg + "." + name
	}
	return name
}

// isLikelyPreferenceActivity matches class names whose simple name ends in
// PreferenceActivity; used only when the class is not declared in project sources.
func isLikelyPreferenceActivity(name string) bool {
	simple := name
	if i := strings.LastIndexAny(simple, ".$"); i >= 0 {
		simple = simple[i+1:]
	}
	return strings.HasSuffix(simple, "PreferenceActivity")
}

// ---------------------------------------------------------------------------
// Rule: UnprotectedSMSBroadcastReceiverManifestRule
// ---------------------------------------------------------------------------

// UnprotectedSMSBroadcastReceiverManifestRule detects broadcast receivers that
// listen for SMS_RECEIVED but do not declare android:permission. Without a
// permission, any app can send fake SMS_RECEIVED broadcasts.
type UnprotectedSMSBroadcastReceiverManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *UnprotectedSMSBroadcastReceiverManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	for _, recv := range m.Application.Receivers {
		if recv.Permission != "" {
			continue
		}
		for _, action := range recv.IntentFilterActions {
			if action == "android.provider.Telephony.SMS_RECEIVED" {
				ctx.Emit(baseFinding(m.Path, recv.Line, r.BaseRule,
					fmt.Sprintf("Receiver `%s` listens for SMS_RECEIVED but has no android:permission. "+
						"Add `android:permission=\"android.permission.BROADCAST_SMS\"` to prevent spoofed broadcasts.",
						recv.Name)))
				break
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: UnsafeProtectedBroadcastReceiverManifestRule
// ---------------------------------------------------------------------------

// UnsafeProtectedBroadcastReceiverManifestRule detects receivers listening for
// protected broadcast actions (e.g., BOOT_COMPLETED) that are exported without
// a permission. This is a Krit-specific check, not AOSP's source-based
// UnsafeProtectedBroadcastReceiver. Only the system can send a protected
// broadcast, so this is opt-in.
type UnsafeProtectedBroadcastReceiverManifestRule struct {
	ManifestBase
	AndroidRule
}

// protectedBroadcastActions lists well-known protected broadcast action names.
var protectedBroadcastActions = map[string]bool{
	"android.intent.action.BOOT_COMPLETED":     true,
	"android.intent.action.PACKAGE_ADDED":      true,
	"android.intent.action.PACKAGE_REMOVED":    true,
	"android.intent.action.PACKAGE_REPLACED":   true,
	"android.intent.action.NEW_OUTGOING_CALL":  true,
	"android.intent.action.PHONE_STATE":        true,
	"android.intent.action.TIMEZONE_CHANGED":   true,
	"android.intent.action.TIME_SET":           true,
	"android.intent.action.LOCALE_CHANGED":     true,
	"android.intent.action.BATTERY_LOW":        true,
	"android.intent.action.BATTERY_OKAY":       true,
	"android.intent.action.POWER_CONNECTED":    true,
	"android.intent.action.POWER_DISCONNECTED": true,
	"android.intent.action.DEVICE_STORAGE_LOW": true,
	"android.intent.action.DEVICE_STORAGE_OK":  true,
}

func (r *UnsafeProtectedBroadcastReceiverManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	for _, recv := range m.Application.Receivers {
		if !recv.HasIntentFilter {
			continue
		}
		exported := recv.Exported != nil && *recv.Exported
		if !exported {
			continue
		}
		if recv.Permission != "" {
			continue
		}
		for _, action := range recv.IntentFilterActions {
			if protectedBroadcastActions[action] {
				ctx.Emit(baseFinding(m.Path, recv.Line, r.BaseRule,
					fmt.Sprintf("Receiver `%s` listens for protected broadcast `%s` and is exported without a permission. "+
						"Add android:permission to prevent unauthorized broadcasts.",
						recv.Name, action)))
				break
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: UseCheckPermissionManifestRule
// ---------------------------------------------------------------------------

// UseCheckPermissionManifestRule detects exported services that handle
// sensitive system intent actions (e.g., BIND_ACCESSIBILITY_SERVICE,
// BIND_INPUT_METHOD) without declaring android:permission. These services
// are security-sensitive and must restrict access via permissions.
type UseCheckPermissionManifestRule struct {
	ManifestBase
	AndroidRule
}

// sensitiveServiceActions lists intent actions for services that require
// permission protection when exported.
var sensitiveServiceActions = map[string]bool{
	"android.accessibilityservice.AccessibilityService":        true,
	"android.view.InputMethod":                                 true,
	"android.service.notification.NotificationListenerService": true,
	"android.service.dreams.DreamService":                      true,
	"android.service.wallpaper.WallpaperService":               true,
	"android.telecom.ConnectionService":                        true,
	"android.telecom.InCallService":                            true,
	"android.service.voice.VoiceInteractionService":            true,
	"android.net.VpnService":                                   true,
	"android.nfc.cardemulation.action.HOST_APDU_SERVICE":       true,
	"android.nfc.cardemulation.action.OFF_HOST_APDU_SERVICE":   true,
}

func (r *UseCheckPermissionManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	for _, svc := range m.Application.Services {
		exported := svc.Exported != nil && *svc.Exported
		if !exported {
			continue
		}
		if svc.Permission != "" {
			continue
		}
		for _, action := range svc.IntentFilterActions {
			if sensitiveServiceActions[action] {
				ctx.Emit(baseFinding(m.Path, svc.Line, r.BaseRule,
					fmt.Sprintf("Exported service `%s` handles sensitive action `%s` "+
						"without declaring android:permission. "+
						"Add a permission attribute to restrict access to this service.",
						svc.Name, action)))
				break
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: ProtectedPermissionsManifestRule
// ---------------------------------------------------------------------------

// ProtectedPermissionsManifestRule detects requests for system-only
// (signature|system) permissions that third-party apps cannot obtain.
type ProtectedPermissionsManifestRule struct {
	ManifestBase
	AndroidRule
}

// protectedPermissions lists permissions that are only available to system apps.
func (r *ProtectedPermissionsManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	minSdk := manifestEffectiveMinSdk(m)
	for _, perm := range m.UsesPermissionEntries {
		last, protected := lintSystemPermissions[perm.Name]
		if !protected {
			continue
		}
		if last != -1 {
			// The permission was grantable up to API last: a maxSdkVersion at or below it is fine, and a
			// project whose minSdk is already past it never saw the permission as grantable.
			if perm.HasMaxSdk && perm.MaxSdk <= last {
				continue
			}
			if minSdk >= last+1 {
				continue
			}
		}
		ctx.Emit(baseFinding(m.Path, perm.Line, r.BaseRule, "Permission is only granted to system apps"))
	}
}

// ---------------------------------------------------------------------------
// Rule: ServiceExportedManifestRule
// ---------------------------------------------------------------------------

// ServiceExportedManifestRule checks for exported services without a
// permission attribute. Uses the AOSP IssueID "ServiceExported" for
// @SuppressLint compatibility. This is the AOSP-compatible variant of
// ExportedServiceManifest.
type ServiceExportedManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *ServiceExportedManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	for _, svc := range m.Application.Services {
		if svc.Exported != nil && *svc.Exported && svc.Permission == "" {
			ctx.Emit(baseFinding(m.Path, svc.Line, r.BaseRule,
				fmt.Sprintf("Exported service `%s` does not require a permission. "+
					"Any app can bind to it. Consider adding android:permission.",
					svc.Name)))
		}
	}
}
