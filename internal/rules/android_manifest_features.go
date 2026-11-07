package rules

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// ---------------------------------------------------------------------------
// Rule: RtlEnabledManifestRule
// ---------------------------------------------------------------------------

// RtlEnabledManifestRule ports Lint's RtlEnabled. A module whose resources use textAlignment or a start/end
// gravity without its manifest saying whether it supports RTL gets a finding on its manifest, and
// supportsRtl="true" is reported when the app targets below API 17. Like Lint, both apply only through an app
// that includes the module (the module itself when it has an <application>), and an unset supportsRtl counts as on.
type RtlEnabledManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *RtlEnabledManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Path == "" || isNonMainManifestPath(m.Path) {
		return
	}
	facts := librarymodel.EnsureFacts(ctx.LibraryFacts)
	mains := rtlResolveMains(ctx, facts, m.Path)
	module := newRtlModuleCache(ctx, facts).forLayout(filepath.Join(filepath.Dir(m.Path), "res", "layout", "x.xml"))
	if app := m.Application; app != nil && app.SupportsRtl != nil {
		if *app.SupportsRtl && !module.modern {
			if _, ok := rtlApplies(rtlTargetBelow17, mains, module); ok {
				ctx.Emit(baseFinding(m.Path, app.Line, r.BaseRule,
					fmt.Sprintf("You must set `android:targetSdkVersion` to at least %d when enabling RTL support", rtlAPI)))
			}
		}
		return
	}
	if _, ok := rtlApplies(rtlEnabled, mains, module); ok && resourcesUseRtlAttributes(filepath.Join(filepath.Dir(m.Path), "res")) {
		ctx.Emit(baseFinding(m.Path, 1, r.BaseRule,
			"The project references RTL attributes, but does not explicitly enable or disable RTL support with `android:supportsRtl` in the manifest"))
	}
}

var (
	rtlTextAlignmentRe = regexp.MustCompile(`\bandroid:textAlignment\s*=`)
	rtlGravityRe       = regexp.MustCompile(`\bandroid:(?:layout_)?gravity\s*=\s*"([^"]*)"`)
)

// resourcesUseRtlAttributes reports whether any XML resource under resDir (outside values) sets textAlignment or
// a start/end gravity, the two usages RtlDetector counts as referencing RTL attributes.
func resourcesUseRtlAttributes(resDir string) bool {
	entries, err := os.ReadDir(resDir)
	if err != nil {
		return false
	}
	for _, dir := range entries {
		if !dir.IsDir() || strings.HasPrefix(dir.Name(), "values") {
			continue
		}
		files, err := os.ReadDir(filepath.Join(resDir, dir.Name()))
		if err != nil {
			continue
		}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".xml") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(resDir, dir.Name(), file.Name()))
			if err == nil && rtlXMLUsesRtlAttributes(data) {
				return true
			}
		}
	}
	return false
}

func rtlXMLUsesRtlAttributes(data []byte) bool {
	if rtlTextAlignmentRe.Match(data) {
		return true
	}
	for _, match := range rtlGravityRe.FindAllSubmatch(data, -1) {
		value := string(match[1])
		if strings.Contains(value, "left") || strings.Contains(value, "right") {
			continue
		}
		if strings.Contains(value, "start") || strings.Contains(value, "end") {
			return true
		}
	}
	return false
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

func (r *GoogleAppIndexingDeepLinkErrorManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil || m.Path == "" {
		return
	}
	if isNonMainManifestPath(m.Path) {
		return
	}
	emit := func(line int, msg string) {
		ctx.Emit(baseFinding(m.Path, line, r.BaseRule, msg))
	}
	for _, f := range manifestActivityIntentFilters(m.Path) {
		hasWebScheme, hasHost := false, false
		for _, d := range f.data {
			scheme := strings.ToLower(resolveManifestStringRef(ctx, d.scheme))
			if scheme == "http" || scheme == "https" {
				hasWebScheme = true
			}
			if d.host != "" {
				hasHost = true
			}
		}
		// Custom schemes are not web deep links, so only filters with an http(s) scheme are checked.
		if !hasWebScheme {
			continue
		}
		where := "Activity `" + f.activity + "`: "
		if !f.hasAction("android.intent.action.VIEW") {
			emit(f.line, where+"web deep link intent filter is missing the `android.intent.action.VIEW` action.")
		}
		if !f.hasCategory("android.intent.category.BROWSABLE") {
			emit(f.line, where+"web deep link intent filter is missing the `android.intent.category.BROWSABLE` category.")
		}
		if !f.hasCategory("android.intent.category.DEFAULT") {
			emit(f.line, where+"web deep link intent filter is missing the `android.intent.category.DEFAULT` category.")
		}
		if !hasHost {
			emit(f.line, where+"web deep link intent filter has an http/https scheme but no `android:host`. "+
				"A web URI with a scheme but no host is malformed.")
		}
		for _, d := range f.data {
			for _, p := range [][2]string{{"path", d.path}, {"pathPrefix", d.pathPrefix}} {
				if p[1] != "" && !strings.HasPrefix(p[1], "/") && !strings.HasPrefix(p[1], "@") && !strings.Contains(p[1], "${") {
					emit(d.line, where+"`android:"+p[0]+"` should start with `/`: `"+p[1]+"`.")
				}
			}
		}
	}
}

// manifestFilter is one <intent-filter> of an activity or activity-alias, read from the raw manifest.
type manifestFilter struct {
	activity   string
	line       int
	actions    []string
	categories []string
	data       []manifestFilterData
}

type manifestFilterData struct {
	line                           int
	scheme, host, path, pathPrefix string
}

func (f manifestFilter) hasAction(name string) bool {
	for _, a := range f.actions {
		if a == name {
			return true
		}
	}
	return false
}

func (f manifestFilter) hasCategory(name string) bool {
	for _, c := range f.categories {
		if c == name {
			return true
		}
	}
	return false
}

// manifestActivityIntentFilters lists each intent filter of every activity with its own actions,
// categories and data elements, so checks apply per filter rather than per component.
func manifestActivityIntentFilters(path string) []manifestFilter {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	var out []manifestFilter
	var stack []string
	var activity string
	var cur *manifestFilter
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
			line := xmlLineAtOffset(data, decoder.InputOffset())
			switch {
			case t.Name.Local == "activity" || t.Name.Local == "activity-alias":
				activity = androidXMLAttr(t.Attr, "name")
			case t.Name.Local == "intent-filter" && (parent == "activity" || parent == "activity-alias"):
				cur = &manifestFilter{activity: activity, line: line}
			case cur != nil && t.Name.Local == "action":
				cur.actions = append(cur.actions, androidXMLAttr(t.Attr, "name"))
			case cur != nil && t.Name.Local == "category":
				cur.categories = append(cur.categories, androidXMLAttr(t.Attr, "name"))
			case cur != nil && t.Name.Local == "data":
				cur.data = append(cur.data, manifestFilterData{
					line:       line,
					scheme:     androidXMLAttr(t.Attr, "scheme"),
					host:       androidXMLAttr(t.Attr, "host"),
					path:       androidXMLAttr(t.Attr, "path"),
					pathPrefix: androidXMLAttr(t.Attr, "pathPrefix"),
				})
			}
		case xml.EndElement:
			if cur != nil && t.Name.Local == "intent-filter" {
				out = append(out, *cur)
				cur = nil
			}
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return out
}

// resolveManifestStringRef resolves an `@string/name` value through the resource
// index when one is available; unresolvable references return "".
func resolveManifestStringRef(ctx *api.Context, value string) string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, "@") && !strings.Contains(value, "${") {
		return value
	}
	name, ok := strings.CutPrefix(value, "@string/")
	if !ok || ctx.ResourceIndex == nil {
		return ""
	}
	return strings.TrimSpace(ctx.ResourceIndex.Strings[name])
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

func (r *GoogleAppIndexingWarningManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	// Skip variant manifests and test fixtures — app indexing is only relevant
	// to production manifests.
	if isTestManifestPath(m.Path) || isLibraryOrTestModuleManifest(m.Path) {
		return
	}
	// Only an application module's manifest can be indexed; skip modules that are not applications.
	if gi := lookupManifestGradleInfo(m.Path); gi.found && !gi.isApplication {
		return
	}
	// A manifest with no intent filters is a fragment of a merged manifest, not a full app entry point.
	hasIntentFilter := false
	for _, act := range m.Application.Activities {
		if act.HasIntentFilter || len(act.IntentFilterActions) > 0 {
			hasIntentFilter = true
			break
		}
	}
	if !hasIntentFilter {
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
	// Lint reports on <application>, which is also where `tools:ignore="GoogleAppIndexingWarning"` goes.
	line := m.Application.Line
	if line < 1 {
		line = 1
	}
	ctx.Emit(baseFinding(m.Path, line, r.BaseRule,
		"No activity with a VIEW intent filter and http/https data scheme found. "+
			"Add deep link support to enable Google App Indexing. "+
			"https://developer.android.com/training/app-indexing"))
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

// ---------------------------------------------------------------------------
// Rule: MissingLeanbackSupportManifestRule
// ---------------------------------------------------------------------------

// MissingLeanbackSupportManifestRule detects apps that declare the
// android.software.leanback feature, or a LEANBACK_LAUNCHER activity, but do
// not opt out of the touchscreen requirement. TV apps must declare
// <uses-feature android:name="android.hardware.touchscreen" android:required="false" />
// because TV devices do not have touchscreens.
type MissingLeanbackSupportManifestRule struct {
	ManifestBase
	AndroidRule
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

// chromeOsUnsupportedFeatures lists features Lint reports as unavailable on some Chrome OS devices.
var chromeOsUnsupportedFeatures = map[string]bool{
	"android.hardware.camera.autofocus":                         true,
	"android.hardware.camera.capability.manual_post_processing": true,
	"android.hardware.camera.capability.manual_sensor":          true,
	"android.hardware.camera.capability.raw":                    true,
	"android.hardware.camera.flash":                             true,
	"android.hardware.camera.level.full":                        true,
	"android.hardware.consumerir":                               true,
	"android.hardware.location.gps":                             true,
	"android.hardware.nfc":                                      true,
	"android.hardware.nfc.hce":                                  true,
	"android.hardware.sensor.accelerometer":                     true,
	"android.hardware.sensor.barometer":                         true,
	"android.hardware.sensor.compass":                           true,
	"android.hardware.sensor.gyroscope":                         true,
	"android.hardware.sensor.light":                             true,
	"android.hardware.sensor.proximity":                         true,
	"android.hardware.sensor.stepcounter":                       true,
	"android.hardware.sensor.stepdetector":                      true,
	"android.hardware.telephony":                                true,
	"android.hardware.telephony.cdma":                           true,
	"android.hardware.telephony.gsm":                            true,
	"android.hardware.touchscreen":                              true,
	"android.hardware.type.automotive":                          true,
	"android.hardware.type.television":                          true,
	"android.hardware.usb.accessory":                            true,
	"android.hardware.usb.host":                                 true,
	"android.software.app_widgets":                              true,
	"android.software.device_admin":                             true,
	"android.software.home_screen":                              true,
	"android.software.input_methods":                            true,
	"android.software.leanback":                                 true,
	"android.software.live_tv":                                  true,
	"android.software.live_wallpaper":                           true,
	"android.software.managed_users":                            true,
	"android.software.midi":                                     true,
	"android.software.sip":                                      true,
	"android.software.sip.voip":                                 true,
}

// isChromeOsUnsupportedFeature also matches every android.hardware.camera feature, like Lint.
func isChromeOsUnsupportedFeature(name string) bool {
	return chromeOsUnsupportedFeatures[name] || strings.HasPrefix(name, "android.hardware.camera")
}

func (r *UnsupportedChromeOsHardwareManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	for _, f := range m.UsesFeatures {
		if !isChromeOsUnsupportedFeature(f.Name) {
			continue
		}
		if strings.EqualFold(f.Required, "false") {
			continue
		}
		ctx.Emit(baseFinding(m.Path, f.Line, r.BaseRule,
			"Expecting `android:required=\"false\"` for this hardware feature that may not be supported by all Chrome OS devices"))
	}
}

// ---------------------------------------------------------------------------
// Rule: DeviceAdminManifestRule
// ---------------------------------------------------------------------------

// DeviceAdminManifestRule mirrors Lint's checkDeviceAdmin: a <receiver> that
// declares the android.app.device_admin meta-data must also have an
// intent filter for android.app.action.DEVICE_ADMIN_ENABLED without data.
type DeviceAdminManifestRule struct {
	ManifestBase
	AndroidRule
}

func (r *DeviceAdminManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	app := manifestApplicationNode(manifestRoot(m))
	if app == nil {
		return
	}
	for _, recv := range xmlChildrenNamed(app, "receiver") {
		var metaName android.XMLAttribute
		hasMeta, hasFilter := false, false
		for _, child := range recv.Children {
			switch child.Tag {
			case "intent-filter":
				enabled, hasData := false, false
				for _, c := range child.Children {
					switch {
					case c.Tag == "action" && xmlAttrValue(c, "android:name") == "android.app.action.DEVICE_ADMIN_ENABLED":
						enabled = true
					case c.Tag == "data":
						hasData = true
					}
				}
				if enabled && !hasData {
					hasFilter = true
				}
			case "meta-data":
				if a, ok := xmlAttrOf(child, "android:name"); ok && a.Value == "android.app.device_admin" {
					metaName, hasMeta = a, true
				}
			}
		}
		if hasMeta && !hasFilter {
			ctx.Emit(baseFindingAt(m.Path, metaName.Line, metaName.Col, r.BaseRule,
				"You must have an intent filter for action `android.app.action.DEVICE_ADMIN_ENABLED`"))
		}
	}
}

// ---------------------------------------------------------------------------
// Rule: FullBackupContentManifestRule
// ---------------------------------------------------------------------------

// FullBackupContentManifestRule validates the XML resources referenced by android:fullBackupContent
// and android:dataExtractionRules: every include/exclude needs a known domain and a path without "..".
type FullBackupContentManifestRule struct {
	ManifestBase
	AndroidRule
}

// backupDomains are the domain values accepted in backup rule files.
var backupDomains = map[string]bool{
	"root": true, "file": true, "database": true, "sharedpref": true, "external": true,
	"device_root": true, "device_file": true, "device_database": true, "device_sharedpref": true,
}

func (r *FullBackupContentManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil || m.Path == "" {
		return
	}
	for _, ref := range []string{m.Application.FullBackupContent, m.Application.DataExtractionRules} {
		name, ok := networkSecurityConfigResourceName(ref)
		if !ok {
			continue
		}
		path := filepath.Join(filepath.Dir(m.Path), "res", "xml", name+".xml")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		root, err := android.ParseXMLAST(context.Background(), data)
		if err != nil {
			continue
		}
		r.checkBackupNode(ctx, path, root)
	}
}

// backupDomainList is the domain order Lint prints in its messages.
const backupDomainList = "file, database, sharedpref, external, root, device_file, device_database, device_sharedpref, device_root"

func (r *FullBackupContentManifestRule) checkBackupNode(ctx *api.Context, path string, root *android.XMLNode) {
	if root == nil {
		return
	}
	switch root.Tag {
	case "full-backup-content":
		r.checkBackupSection(ctx, path, root)
	case "data-extraction-rules":
		for _, section := range root.Children {
			if section.Tag == "cross-platform-transfer" && xmlAttrValue(section, "platform") == "" {
				ctx.Emit(baseFindingAt(path, section.Line, section.Col+1, r.BaseRule, "Missing required attribute `platform`"))
			}
			r.checkBackupSection(ctx, path, section)
		}
	}
}

// backupValueAt anchors a finding at the value of an attribute, as Lint's value location does.
func (r *FullBackupContentManifestRule) backupValueAt(ctx *api.Context, path string, a android.XMLAttribute, msg string) {
	ctx.Emit(baseFindingAt(path, a.Line, a.Col+len(a.Name)+2, r.BaseRule, msg))
}

// backupPath validates a path attribute and returns it ("" when absent).
func (r *FullBackupContentManifestRule) backupPath(ctx *api.Context, path string, n *android.XMLNode) string {
	a, ok := xmlAttrOf(n, "path")
	if !ok {
		return ""
	}
	switch {
	case strings.Contains(a.Value, "//"):
		r.backupValueAt(ctx, path, a, "Paths are not allowed to contain `//`")
	case strings.Contains(a.Value, ".."):
		r.backupValueAt(ctx, path, a, "Paths are not allowed to contain `..`")
	case strings.Contains(a.Value, "/"):
		if d := n.Attr("domain"); d == "sharedpref" || d == "database" {
			r.backupValueAt(ctx, path, a, "Subdirectories are not allowed for domain `"+d+"`")
		}
	}
	return a.Value
}

// backupDomain validates the domain attribute and returns it, or "" when it is missing. An unknown domain is
// reported but still returned, as Lint does.
func (r *FullBackupContentManifestRule) backupDomain(ctx *api.Context, path string, n *android.XMLNode) string {
	a, ok := xmlAttrOf(n, "domain")
	if !ok || a.Value == "" {
		ctx.Emit(baseFindingAt(path, n.Line, n.Col, r.BaseRule, "Missing domain attribute, expected one of "+backupDomainList))
		return ""
	}
	if !backupDomains[a.Value] {
		r.backupValueAt(ctx, path, a, "Unexpected domain `"+a.Value+"`, expected one of "+backupDomainList)
	}
	return a.Value
}

func (r *FullBackupContentManifestRule) checkBackupSection(ctx *api.Context, path string, section *android.XMLNode) {
	var includes, excludes []*android.XMLNode
	for _, child := range section.Children {
		switch {
		case child.Tag == "include":
			includes = append(includes, child)
		case child.Tag == "exclude":
			excludes = append(excludes, child)
		case section.Tag == "cross-platform-transfer" && child.Tag == "platform-specific-params":
		default:
			ctx.Emit(baseFindingAt(path, child.Line, child.Col+1, r.BaseRule, "Unexpected element `<"+child.Tag+">`"))
		}
	}
	included := map[string][]string{}
	hasIncluded := false
	for _, inc := range includes {
		domain := r.backupDomain(ctx, path, inc)
		incPath := r.backupPath(ctx, path, inc)
		if domain == "" {
			continue
		}
		included[domain] = append(included[domain], incPath)
		hasIncluded = true
	}
	for _, exc := range excludes {
		excPath := r.backupPath(ctx, path, exc)
		if excPath == "" {
			continue
		}
		domain := r.backupDomain(ctx, path, exc)
		if domain == "" || !hasIncluded {
			continue
		}
		pathAttr, _ := xmlAttrOf(exc, "path")
		found := false
		for _, incPath := range included[domain] {
			if strings.HasPrefix(excPath, incPath) || incPath == "." {
				found = true
				if excPath == incPath {
					r.backupValueAt(ctx, path, pathAttr, "Include `"+incPath+"` is also excluded")
				}
				break
			}
		}
		if !found {
			r.backupValueAt(ctx, path, pathAttr, "`"+excPath+"` is not in an included path")
		}
	}
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

var manifestPlaceholderRe = regexp.MustCompile(`\$\{[^}]*\}`)

func isInvalidComponentName(name string) (bool, string) {
	if name == "" {
		return true, "empty component name"
	}
	// A resource reference resolves at build time.
	if strings.HasPrefix(name, "@") || strings.HasPrefix(name, "?") {
		return false, ""
	}
	// A manifest placeholder such as ${applicationId} expands to a valid name segment.
	name = manifestPlaceholderRe.ReplaceAllString(name, "P")
	// Strip leading dot (relative class name)
	checkName := name
	checkName = strings.TrimPrefix(checkName, ".")
	if checkName == "" {
		return true, "component name is just a dot"
	}
	// Check if name starts with a digit
	if checkName[0] >= '0' && checkName[0] <= '9' {
		return true, "component name starts with a digit"
	}
	// Check for invalid characters (only letters, digits, dots, underscores, $ allowed)
	for _, c := range checkName {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '.' && c != '_' && c != '$' {
			return true, fmt.Sprintf("component name contains invalid character '%c'", c)
		}
	}
	return false, ""
}

func (r *MissingRegisteredManifestRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m.Application == nil {
		return
	}
	for _, c := range allComponents(m.Application) {
		invalid, reason := isInvalidComponentName(c.Name)
		if invalid {
			ctx.Emit(baseFinding(m.Path, c.Line, r.BaseRule,
				fmt.Sprintf("Invalid component name `%s` in <%s>: %s.",
					c.Name, c.Tag, reason)))
		}
	}
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------
