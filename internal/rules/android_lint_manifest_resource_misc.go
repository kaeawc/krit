package rules

import (
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/lazyre"
	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of Android Lint 32.4.1's ScopedStorageDetector, TextViewDetector (SelectableText), SplashScreenDetector
// (CustomSplashScreen), VectorDrawableCompatDetector and TranslucentViewDetector (TranslucentOrientation).

// ScopedStorageRule mirrors Lint's ScopedStorage: external storage permissions that scoped storage changes.
type ScopedStorageRule struct {
	ManifestBase
	AndroidRule
}

// SelectableTextRule mirrors Lint's SelectableText: a TextView whose text is set at runtime and is not selectable.
type SelectableTextRule struct {
	LayoutResourceBase
	AndroidRule
}

// CustomSplashScreenRule mirrors Lint's CustomSplashScreen: an app-defined splash screen activity, fragment or view.
type CustomSplashScreenRule struct {
	AndroidRule
}

// VectorDrawableCompatRule mirrors Lint's VectorDrawableCompat: `android:src` with support library vectors, or
// `app:srcCompat` without them, below minSdk 21.
type VectorDrawableCompatRule struct {
	LayoutResourceBase
	AndroidRule
}

// TranslucentOrientationRule mirrors Lint's TranslucentOrientation: a fixed screenOrientation on an activity whose
// theme is floating.
type TranslucentOrientationRule struct {
	ValuesResourceBase
	AndroidRule
}

// Confidence reports a high base confidence: the checks read manifest attributes and the build's target SDK.
func (r *ScopedStorageRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a medium base confidence: like Lint, the check guesses which text is dynamic.
func (r *SelectableTextRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a medium-high base confidence: the supertype is followed through source and known framework
// classes.
func (r *CustomSplashScreenRule) Confidence() float64 { return api.ConfidenceMediumHigh }

// Confidence reports a high base confidence: the resources and build script are read directly.
func (r *VectorDrawableCompatRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the manifest and styles are read directly.
func (r *TranslucentOrientationRule) Confidence() float64 { return api.ConfidenceHigh }

// ---------------------------------------------------------------------------
// ScopedStorage
// ---------------------------------------------------------------------------

const (
	storageRead   = "android.permission.READ_EXTERNAL_STORAGE"
	storageWrite  = "android.permission.WRITE_EXTERNAL_STORAGE"
	storageManage = "android.permission.MANAGE_EXTERNAL_STORAGE"
)

func (r *ScopedStorageRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m == nil || m.Root == nil {
		return
	}
	target := manifestTargetSdk(m)
	if target <= 0 {
		target = manifestMinSdk(m)
	}
	canManage, legacy := false, false
	manifestWalk(m.Root, func(n, parent *manifest.Node) {
		switch {
		case n.Tag == "application" && parent == m.Root:
			legacy = legacy || n.AttrValue("android:requestLegacyExternalStorage") == "true"
		case n.Tag == "uses-permission" && parent == m.Root && n.AttrValue("tools:node") != "remove":
			canManage = canManage || n.AttrValue("android:name") == storageManage
		}
	})
	manifestWalk(m.Root, func(n, _ *manifest.Node) {
		if n.Tag != "uses-permission" || n.AttrValue("tools:node") == "remove" {
			return
		}
		name, ok := n.Attr("android:name")
		if !ok {
			return
		}
		switch name.Value {
		case storageManage:
			emitManifestAtValue(ctx, name, "The Google Play store has a policy that limits usage of MANAGE_EXTERNAL_STORAGE")
		case storageRead, storageWrite:
			maxSdk := math.MaxInt32
			if v, err := strconv.Atoi(n.AttrValue("android:maxSdkVersion")); err == nil {
				maxSdk = v
			}
			if msg := scopedStorageMessage(min(target, maxSdk), name.Value == storageRead, canManage, legacy); msg != "" {
				emitManifestAtValue(ctx, name, msg)
			}
		}
	})
}

// scopedStorageMessage is filterIncident's verdict for a READ or WRITE permission effective up to sdk, or "".
func scopedStorageMessage(sdk int, read, canManage, legacy bool) string {
	switch {
	case sdk < 29 || canManage, sdk == 29 && legacy:
		return ""
	case sdk >= 33 && read:
		return "READ_EXTERNAL_STORAGE is deprecated (and is not granted) when targeting Android 13+. If you need to query " +
			"or interact with MediaStore or media files on the shared storage, you should instead use one or more new " +
			"storage permissions: `READ_MEDIA_IMAGES`, `READ_MEDIA_VIDEO` or `READ_MEDIA_AUDIO`."
	case sdk >= 33:
		return "WRITE_EXTERNAL_STORAGE is deprecated (and is not granted) when targeting Android 13+. If you need to " +
			"write to shared storage, use the `MediaStore.createWriteRequest` intent."
	case read:
		return ""
	case legacy:
		return "WRITE_EXTERNAL_STORAGE no longer provides write access when targeting Android 11+, even when using `requestLegacyExternalStorage`"
	case sdk == 29:
		return "WRITE_EXTERNAL_STORAGE no longer provides write access when targeting Android 10, unless you use `requestLegacyExternalStorage`"
	}
	return "WRITE_EXTERNAL_STORAGE no longer provides write access when targeting Android 10+"
}

// ---------------------------------------------------------------------------
// SelectableText
// ---------------------------------------------------------------------------

func (r *SelectableTextRule) check(ctx *api.Context) {
	if ctx.ResourceIndex == nil {
		return
	}
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		walkViews(layout.RootView, func(v *android.View) {
			if v.Type != "TextView" {
				return
			}
			attrs := v.Attributes
			for _, missing := range []string{"android:text", "android:textIsSelectable", "android:visibility", "android:onClick"} {
				if _, ok := attrs[missing]; ok {
					return
				}
			}
			if _, ok := attrs["android:id"]; !ok {
				return
			}
			ctx.Emit(scanner.Finding{File: layout.FilePath, Line: v.Line, Col: v.Col + 1,
				Message: "Consider making the text value selectable by specifying `android:textIsSelectable=\"true\"`"})
		})
	}
}

// ---------------------------------------------------------------------------
// CustomSplashScreen
// ---------------------------------------------------------------------------

var splashScreenKeywords = []string{"splashscreen", "splashactivity", "launchactivity", "launchscreen"}

// splashScreenSupertypes are the classes Lint's check extends from, each with the framework and library subclasses a
// declaration may name instead.
var splashScreenSupertypes = []struct {
	fqn   string
	known map[string]struct{}
}{
	{"android.app.Activity", lintKnownSet("Activity", "AppCompatActivity", "ComponentActivity", "FragmentActivity",
		"ListActivity", "PreferenceActivity", "ExpandableListActivity", "TabActivity", "AccountAuthenticatorActivity",
		"LauncherActivity", "AliasActivity", "NativeActivity")},
	{"androidx.fragment.app.Fragment", lintKnownSet("DialogFragment", "ListFragment", "AppCompatDialogFragment",
		"PreferenceFragmentCompat", "BottomSheetDialogFragment")},
	{"android.support.v4.app.Fragment", nil},
	{"android.view.View", lintKnownSet(append(append([]string{}, viewSuperclasses...), "ImageButton", "Button",
		"CompoundButton", "CheckBox", "ProgressBar", "WebView", "VideoView", "TextureView")...)},
}

func lintKnownSet(names ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(names))
	for _, n := range names {
		out[n] = struct{}{}
	}
	return out
}

func (r *CustomSplashScreenRule) check(ctx *api.Context) {
	file := ctx.File
	cls := ctx.Idx
	nameNode := lintClassNameNode(file, cls)
	if nameNode == 0 {
		return
	}
	lower := strings.ToLower(file.FlatNodeText(nameNode))
	matched := false
	for _, k := range splashScreenKeywords {
		if strings.Contains(lower, k) {
			matched = true
			break
		}
	}
	if !matched || lintTargetSdk(ctx) < 31 {
		return
	}
	for _, super := range splashScreenSupertypes {
		if lintClassExtends(ctx, cls, super.fqn, super.known) {
			ctx.Emit(lintFindingAtNode(ctx, nameNode, "", "The application should not provide its own launch screen"))
			return
		}
	}
}

// ---------------------------------------------------------------------------
// VectorDrawableCompat
// ---------------------------------------------------------------------------

// moduleVectorSettings is what VectorDrawableCompatDetector reads from the build: the minSdk, whether
// vectorDrawables.useSupportLibrary is on, and the module's Gradle path for the message.
type moduleVectorSettings struct {
	known         bool
	minSdk        int
	useSupportLib bool
	buildFile     string
}

var (
	vectorSupportLibraryRe  = lazyre.New(`useSupportLibrary\s*=?\s*(true|false)`)
	vectorRootElementNeedle = []byte("vector")
)

// moduleVectorSettingsForRes reads the settings of the module that owns a res directory. Without a build script
// the check is skipped, as Lint skips it without a build variant.
func moduleVectorSettingsForRes(resDir string) moduleVectorSettings {
	module := resModuleDir(resDir)
	return filefacts.RequestFact(fileFactsCache(), module, slotModuleVectorSettings, func() moduleVectorSettings {
		return readModuleVectorSettings(module)
	})
}

// slotModuleVectorSettings memoizes moduleVectorSettingsForRes per module directory for one request.
const slotModuleVectorSettings = "lintModuleVectorSettings"

func readModuleVectorSettings(module string) moduleVectorSettings {
	var s moduleVectorSettings
	for _, name := range manifestBuildScriptNames {
		data, err := os.ReadFile(filepath.Join(module, name))
		if err != nil {
			continue
		}
		s.known = true
		if info, ok := tryReadGradleInfo(module); ok {
			s.minSdk = info.minSdk
		}
		// A commented-out useSupportLibrary setting does not count.
		if m := vectorSupportLibraryRe.FindStringSubmatch(gradleWithoutComments(string(data))); m != nil {
			s.useSupportLib = m[1] == "true"
		}
		s.buildFile = gradleModulePath(module) + "build.gradle"
		break
	}
	return s
}

// vectorLayoutNamespaces maps the namespace prefixes a layout declares (xmlns:app=...) to their URIs.
func vectorLayoutNamespaces(root *android.View) map[string]string {
	out := map[string]string{}
	walkViews(root, func(v *android.View) {
		for name, value := range v.Attributes {
			if prefix, ok := strings.CutPrefix(name, "xmlns:"); ok {
				out[prefix] = value
			}
		}
	})
	return out
}

// gradleModulePath returns the module's Gradle path with a trailing separator (":app/") when a settings script above
// it includes it, or "" for a module krit cannot place in a build.
func gradleModulePath(module string) string {
	candidates := gradleSettingsCandidates(module)
	if len(candidates) == 0 {
		return ""
	}
	settings := candidates[len(candidates)-1]
	if _, err := os.Stat(settings); err != nil {
		return ""
	}
	rel, err := filepath.Rel(filepath.Dir(settings), module)
	if err != nil {
		return ""
	}
	return ":" + strings.ReplaceAll(filepath.ToSlash(rel), "/", ":") + string(filepath.Separator)
}

// gradleSettingsCandidates returns the settings scripts gradleModulePath looks for above module, nearest first, up to
// and including the first that exists; the list holds none that exists when there is no settings script above it.
func gradleSettingsCandidates(module string) []string {
	var candidates []string
	for dir := filepath.Dir(module); ; dir = filepath.Dir(dir) {
		for _, name := range []string{"settings.gradle.kts", "settings.gradle"} {
			path := filepath.Join(dir, name)
			candidates = append(candidates, path)
			if _, err := os.Stat(path); err == nil {
				return candidates
			}
		}
		if parent := filepath.Dir(dir); parent == dir {
			return candidates
		}
	}
}

func (r *VectorDrawableCompatRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	if idx == nil {
		return
	}
	// Lint only checks modules below minSdk 21 with a build model; skip the drawable scan when there are none.
	var layouts []*android.Layout
	for _, layout := range idx.AllLayouts() {
		if settings := moduleVectorSettingsForRes(filepath.Dir(filepath.Dir(layout.FilePath))); settings.known && settings.minSdk > 0 && settings.minSdk < 21 {
			layouts = append(layouts, layout)
		}
	}
	if len(layouts) == 0 {
		return
	}
	// The vector drawables of each res directory, keyed by resource name.
	vectors := map[string]bool{}
	for _, doc := range resFolderFiles(idx, "drawable", vectorRootElementNeedle) {
		if doc.root.Tag == "vector" || doc.root.Tag == "animated-vector" {
			vectors[strings.TrimSuffix(filepath.Base(doc.path), ".xml")] = true
		}
	}
	if len(vectors) == 0 {
		return
	}
	for _, layout := range layouts {
		settings := moduleVectorSettingsForRes(filepath.Dir(filepath.Dir(layout.FilePath)))
		namespaces := vectorLayoutNamespaces(layout.RootView)
		walkViews(layout.RootView, func(v *android.View) {
			for name, value := range v.Attributes {
				var message string
				switch {
				case name == "android:src" && settings.useSupportLib:
					message = "When using VectorDrawableCompat, you need to use `app:srcCompat`"
				case strings.HasSuffix(name, ":srcCompat") && !settings.useSupportLib &&
					namespaces[strings.TrimSuffix(name, ":srcCompat")] == resAutoURI:
					message = "To use VectorDrawableCompat, you need to set " +
						"`android.defaultConfig.vectorDrawables.useSupportLibrary = true` in `" + settings.buildFile + "`"
				default:
					continue
				}
				// Like Lint, a reference matches a local vector by name alone (even @android:drawable/<name>).
				url, ok := parseResourceURL(value)
				if !ok || !vectors[url.name] {
					continue
				}
				line, col := v.PosOf(name)
				ctx.Emit(scanner.Finding{File: layout.FilePath, Line: line, Col: col, Message: message})
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TranslucentOrientation
// ---------------------------------------------------------------------------

var styleNeedle = []byte("<style")

func (r *TranslucentOrientationRule) check(ctx *api.Context) {
	idx := ctx.ResourceIndex
	if idx == nil {
		return
	}
	for _, resDir := range idx.ResDirs {
		themes, defaultTheme, ok := translucentInterestingThemes(resDir)
		if !ok {
			continue
		}
		for _, doc := range resFolderFiles(&android.ResourceIndex{ResDirs: []string{resDir}}, "values", styleNeedle) {
			for _, style := range doc.root.Children {
				if style.Tag != "style" {
					continue
				}
				name := style.Attr("name")
				if (defaultTheme == "" || name != defaultTheme) && !themes[name] {
					continue
				}
				for _, item := range style.Children {
					if item.Tag != "item" {
						continue
					}
					// Lint matches the un-prefixed windowIsTranslucent, not android:windowIsTranslucent.
					if attr := item.Attr("name"); attr == "android:windowIsFloating" || attr == "windowIsTranslucent" {
						for _, a := range item.Attrs {
							if a.Name == "name" {
								ctx.Emit(doc.finding(doc.valueOffset(a), "Should not specify screen orientation with translucent or floating theme"))
							}
						}
						break
					}
				}
			}
		}
	}
}

// translucentInterestingThemes reads the module manifest for a res directory: the themes of activities with a fixed
// screenOrientation, and the application theme when such an activity has none. It reports false when the module's
// target SDK is below 26 or it has no manifest.
func translucentInterestingThemes(resDir string) (map[string]bool, string, bool) {
	path := translucentManifestPath(resDir)
	root := manifestRoot(&Manifest{Path: path})
	if root == nil {
		return nil, "", false
	}
	themes := map[string]bool{}
	defaultTheme := ""
	var visit func(n, parent *android.XMLNode)
	visit = func(n, parent *android.XMLNode) {
		orientation := strings.ToLower(n.Attr("android:screenOrientation"))
		if (strings.HasSuffix(orientation, "portrait") || strings.HasSuffix(orientation, "landscape")) && xmlHasAttr(n, "android:name") {
			if theme := strings.TrimSpace(n.Attr("android:theme")); theme != "" {
				if name := translucentThemeName(theme); name != "" {
					themes[name] = true
				}
			} else {
				defaultTheme = ""
				if parent != nil {
					defaultTheme = translucentThemeName(parent.Attr("android:theme"))
				}
			}
		}
		for _, c := range n.Children {
			visit(c, n)
		}
	}
	visit(root, nil)
	// The target SDK reads the build script, so only resolve it when an activity qualifies.
	if (len(themes) == 0 && defaultTheme == "") || translucentTargetSdk(path, root) < 26 {
		return nil, "", false
	}
	return themes, defaultTheme, len(themes) > 0 || defaultTheme != ""
}

// translucentManifestPath is the module manifest TranslucentOrientation reads for a res directory: its source set's,
// or the main one for a res directory of another source set.
func translucentManifestPath(resDir string) string {
	sourceSet := filepath.Dir(resDir)
	if filepath.Base(filepath.Dir(sourceSet)) == "src" && filepath.Base(sourceSet) != "main" {
		return filepath.Join(filepath.Dir(sourceSet), "main", "AndroidManifest.xml")
	}
	return filepath.Join(sourceSet, "AndroidManifest.xml")
}

// translucentTargetSdk resolves the module's target SDK as Lint does: the build script, else <uses-sdk>, else the
// minimum SDK.
func translucentTargetSdk(path string, root *android.XMLNode) int {
	info := lookupManifestGradleInfo(path)
	if info.targetSdk > 0 {
		return info.targetSdk
	}
	for _, c := range root.ChildrenByTag("uses-sdk") {
		if v, err := strconv.Atoi(c.Attr("android:targetSdkVersion")); err == nil {
			return v
		}
		if v, err := strconv.Atoi(c.Attr("android:minSdkVersion")); err == nil {
			return v
		}
	}
	if info.minSdk > 0 {
		return info.minSdk
	}
	return 1
}

func xmlHasAttr(n *android.XMLNode, name string) bool {
	for _, a := range n.Attrs {
		if a.Name == name {
			return true
		}
	}
	return false
}

// translucentThemeName returns the style name of a non-framework `@style/` reference, or "".
func translucentThemeName(url string) string {
	if !strings.HasPrefix(url, "@style/") {
		return ""
	}
	return strings.TrimPrefix(url, "@style/")
}
