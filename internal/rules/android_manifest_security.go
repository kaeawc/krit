package rules

import (
	"bytes"
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
// Rule: GrantAllUris
// ---------------------------------------------------------------------------

// GrantAllUrisRule ports Android Lint's GrantAllUris: a <grant-uri-permission>
// whose android:path, android:pathPrefix or android:pathPattern is just "/"
// shares every URI of the provider. The check reads the manifest only; calls
// to grantUriPermission in source are not part of Lint's rule.
type GrantAllUrisRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a tier-1 (high) base confidence: the check compares
// attribute values in the manifest exactly as the Lint detector does.
func (r *GrantAllUrisRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *GrantAllUrisRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m == nil || m.Path == "" {
		return
	}
	data, err := os.ReadFile(m.Path)
	if err != nil || !bytes.Contains(data, []byte("grant-uri-permission")) {
		return
	}
	root, err := android.ParseXMLAST(context.Background(), data)
	if err != nil {
		return
	}
	r.checkNode(ctx, m.Path, root)
}

func (r *GrantAllUrisRule) checkNode(ctx *api.Context, path string, node *android.XMLNode) {
	if node.Tag == "grant-uri-permission" {
		for _, attr := range node.Attrs {
			switch attr.Name {
			case "android:path", "android:pathPrefix", "android:pathPattern":
				if attr.Value == "/" {
					ctx.Emit(baseFindingAt(path, attr.Line, attr.Col, r.BaseRule,
						"Content provider shares everything; this is potentially dangerous"))
				}
			}
		}
	}
	for _, child := range node.Children {
		r.checkNode(ctx, path, child)
	}
}

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
		// An overlay manifest only contributes attributes to the app manifest it merges into.
		if isManifestOverlay(m) {
			return
		}
		// Backup rules show the developer decided what is backed up, so the default is not accidental.
		if app.FullBackupContent != "" || app.DataExtractionRules != "" {
			return
		}
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

// isManifestOverlay reports whether a manifest merges into the app's real manifest
// instead of standing alone: it declares no components, carries a tools:node merge
// marker on <application>, or lives under a bazel directory.
func isManifestOverlay(m *Manifest) bool {
	app := m.Application
	if app == nil {
		return false
	}
	if app.ToolsNode != "" {
		return true
	}
	for _, segment := range strings.Split(strings.ReplaceAll(m.Path, "\\", "/"), "/") {
		if segment == "bazel" {
			return true
		}
	}
	return len(app.Activities) == 0 && len(app.Services) == 0 && len(app.Receivers) == 0 && len(app.Providers) == 0
}

// ---------------------------------------------------------------------------
// Rule: DebuggableManifest
// ---------------------------------------------------------------------------

// DebuggableManifestRule is Lint's HardcodedDebugMode: any android:debuggable value in <application>.
// The flag should come from the build type, not the manifest.
type DebuggableManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *DebuggableManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	// Test and variant manifests do not ship in the release artifact.
	if isTestManifestPath(m.Path) {
		return
	}
	app := m.Application
	// Lint reports any hardcoded android:debuggable value, not only "true".
	if app.Debuggable != nil {
		ctx.Emit(baseFinding(m.Path, app.Line, r.BaseRule,
			"Avoid hardcoding the debug mode; leaving it out allows debug and release builds to automatically assign one"))
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
	hasView    bool
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
		tokenStart := decoder.InputOffset()
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
				filter = &deepLinkIntentFilter{line: xmlLineAtOffset(data, tokenStart)}
				for _, attr := range t.Attr {
					// A placeholder or resource value is unknown, so only "false" is missing.
					if attr.Name.Local == "autoVerify" && !strings.EqualFold(strings.TrimSpace(attr.Value), "false") {
						filter.autoVerify = true
					}
				}
				continue
			}
			if filter == nil {
				continue
			}
			switch t.Name.Local {
			case "action":
				if androidXMLAttr(t.Attr, "name") == "android.intent.action.VIEW" {
					filter.hasView = true
				}
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
				if !filter.autoVerify && filter.hasView && filter.hasDefault && filter.hasBrowse && filter.hasWebData {
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

// isDebugManifestPath reports whether the manifest sits in a debug or test source set such as src/debug, src/stagingDebug or src/androidTest.
func isDebugManifestPath(path string) bool {
	p := strings.ReplaceAll(path, "\\", "/")
	idx := strings.LastIndex(p, "/src/")
	if idx < 0 {
		return false
	}
	rest := p[idx+5:]
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return false
	}
	segment := strings.ToLower(rest[:slash])
	return strings.Contains(segment, "debug") || segment == "androidtest" || segment == "test"
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
	// Debug and other variant manifests do not ship the production configuration.
	if isNonMainManifestPath(m.Path) || isLibraryOrTestModuleManifest(m.Path) {
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
	// Variant manifests merge into the main one, which owns the backup configuration.
	if isNonMainManifestPath(m.Path) || isLibraryOrTestModuleManifest(m.Path) {
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

// InsecureBaseConfigurationManifestRule detects explicitly permitted cleartext
// traffic or user-CA trust: usesCleartextTraffic="true", or a referenced
// network security config whose <base-config> enables either. Settings in
// <domain-config> and <debug-overrides> are scoped and not reported.
// A missing config is safe because API 28+ blocks cleartext by default.
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
	// Cleartext is blocked by default from API 28, so a missing config is safe.
	if cleartext := m.Application.UsesCleartextTraffic; cleartext != nil && *cleartext {
		ctx.Emit(baseFinding(m.Path, m.Application.Line, r.BaseRule,
			"`android:usesCleartextTraffic=\"true\"` permits unencrypted traffic for the whole app. "+
				"Restrict cleartext to specific domains with a Network Security Configuration. "+
				"https://developer.android.com/training/articles/security-config"))
		return
	}
	resourceName, ok := networkSecurityConfigResourceName(m.Application.NetworkSecurityConfig)
	if !ok {
		return
	}
	configPath := filepath.Join(filepath.Dir(m.Path), "res", "xml", resourceName+".xml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	root, err := android.ParseXMLAST(context.Background(), data)
	if err != nil {
		return
	}
	if reason := networkSecurityConfigBaseInsecureReason(root); reason != "" {
		ctx.Emit(baseFinding(m.Path, m.Application.Line, r.BaseRule,
			"The referenced network security config "+reason+". "+
				"https://developer.android.com/training/articles/security-config"))
	}
}

// networkSecurityConfigBaseInsecureReason checks only <base-config>. A <domain-config> that allows
// cleartext or user CAs for named domains is the restriction the rule recommends.
func networkSecurityConfigBaseInsecureReason(node *android.XMLNode) string {
	if node == nil {
		return ""
	}
	if node.Tag == "base-config" {
		return networkSecurityConfigInsecureReason(node)
	}
	for _, child := range node.Children {
		if reason := networkSecurityConfigBaseInsecureReason(child); reason != "" {
			return reason
		}
	}
	return ""
}

// networkSecurityConfigInsecureReason describes an explicit insecure setting
// outside <debug-overrides>, or returns "" when none is present.
func networkSecurityConfigInsecureReason(node *android.XMLNode) string {
	if node == nil || node.Tag == "debug-overrides" {
		return ""
	}
	if node.Attr("cleartextTrafficPermitted") == "true" {
		return "permits cleartext traffic (`cleartextTrafficPermitted=\"true\"`)"
	}
	if node.Tag == "certificates" && node.Attr("src") == "user" {
		return "trusts user-installed certificate authorities"
	}
	for _, child := range node.Children {
		if reason := networkSecurityConfigInsecureReason(child); reason != "" {
			return reason
		}
	}
	return ""
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
		"`<debug-overrides>` is declared in a main/shared network security config. Android only honors it in debuggable builds, but move it to `src/debug/res/xml/` so debug trust anchors are not packaged in release builds."))
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
