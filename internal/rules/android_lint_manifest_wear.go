package rules

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	wearWatchFeature      = "android.hardware.type.watch"
	wearStandaloneMeta    = "com.google.android.wearable.standalone"
	wffVersionProperty    = "com.google.wear.watchface.format.version"
	watchFaceEditorAction = "androidx.wear.watchface.editor.action.WATCH_FACE_EDITOR"
	// wearCodenameTargetSdk stands in for a preview codename target, which is above every numbered release.
	wearCodenameTargetSdk = 1000
)

// InvalidWearFeatureAttributeRule mirrors Lint's InvalidWearFeatureAttribute: android:required="false" on the watch
// uses-feature.
type InvalidWearFeatureAttributeRule struct {
	ManifestBase
	AndroidRule
}

// WearStandaloneAppFlagRule mirrors Lint's WearStandaloneAppFlag: a Wear app without a valid standalone meta-data.
type WearStandaloneAppFlagRule struct {
	ManifestBase
	AndroidRule
}

// WearRecentsRule mirrors Lint's WearRecents (manifest part): Wear activities need noHistory with excludeFromRecents,
// and a taskAffinity otherwise.
type WearRecentsRule struct {
	ManifestBase
	AndroidRule
}

// WearSplashScreenRule mirrors Lint's WearSplashScreen: a launcher activity named like a splash screen in a Wear app
// that does not use the SplashScreen library.
type WearSplashScreenRule struct {
	ManifestBase
	AndroidRule
}

// WatchFaceEditorRule mirrors Lint's WatchFaceEditor: a watch face editor activity with a launchMode other than
// standard.
type WatchFaceEditorRule struct {
	ManifestBase
	AndroidRule
}

// WatchFaceFormatDeclaresHasNoCodeRule mirrors Lint's WatchFaceFormatDeclaresHasNoCode: a Watch Face Format app that
// does not declare hasCode=false.
type WatchFaceFormatDeclaresHasNoCodeRule struct {
	ManifestBase
	AndroidRule
}

// WatchFaceFormatMissingVersionRule mirrors Lint's WatchFaceFormatMissingVersion: a Watch Face Format app without the
// version property, or a version property without a value.
type WatchFaceFormatMissingVersionRule struct {
	ManifestBase
	AndroidRule
}

// WatchFaceFormatInvalidVersionRule mirrors Lint's WatchFaceFormatInvalidVersion: a version property that is not an
// integer.
type WatchFaceFormatInvalidVersionRule struct {
	ManifestBase
	AndroidRule
}

// Confidence reports a high base confidence: the checks read literal manifest attributes.
func (r *InvalidWearFeatureAttributeRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal manifest attributes.
func (r *WearStandaloneAppFlagRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal manifest attributes.
func (r *WearRecentsRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a medium base confidence: the SplashScreen dependency is read from the module build script text.
func (r *WearSplashScreenRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a high base confidence: the checks read literal manifest attributes.
func (r *WatchFaceEditorRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal manifest attributes.
func (r *WatchFaceFormatDeclaresHasNoCodeRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal manifest attributes.
func (r *WatchFaceFormatMissingVersionRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal manifest attributes.
func (r *WatchFaceFormatInvalidVersionRule) Confidence() float64 { return api.ConfidenceHigh }

// wearDeclaresWatch reports whether a direct child of the manifest element is the watch uses-feature, as Lint's
// WearDetector.containsWearFeature does. The required value and tools:node markers do not matter.
func wearDeclaresWatch(root *manifest.Node) bool {
	if root == nil {
		return false
	}
	for _, c := range root.Children {
		if c.Tag == "uses-feature" && c.AttrValue("android:name") == wearWatchFeature {
			return true
		}
	}
	return false
}

// wearManifestPathDeclaresWatch is wearDeclaresWatch for a manifest file, read through the shared parse cache.
func wearManifestPathDeclaresWatch(path string) bool {
	root := manifestRoot(&Manifest{Path: path})
	if root == nil {
		return false
	}
	for _, c := range root.Children {
		if c.Tag == "uses-feature" && c.Attr("android:name") == wearWatchFeature {
			return true
		}
	}
	return false
}

// wearMainManifestPath returns the main source set manifest next to a variant manifest, or "" for the main one.
func wearMainManifestPath(path string) string {
	if !isNonMainManifestPath(path) {
		return ""
	}
	return filepath.Join(filepath.Dir(filepath.Dir(path)), "main", "AndroidManifest.xml")
}

// wearIsWearProject approximates Lint's isWearProject, which reads the merged manifest: a variant manifest also
// counts when the main manifest of its module declares the watch feature.
func wearIsWearProject(m *manifest.Manifest) bool {
	if wearDeclaresWatch(m.Root) {
		return true
	}
	mainPath := wearMainManifestPath(m.Path)
	return mainPath != "" && wearManifestPathDeclaresWatch(mainPath)
}

// wearIsWearResDir reports whether the module that owns a res directory is a Wear project, using the manifest next to
// the directory or, for a variant source set without one, the main manifest.
func wearIsWearResDir(resDir string) bool {
	candidate := filepath.Join(filepath.Dir(resDir), "AndroidManifest.xml")
	if _, err := os.Stat(candidate); err != nil {
		candidate = filepath.Join(filepath.Dir(filepath.Dir(resDir)), "main", "AndroidManifest.xml")
	}
	if wearManifestPathDeclaresWatch(candidate) {
		return true
	}
	mainPath := wearMainManifestPath(candidate)
	return mainPath != "" && wearManifestPathDeclaresWatch(mainPath)
}

func (r *InvalidWearFeatureAttributeRule) check(ctx *api.Context) {
	manifestWalk(ctx.Manifest.Root, func(n, _ *manifest.Node) {
		if n.Tag != "uses-feature" || n.AttrValue("android:name") != wearWatchFeature {
			return
		}
		// Lint parses android:required as a boolean, so references and placeholders count as false.
		if a, ok := n.Attr("android:required"); ok && !strings.EqualFold(a.Value, "true") {
			emitManifestAtAttr(ctx, a, "`android:required=\"false\"` is not supported for this feature")
		}
	})
}

// wearTargetSdk is the project target SDK as Lint resolves it: the declared value, else the minimum SDK. A preview
// codename counts as above every release.
func wearTargetSdk(m *manifest.Manifest) int {
	if uses := firstChild(m.Root, "uses-sdk"); uses != nil {
		if v := uses.AttrValue("android:targetSdkVersion"); v != "" {
			if _, err := strconv.Atoi(v); err != nil && !strings.Contains(v, "$") && !strings.HasPrefix(v, "@") {
				return wearCodenameTargetSdk
			}
		}
	}
	if target := manifestTargetSdk(m); target > 0 {
		return target
	}
	return manifestMinSdk(m)
}

func (r *WearStandaloneAppFlagRule) check(ctx *api.Context) {
	m := ctx.Manifest
	sawWear, sawStandalone := false, false
	manifestWalk(m.Root, func(n, parent *manifest.Node) {
		switch n.Tag {
		case "uses-feature":
			if n.AttrValue("android:name") == wearWatchFeature {
				sawWear = true
			}
		case "meta-data":
			if n.AttrValue("android:name") != wearStandaloneMeta || parent == nil || parent.Tag != "application" {
				return
			}
			sawStandalone = true
			value, ok := n.Attr("android:value")
			switch {
			case !ok:
				emitManifestAtName(ctx, n, "Missing `android:value` attribute")
			case value.Value != "" && !strings.EqualFold(value.Value, "true") && !strings.EqualFold(value.Value, "false"):
				emitManifestAtValue(ctx, value, "Expecting a boolean value for attribute `android:value`")
			}
		}
	})
	app := firstChild(m.Root, "application")
	if !sawWear || sawStandalone || app == nil || wearTargetSdk(m) < 23 || manifestIsLibrary(m) {
		return
	}
	emitManifestAtName(ctx, app, "Missing `<meta-data android:name=\"com.google.android.wearable.standalone\" ../>` element")
}

func (r *WearRecentsRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if !wearIsWearProject(m) {
		return
	}
	manifestWalk(m.Root, func(n, _ *manifest.Node) {
		if n.Tag != "activity" {
			return
		}
		_, excluded := n.Attr("android:excludeFromRecents")
		_, noHistory := n.Attr("android:noHistory")
		_, affinity := n.Attr("android:taskAffinity")
		switch {
		case excluded && !noHistory:
			emitManifestAtNode(ctx, n, "In addition to `excludeFromRecents`, set `noHistory` flag to avoid showing this activity in recents")
		case !excluded && !affinity:
			emitManifestAtNode(ctx, n, "Set `taskAffinity` for Wear activities to make them appear correctly in recents")
		}
	})
}

// firstChild returns the first child element with the tag, or nil.
func firstChild(n *manifest.Node, tag string) *manifest.Node {
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		if c.Tag == tag {
			return c
		}
	}
	return nil
}

// wearIsMainLauncher mirrors the splash check: the first intent-filter's first action is MAIN and its first category is
// LAUNCHER.
func wearIsMainLauncher(component *manifest.Node) bool {
	filter := firstChild(component, "intent-filter")
	if filter == nil {
		return false
	}
	action, category := firstChild(filter, "action"), firstChild(filter, "category")
	return action != nil && action.AttrValue("android:name") == "android.intent.action.MAIN" &&
		category != nil && category.AttrValue("android:name") == "android.intent.category.LAUNCHER"
}

func (r *WearSplashScreenRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if !wearIsWearProject(m) || manifestMinSdk(m) >= 31 {
		return
	}
	if m.Path != "" && lookupManifestGradleInfo(m.Path).usesSplashScreenLibrary {
		return
	}
	manifestWalk(m.Root, func(n, _ *manifest.Node) {
		if (n.Tag == "activity" || n.Tag == "activity-alias") &&
			strings.Contains(n.AttrValue("android:name"), "Splash") && wearIsMainLauncher(n) {
			emitManifestAtNode(ctx, n, "Applications using splash screens are strongly recommended to use the 'androidx.core:core-splashscreen' library")
		}
	})
}

func (r *WatchFaceEditorRule) check(ctx *api.Context) {
	manifestWalk(ctx.Manifest.Root, func(n, _ *manifest.Node) {
		if n.Tag != "activity" {
			return
		}
		mode, ok := n.Attr("android:launchMode")
		if !ok || mode.Value == "" || mode.Value == "standard" {
			return
		}
		for _, filter := range n.ChildrenByTag("intent-filter") {
			for _, action := range filter.ChildrenByTag("action") {
				if action.AttrValue("android:name") == watchFaceEditorAction {
					emitManifestAtAttr(ctx, mode, "Watch face editor must use launchMode=\"standard\"")
				}
			}
		}
	})
}

// wffVersionProperties returns the Watch Face Format version <property> children of the application.
func wffVersionProperties(app *manifest.Node) []*manifest.Node {
	var out []*manifest.Node
	for _, p := range app.ChildrenByTag("property") {
		if p.AttrValue("android:name") == wffVersionProperty {
			out = append(out, p)
		}
	}
	return out
}

// wffHasDeclarativeWatchFace reports whether a raw* resource folder next to the manifest holds an XML file whose root
// element is <WatchFace>.
func wffHasDeclarativeWatchFace(manifestPath string) bool {
	resDir := filepath.Join(filepath.Dir(manifestPath), "res")
	folders, err := os.ReadDir(resDir)
	if err != nil {
		return false
	}
	for _, folder := range folders {
		if !strings.HasPrefix(folder.Name(), "raw") {
			continue
		}
		files, err := os.ReadDir(filepath.Join(resDir, folder.Name()))
		if err != nil {
			continue
		}
		for _, file := range files {
			if wffRootIsWatchFace(filepath.Join(resDir, folder.Name(), file.Name())) {
				return true
			}
		}
	}
	return false
}

func wffRootIsWatchFace(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	dec := xml.NewDecoder(f)
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		if start, ok := tok.(xml.StartElement); ok {
			return start.Name.Local == "WatchFace"
		}
	}
}

var wffResourceURLRe = regexp.MustCompile(`^@\+?(?:([\w.]+):)?(\w+)/([\w.]+)$`)

// wffResourceValues returns the values every values folder of the module defines for a resource. A nil result means
// the resource is not defined (or lives in the framework).
func wffResourceValues(resDir, pkg, typ, name string) []string {
	if pkg == "android" {
		return nil
	}
	var out []string
	folders, _ := os.ReadDir(resDir)
	for _, folder := range folders {
		if !strings.HasPrefix(folder.Name(), "values") {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(resDir, folder.Name()))
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".xml") {
				out = append(out, wffValuesInFile(filepath.Join(resDir, folder.Name(), file.Name()), typ, name)...)
			}
		}
	}
	return out
}

func wffValuesInFile(path, typ, name string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	dec := xml.NewDecoder(f)
	var out []string
	depth := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			return out
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 && wffDefines(t, typ, name) {
				out = append(out, wffElementText(dec))
				depth--
			}
		case xml.EndElement:
			depth--
		}
	}
}

// wffDefines reports whether a values element defines the named resource of the type.
func wffDefines(el xml.StartElement, typ, name string) bool {
	var gotName, gotType string
	for _, a := range el.Attr {
		switch a.Name.Local {
		case "name":
			gotName = a.Value
		case "type":
			gotType = a.Value
		}
	}
	return gotName == name && (el.Name.Local == typ || el.Name.Local == "item" && gotType == typ)
}

// wffElementText reads the character data up to the end of the current element.
func wffElementText(dec *xml.Decoder) string {
	var sb strings.Builder
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 1 {
				sb.Write(t)
			}
		}
	}
	return strings.TrimSpace(sb.String())
}

// wffDeclaresNoCode mirrors the detector's isFalse: a resource reference counts when every definition of it is the
// text "false" (an undefined reference counts too), any other value must be "false" itself.
func wffDeclaresNoCode(manifestPath, value string) bool {
	match := wffResourceURLRe.FindStringSubmatch(value)
	if match == nil {
		return value == "false"
	}
	resDir := filepath.Join(filepath.Dir(manifestPath), "res")
	for _, v := range wffResourceValues(resDir, match[1], match[2], match[3]) {
		if v != "false" {
			return false
		}
	}
	return true
}

func (r *WatchFaceFormatDeclaresHasNoCodeRule) check(ctx *api.Context) {
	m := ctx.Manifest
	app := firstChild(m.Root, "application")
	if app == nil || !wearIsWearProject(m) {
		return
	}
	if len(wffVersionProperties(app)) == 0 && !wffHasDeclarativeWatchFace(m.Path) {
		return
	}
	hasCode, ok := app.Attr("android:hasCode")
	switch {
	case !ok:
		emitManifestAtName(ctx, app, "Applications using Watch Face Format must declare `hasCode=false`")
	case !wffDeclaresNoCode(m.Path, hasCode.Value):
		emitManifestAtAttr(ctx, hasCode, "Applications using Watch Face Format must declare `hasCode=false`")
	}
}

func (r *WatchFaceFormatMissingVersionRule) check(ctx *api.Context) {
	m := ctx.Manifest
	app := firstChild(m.Root, "application")
	if app == nil {
		return
	}
	for _, p := range wffVersionProperties(app) {
		if _, ok := p.Attr("android:value"); !ok {
			emitManifestAtName(ctx, p, "The `android:value` attribute is missing")
		}
	}
	if len(wffVersionProperties(app)) == 0 && wearIsWearProject(m) && wffHasDeclarativeWatchFace(m.Path) {
		emitManifestAtName(ctx, app, "The Watch Face Format version property must be set")
	}
}

func (r *WatchFaceFormatInvalidVersionRule) check(ctx *api.Context) {
	app := firstChild(ctx.Manifest.Root, "application")
	if app == nil {
		return
	}
	for _, p := range wffVersionProperties(app) {
		value, ok := p.Attr("android:value")
		if !ok || strings.Contains(value.Value, "${") {
			continue
		}
		if _, err := strconv.ParseInt(value.Value, 10, 32); err != nil {
			emitManifestAtAttr(ctx, value, "The Watch Face Format version is invalid")
		}
	}
}

// WearPasswordInputRule mirrors Lint's WearPasswordInput (layout part): a password inputType on an EditText of a Wear
// app.
type WearPasswordInputRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a high base confidence: the check reads literal layout attributes.
func (r *WearPasswordInputRule) Confidence() float64 { return api.ConfidenceHigh }

var wearPasswordInputTypes = []string{"numberPassword", "textPassword", "textVisiblePassword", "textWebPassword"}

func (r *WearPasswordInputRule) check(ctx *api.Context) {
	wearByResDir := map[string]bool{}
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		resDir := filepath.Dir(filepath.Dir(layout.FilePath))
		wear, seen := wearByResDir[resDir]
		if !seen {
			wear = wearIsWearResDir(resDir)
			wearByResDir[resDir] = wear
		}
		if !wear {
			continue
		}
		walkViewsUnlessIgnored(layout.RootView, []string{"WearPasswordInput"}, func(v *android.View) {
			if v.Type != "EditText" || !wearIsPasswordInputType(v.Attributes["android:inputType"]) {
				return
			}
			line, col := v.PosOf("android:inputType")
			ctx.Emit(scanner.Finding{File: layout.FilePath, Line: line, Col: col, Message: "Don't ask Wear OS users for a password"})
		})
	}
}

// wearIsPasswordInputType matches the password flags as substrings, as Lint's findAnyOf does.
func wearIsPasswordInputType(value string) bool {
	for _, t := range wearPasswordInputTypes {
		if strings.Contains(value, t) {
			return true
		}
	}
	return false
}
