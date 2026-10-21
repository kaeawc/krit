package rules

import (
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/android"
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
		ctx.Emit(baseFinding(m.Path, app.Line, r.BaseRule,
			"Missing `android:allowBackup` attribute on <application>. "+
				"Consider explicitly setting android:allowBackup=\"false\" to disable backup. "+
				"http://developer.android.com/reference/android/R.attr.html#allowBackup"))
		return
	}
	if *app.AllowBackup {
		ctx.Emit(baseFinding(m.Path, app.Line, r.BaseRule,
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
		ctx.Emit(baseFinding(m.Path, app.Line, r.BaseRule,
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
		ctx.Emit(baseFinding(m.Path, filter.line, r.BaseRule,
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
		ctx.Emit(baseFinding(m.Path, m.Application.Line, r.BaseRule,
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
		ctx.Emit(baseFinding(m.Path, app.Line, r.BaseRule,
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
		ctx.Emit(baseFinding(m.Path, m.Application.Line, r.BaseRule,
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
	ctx.Emit(baseFinding(configPath, debugNode.Line, r.BaseRule,
		"`<debug-overrides>` is declared in a main/shared network security config. Move debug trust anchors to `src/debug/res/xml/` so they do not ship in release builds."))
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
