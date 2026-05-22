package rules

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/experiment"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ---------------------------------------------------------------------------
// Rule: AllowBackupManifest
// ---------------------------------------------------------------------------

// AllowBackupManifestRule checks for android:allowBackup="true" in <application>.
// When allowBackup is true (or unset, which defaults to true), app data can be
// backed up and restored via adb, which has security implications.
type AllowBackupManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *AllowBackupManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	// Skip variant manifests — they merge into main and inherit attributes.
	if isNonMainManifestPath(m.Path) {
		return
	}
	if isLibraryOrTestModuleManifest(m.Path) {
		return
	}
	app := m.Application
	if app.AllowBackup == nil {
		// Not set — defaults to true, which is the risky default
		ctx.Emit(manifestFinding(m.Path, app.Line, r.BaseRule,
			"Missing `android:allowBackup` attribute on <application>. "+
				"Consider explicitly setting android:allowBackup=\"false\" to disable backup. "+
				"http://developer.android.com/reference/android/R.attr.html#allowBackup"))
		return
	}
	if *app.AllowBackup {
		ctx.Emit(manifestFinding(m.Path, app.Line, r.BaseRule,
			"`android:allowBackup=\"true\"` allows app data to be backed up via adb. "+
				"Set to false if your app handles sensitive data. "+
				"http://developer.android.com/reference/android/R.attr.html#allowBackup"))
		return
	}
}

// ---------------------------------------------------------------------------
// Rule: DebuggableManifest
// ---------------------------------------------------------------------------

// DebuggableManifestRule checks for android:debuggable="true" in <application>.
// The debuggable flag should not be hardcoded in the manifest; it should be
// controlled by the build system.
type DebuggableManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *DebuggableManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	app := m.Application
	if app.Debuggable != nil && *app.Debuggable {
		ctx.Emit(manifestFinding(m.Path, app.Line, r.BaseRule,
			"`android:debuggable=\"true\"` is set in the manifest. "+
				"This should be controlled by the build system (debug vs release build type). "+
				"Remove the debuggable attribute from the manifest."))
		return
	}
}

// DeepLinkMissingAutoVerifyRule checks for web deep-link intent filters that
// are missing android:autoVerify="true".
type DeepLinkMissingAutoVerifyRule struct {
	ManifestBase
	AndroidRule
}

func (r *DeepLinkMissingAutoVerifyRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *DeepLinkMissingAutoVerifyRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m == nil || m.Path == "" {
		return
	}
	filters := deepLinkMissingAutoVerifyFilters(m.Path)
	for _, filter := range filters {
		ctx.Emit(manifestFinding(m.Path, filter.line, r.BaseRule,
			"HTTP(S) deep link intent-filter is missing android:autoVerify=\"true\". Add autoVerify for verified Android App Links."))
	}
}

type deepLinkIntentFilter struct {
	line       int
	autoVerify bool
	hasDefault bool
	hasBrowse  bool
	hasWebData bool
}

func deepLinkMissingAutoVerifyFilters(path string) []deepLinkIntentFilter {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	var out []deepLinkIntentFilter
	var stack []string
	var filter *deepLinkIntentFilter
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			parent := ""
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			stack = append(stack, t.Name.Local)
			if t.Name.Local == "intent-filter" && (parent == "activity" || parent == "activity-alias") {
				filter = &deepLinkIntentFilter{line: xmlLineAtOffset(data, decoder.InputOffset())}
				for _, attr := range t.Attr {
					if attr.Name.Local == "autoVerify" && strings.EqualFold(attr.Value, "true") {
						filter.autoVerify = true
					}
				}
				continue
			}
			if filter == nil {
				continue
			}
			switch t.Name.Local {
			case "category":
				name := androidXMLAttr(t.Attr, "name")
				if name == "android.intent.category.DEFAULT" {
					filter.hasDefault = true
				}
				if name == "android.intent.category.BROWSABLE" {
					filter.hasBrowse = true
				}
			case "data":
				scheme := strings.ToLower(androidXMLAttr(t.Attr, "scheme"))
				if scheme == "http" || scheme == "https" {
					filter.hasWebData = true
				}
			}
		case xml.EndElement:
			if filter != nil && t.Name.Local == "intent-filter" {
				if !filter.autoVerify && filter.hasDefault && filter.hasBrowse && filter.hasWebData {
					out = append(out, *filter)
				}
				filter = nil
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return out
}

func androidXMLAttr(attrs []xml.Attr, local string) string {
	for _, attr := range attrs {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}

func xmlLineAtOffset(data []byte, offset int64) int {
	if offset < 0 {
		offset = 0
	}
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	line := 1
	for _, b := range data[:offset] {
		if b == '\n' {
			line++
		}
	}
	return line
}

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
	if isTestManifestPath(m.Path) {
		return
	}
	skipSystemActions := experiment.Enabled("exported-without-permission-skip-system-actions")
	components := allComponents(m.Application)
	for _, c := range components {
		if c.Exported == nil || !*c.Exported || c.Permission != "" {
			continue
		}
		// Skip components with LAUNCHER intent-filter — launcher activities
		// must be exported and don't need a custom permission.
		if hasIntentFilterCategory(c, "android.intent.category.LAUNCHER") {
			continue
		}
		// Skip components with BROWSABLE intent-filter — deep link entry points
		// must be exported and cannot require custom permissions (the browser
		// invoking them wouldn't hold the permission).
		if hasIntentFilterCategory(c, "android.intent.category.BROWSABLE") {
			continue
		}
		if skipSystemActions && componentHasPublicSystemAction(c) {
			// Public system actions (ACTION_SEND share targets, sync
			// adapters, account authenticators, job services, etc.) must
			// remain exported without a custom permission — requiring one
			// would break the contract with the calling system service.
			continue
		}
		ctx.Emit(manifestFinding(m.Path, c.Line, r.BaseRule,
			fmt.Sprintf("Exported %s `%s` does not require a permission. "+
				"Add android:permission to restrict access.",
				c.Tag, c.Name)))
	}
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

func componentHasPublicSystemAction(c ManifestComponent) bool {
	for _, a := range c.IntentFilterActions {
		if publicSystemActions[a] {
			return true
		}
	}
	return false
}

// isNonMainManifestPath returns true if the manifest is in a non-main source
// set (test/benchmark/debug/flavor/buildType variant). Only the main manifest
// is expected to contain complete version/SDK info and production security
// constraints — variant manifests merge into main.
func isNonMainManifestPath(path string) bool {
	// Normalize path separators
	p := strings.ReplaceAll(path, "\\", "/")
	// Match /src/<segment>/AndroidManifest.xml where segment != "main"
	idx := strings.Index(p, "/src/")
	if idx < 0 {
		return false
	}
	rest := p[idx+5:]
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return false
	}
	segment := rest[:slash]
	return segment != "main"
}

// isTestManifestPath is shared by security rules.
func isTestManifestPath(path string) bool {
	return isNonMainManifestPath(path) || scanner.IsTestFile(path)
}

func networkSecurityConfigResourceName(ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if strings.HasPrefix(ref, "@xml/") {
		name := strings.TrimPrefix(ref, "@xml/")
		return name, networkSecurityConfigResourceNameValid(name)
	}
	if strings.HasPrefix(ref, "@") {
		if idx := strings.Index(ref, ":xml/"); idx > 1 {
			name := ref[idx+5:]
			return name, networkSecurityConfigResourceNameValid(name)
		}
	}
	return "", false
}

func networkSecurityConfigResourceNameValid(name string) bool {
	if name == "" || strings.ContainsAny(name, "/\\") {
		return false
	}
	return true
}

func networkSecurityConfigPathShipsToRelease(path string) bool {
	p := strings.ReplaceAll(path, "\\", "/")
	idx := strings.Index(p, "/src/")
	if idx < 0 {
		return true
	}
	rest := p[idx+5:]
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return true
	}
	return rest[:slash] == "main"
}

func networkSecurityConfigFindDebugOverrides(node *android.XMLNode) *android.XMLNode {
	if node == nil {
		return nil
	}
	if node.Tag == "debug-overrides" {
		return node
	}
	for _, child := range node.Children {
		if found := networkSecurityConfigFindDebugOverrides(child); found != nil {
			return found
		}
	}
	return nil
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

func (r *MissingExportedFlagRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	components := allComponents(m.Application)
	for _, c := range components {
		if c.HasIntentFilter && c.Exported == nil {
			ctx.Emit(manifestFinding(m.Path, c.Line, r.BaseRule,
				fmt.Sprintf("%s `%s` has an intent-filter but does not set android:exported. "+
					"Starting with Android 12 (API 31), android:exported must be explicitly set "+
					"for components with intent-filters.",
					cases.Title(language.Und).String(c.Tag), c.Name)))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: ExportedServiceManifestRule
// ---------------------------------------------------------------------------

// ExportedServiceManifestRule checks for exported services without a permission.
// Similar to ExportedWithoutPermission but specifically targets services.
type ExportedServiceManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *ExportedServiceManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	for _, svc := range m.Application.Services {
		if svc.Exported != nil && *svc.Exported && svc.Permission == "" {
			ctx.Emit(manifestFinding(m.Path, svc.Line, r.BaseRule,
				fmt.Sprintf("Exported service `%s` does not require a permission. "+
					"Any app can bind to it. Add android:permission to restrict access.",
					svc.Name)))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: ExportedPreferenceActivityManifestRule
// ---------------------------------------------------------------------------

// ExportedPreferenceActivityManifestRule detects activities that likely extend
// PreferenceActivity and are exported. Exported PreferenceActivity subclasses
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
		if !isLikelyPreferenceActivity(act.Name) {
			continue
		}
		exported := (act.Exported != nil && *act.Exported) || act.HasIntentFilter
		if exported {
			ctx.Emit(manifestFinding(m.Path, act.Line, r.BaseRule,
				fmt.Sprintf("Activity `%s` appears to extend PreferenceActivity and is exported. "+
					"Exported PreferenceActivity subclasses are vulnerable to fragment injection attacks. "+
					"Restrict access with android:permission or set android:exported=\"false\".",
					act.Name)))
		}
	}
}

func isLikelyPreferenceActivity(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "preferenceactivity") || strings.Contains(lower, "settingsactivity")
}

// ---------------------------------------------------------------------------
// Rule: CleartextTrafficRule
// ---------------------------------------------------------------------------

// CleartextTrafficRule flags android:usesCleartextTraffic="true" as a security concern.
type CleartextTrafficRule struct {
	ManifestBase
	AndroidRule
}

func (r *CleartextTrafficRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	if m.Application.UsesCleartextTraffic != nil && *m.Application.UsesCleartextTraffic {
		ctx.Emit(manifestFinding(m.Path, m.Application.Line, r.BaseRule,
			"`android:usesCleartextTraffic=\"true\"` allows unencrypted HTTP traffic. "+
				"This is a security risk. Use HTTPS and set usesCleartextTraffic to false, "+
				"or use a Network Security Config to restrict cleartext to specific domains."))
		return
	}
}

// ---------------------------------------------------------------------------
// Rule: BackupRulesRule
// ---------------------------------------------------------------------------

// BackupRulesRule checks that the <application> element has backup configuration
// attributes (android:fullBackupContent or android:dataExtractionRules).
type BackupRulesRule struct {
	ManifestBase
	AndroidRule
}

func (r *BackupRulesRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	app := m.Application
	// If backup is explicitly disabled, no need for backup rules
	if app.AllowBackup != nil && !*app.AllowBackup {
		return
	}
	if app.FullBackupContent == "" && app.DataExtractionRules == "" {
		ctx.Emit(manifestFinding(m.Path, app.Line, r.BaseRule,
			"Missing backup configuration. Add `android:fullBackupContent` (API < 31) "+
				"or `android:dataExtractionRules` (API 31+) to control what data is backed up."))
		return
	}
}

// ---------------------------------------------------------------------------
// Rule: InsecureBaseConfigurationManifestRule
// ---------------------------------------------------------------------------

// InsecureBaseConfigurationManifestRule detects missing networkSecurityConfig
// on the application element when targeting API 28+. Without a network security
// config, the app uses the platform default which may allow cleartext traffic.
type InsecureBaseConfigurationManifestRule struct {
	ManifestBase
	AndroidRule
}

// NetworkSecurityConfigDebugOverridesRule detects main/shared network security
// config resources that ship debug-only trust anchors to release builds.
type NetworkSecurityConfigDebugOverridesRule struct {
	ManifestBase
	AndroidRule
}

func (r *NetworkSecurityConfigDebugOverridesRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *InsecureBaseConfigurationManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	// Skip variant manifests.
	if isNonMainManifestPath(m.Path) {
		return
	}
	if isLibraryOrTestModuleManifest(m.Path) {
		return
	}
	if m.TargetSDK > 0 && m.TargetSDK < 28 {
		return
	}
	if m.Application.NetworkSecurityConfig == "" {
		ctx.Emit(manifestFinding(m.Path, m.Application.Line, r.BaseRule,
			"Missing `android:networkSecurityConfig` on <application> with targetSdkVersion >= 28. "+
				"Add a Network Security Configuration to explicitly control network security behavior. "+
				"https://developer.android.com/training/articles/security-config"))
		return
	}
}

func (r *NetworkSecurityConfigDebugOverridesRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m == nil || m.Application == nil || m.Application.NetworkSecurityConfig == "" {
		return
	}
	if isNonMainManifestPath(m.Path) || isLibraryOrTestModuleManifest(m.Path) {
		return
	}
	resourceName, ok := networkSecurityConfigResourceName(m.Application.NetworkSecurityConfig)
	if !ok {
		return
	}
	configPath := filepath.Join(filepath.Dir(m.Path), "res", "xml", resourceName+".xml")
	if !networkSecurityConfigPathShipsToRelease(configPath) {
		return
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	root, err := android.ParseXMLAST(context.Background(), data)
	if err != nil {
		return
	}
	debugNode := networkSecurityConfigFindDebugOverrides(root)
	if debugNode == nil {
		return
	}
	ctx.Emit(manifestFinding(configPath, debugNode.Line, r.BaseRule,
		"`<debug-overrides>` is declared in a main/shared network security config. Move debug trust anchors to `src/debug/res/xml/` so they do not ship in release builds."))
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
				ctx.Emit(manifestFinding(m.Path, recv.Line, r.BaseRule,
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
// a permission. Protected broadcasts should still require a permission guard.
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
				ctx.Emit(manifestFinding(m.Path, recv.Line, r.BaseRule,
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
				ctx.Emit(manifestFinding(m.Path, svc.Line, r.BaseRule,
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
var protectedPermissions = map[string]bool{
	"android.permission.BRICK":                          true,
	"android.permission.SET_TIME":                       true,
	"android.permission.STATUS_BAR":                     true,
	"android.permission.MASTER_CLEAR":                   true,
	"android.permission.SET_ANIMATION_SCALE":            true,
	"android.permission.INSTALL_PACKAGES":               true,
	"android.permission.DELETE_PACKAGES":                true,
	"android.permission.REBOOT":                         true,
	"android.permission.DEVICE_POWER":                   true,
	"android.permission.MOUNT_UNMOUNT_FILESYSTEMS":      true,
	"android.permission.WRITE_SECURE_SETTINGS":          true,
	"android.permission.READ_LOGS":                      true,
	"android.permission.DUMP":                           true,
	"android.permission.HARDWARE_TEST":                  true,
	"android.permission.CHANGE_COMPONENT_ENABLED_STATE": true,
	"android.permission.FORCE_STOP_PACKAGES":            true,
	"android.permission.INTERNAL_SYSTEM_WINDOW":         true,
	"android.permission.MANAGE_APP_TOKENS":              true,
	"android.permission.INJECT_EVENTS":                  true,
	"android.permission.SET_PREFERRED_APPLICATIONS":     true,
	"android.permission.WRITE_GSERVICES":                true,
	"android.permission.GLOBAL_SEARCH":                  true,
	"android.permission.MANAGE_USB":                     true,
	"android.permission.ACCESS_SURFACE_FLINGER":         true,
	"android.permission.SHUTDOWN":                       true,
	"android.permission.STOP_APP_SWITCHES":              true,
	"android.permission.CONTROL_INCALL_EXPERIENCE":      true,
}

func (r *ProtectedPermissionsManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	for _, perm := range m.UsesPermissions {
		if protectedPermissions[perm] {
			short := perm
			if idx := strings.LastIndex(perm, "."); idx >= 0 {
				short = perm[idx+1:]
			}
			ctx.Emit(manifestFinding(m.Path, 1, r.BaseRule,
				fmt.Sprintf("Permission `%s` is only granted to system apps. "+
					"Third-party apps cannot acquire this permission.",
					short)))
		}
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
			ctx.Emit(manifestFinding(m.Path, svc.Line, r.BaseRule,
				fmt.Sprintf("Exported service `%s` does not require a permission. "+
					"Any app can bind to it. Consider adding android:permission.",
					svc.Name)))
		}
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
