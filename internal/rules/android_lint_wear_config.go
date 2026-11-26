package rules

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of Android Lint 32.4.1's WearableConfigurationActionDetector (WearableActionDuplicate,
// WearableConfigurationAction), WatchFaceForAndroidXDetector, WearBackNavigationDetector and
// WearMaterialThemeDetector.

const (
	wearConfigurationActionMeta = "com.google.android.wearable.watchface.wearableConfigurationAction"
	wearConfigurationCategory   = "com.google.android.wearable.watchface.category.WEARABLE_CONFIGURATION"
	wearSwipeToDismissItem      = "android:windowSwipeToDismiss"
	wearComposeMaterialTheme    = "androidx.compose.material.MaterialTheme"
)

// wearWatchFaceCoordinates is the dependency both watch face detectors require on the compile classpath.
var wearWatchFaceCoordinates = []librarymodel.Coordinate{{Group: "androidx.wear.watchface", Name: "watchface"}}

// WearableActionDuplicateRule mirrors Lint's WearableActionDuplicate: more than one intent filter for the watch face
// editor action.
type WearableActionDuplicateRule struct {
	ManifestBase
	AndroidRule
}

// WearableConfigurationActionRule mirrors Lint's WearableConfigurationAction: the editor action metadata and the
// activity that handles the editor action must come together.
type WearableConfigurationActionRule struct {
	ManifestBase
	AndroidRule
}

// WatchFaceForAndroidXRule mirrors Lint's WatchFaceForAndroidX: an AndroidX watch face whose configuration action
// metadata is not the watch face editor action.
type WatchFaceForAndroidXRule struct {
	ManifestBase
	AndroidRule
}

// WearBackNavigationRule mirrors Lint's WearBackNavigation: a values item that disables swipe-to-dismiss in a Wear
// module.
type WearBackNavigationRule struct {
	ValuesResourceBase
	AndroidRule
}

// WearMaterialThemeRule mirrors Lint's WearMaterialTheme: importing the phone Compose MaterialTheme in a Wear module.
type WearMaterialThemeRule struct {
	AndroidRule
}

// Confidence reports a high base confidence: the checks read literal manifest elements.
func (r *WearableActionDuplicateRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal manifest elements.
func (r *WearableConfigurationActionRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read literal manifest attributes.
func (r *WatchFaceForAndroidXRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the check reads literal values items.
func (r *WearBackNavigationRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the check compares import text, as Lint does.
func (r *WearMaterialThemeRule) Confidence() float64 { return api.ConfidenceHigh }

// wearConfigScan is what WearableConfigurationActionDetector collects while visiting a manifest: the last editor
// metadata, the first editor action with its configuration category, and whether a second intent filter has the action.
type wearConfigScan struct {
	metaData  *manifest.Node
	action    *manifest.Node
	category  *manifest.Node
	duplicate bool
}

func wearScanConfiguration(root *manifest.Node) wearConfigScan {
	var s wearConfigScan
	manifestWalk(root, func(n, _ *manifest.Node) {
		switch n.Tag {
		case "meta-data":
			if n.AttrValue("android:name") == wearConfigurationActionMeta && n.AttrValue("android:value") == watchFaceEditorAction {
				s.metaData = n
			}
		case "intent-filter":
			var action, category *manifest.Node
			for _, c := range n.Children {
				switch {
				case c.Tag == "action" && c.AttrValue("android:name") == watchFaceEditorAction:
					action = c
				case c.Tag == "category" && c.AttrValue("android:name") == wearConfigurationCategory:
					category = c
				}
			}
			switch {
			case action != nil && s.action != nil:
				s.duplicate = true
			case action != nil:
				s.action, s.category = action, category
			}
		}
	})
	return s
}

// wearUsesWatchFaceLibrary reports whether the project may depend on androidx.wear.watchface:watchface. Lint requires
// it on the compile classpath; krit stays quiet only when the build scripts prove it absent.
func wearUsesWatchFaceLibrary(ctx *api.Context) bool {
	return librarymodel.EnsureFacts(ctx.LibraryFacts).Profile.MayUseAnyDependency(wearWatchFaceCoordinates...)
}

// wearIsWatchFaceFormat mirrors WatchFaceFormatUtils.isWatchFaceFormatProject: the application declares the Watch Face
// Format version property, or a raw resource folder holds a declarative watch face.
func wearIsWatchFaceFormat(m *manifest.Manifest) bool {
	if app := firstChild(m.Root, "application"); app != nil && len(wffVersionProperties(app)) > 0 {
		return true
	}
	return m.Path != "" && wffHasDeclarativeWatchFace(m.Path)
}

func (r *WearableActionDuplicateRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m == nil || m.Root == nil || wearIsWatchFaceFormat(m) || !wearUsesWatchFaceLibrary(ctx) {
		return
	}
	// Lint keeps the last duplicate, but the merged-manifest location it reports maps back to the first <action> with
	// the editor name.
	if s := wearScanConfiguration(m.Root); s.duplicate {
		emitManifestAtNode(ctx, s.action, "Duplicate watch face configuration activities found")
	}
}

func (r *WearableConfigurationActionRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m == nil || m.Root == nil || wearIsWatchFaceFormat(m) || !wearUsesWatchFaceLibrary(ctx) {
		return
	}
	s := wearScanConfiguration(m.Root)
	below30 := manifestMinSdk(m) < 30
	switch {
	case s.metaData != nil && s.action == nil:
		if a, ok := s.metaData.Attr("android:name"); ok {
			emitManifestAtAttr(ctx, a, "Watch face configuration activity is missing")
		}
	case s.metaData != nil && s.category == nil:
		if below30 {
			emitManifestAtNode(ctx, s.action, "Watch face configuration tag is required")
		}
	case s.action != nil && s.metaData == nil:
		if a, ok := s.action.Attr("android:name"); ok && below30 {
			emitManifestAtAttr(ctx, a, "`wearableConfigurationAction` metadata is missing")
		}
	}
}

func (r *WatchFaceForAndroidXRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m == nil || m.Root == nil {
		return
	}
	var bad []*manifest.Node
	manifestWalk(m.Root, func(n, _ *manifest.Node) {
		if n.Tag == "meta-data" && n.AttrValue("android:name") == wearConfigurationActionMeta &&
			n.AttrValue("android:value") != watchFaceEditorAction {
			bad = append(bad, n)
		}
	})
	// Lint 32.4.1 checks the dependency and nothing else: Watch Face Format projects are reported too.
	if len(bad) == 0 || !librarymodel.EnsureFacts(ctx.LibraryFacts).Profile.HasAnyDependency(wearWatchFaceCoordinates...) {
		return
	}
	for _, n := range bad {
		if value, ok := n.Attr("android:value"); ok {
			emitManifestAtValue(ctx, value, "Watch face configuration action must be set to WATCH_FACE_EDITOR for an AndroidX watch face")
		} else {
			emitManifestAtNode(ctx, n, "Watch face configuration action must be set to WATCH_FACE_EDITOR for an AndroidX watch face")
		}
	}
}

var wearSwipeToDismissBytes = []byte("windowSwipeToDismiss")

func (r *WearBackNavigationRule) check(ctx *api.Context) {
	if ctx.ResourceIndex == nil {
		return
	}
	for _, resDir := range ctx.ResourceIndex.ResDirs {
		if !wearIsWearResDir(resDir) {
			continue
		}
		for _, path := range wearValuesFiles(resDir) {
			data, err := os.ReadFile(path)
			if err != nil || !bytes.Contains(data, wearSwipeToDismissBytes) {
				continue
			}
			root, err := android.ParseXMLAST(context.Background(), data)
			if err != nil || root == nil {
				continue
			}
			wearVisitXML(root, func(n *android.XMLNode) {
				if n.Tag == "item" && n.Attr("name") == wearSwipeToDismissItem && strings.TrimSpace(n.Text) == "false" {
					ctx.Emit(scanner.Finding{File: path, Line: n.Line, Col: n.Col,
						Message: "Disabling swipe-to-dismiss is generally not recommended for Wear applications"})
				}
			})
		}
	}
}

// wearValuesFiles lists the XML files of the values folders (values and values-<qualifiers>) of a res directory.
func wearValuesFiles(resDir string) []string {
	folders, err := os.ReadDir(resDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, folder := range folders {
		name := folder.Name()
		if !folder.IsDir() || (name != "values" && !strings.HasPrefix(name, "values-")) {
			continue
		}
		files, err := os.ReadDir(filepath.Join(resDir, name))
		if err != nil {
			continue
		}
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".xml") {
				out = append(out, filepath.Join(resDir, name, f.Name()))
			}
		}
	}
	sort.Strings(out)
	return out
}

func wearVisitXML(n *android.XMLNode, visit func(*android.XMLNode)) {
	visit(n)
	for _, c := range n.Children {
		wearVisitXML(c, visit)
	}
}

var wearComposeMaterialThemeBytes = []byte(wearComposeMaterialTheme)

// check mirrors WearMaterialThemeDetector over every parsed source file of a module whose manifest declares the watch
// feature. Lint compares the import reference text, which leaves out an alias and a trailing `.*`, so
// `import androidx.compose.material.MaterialTheme as T` and `import androidx.compose.material.MaterialTheme.*` count
// and `import androidx.compose.material.*` does not.
func (r *WearMaterialThemeRule) check(ctx *api.Context) {
	var wearRoots []string
	for _, m := range ctx.Manifests {
		if m != nil && wearIsWearProject(m) {
			wearRoots = append(wearRoots, acuiManifestModuleRoot(m.Path))
		}
	}
	if len(wearRoots) == 0 {
		return
	}
	for _, file := range ctx.ParsedFiles {
		if file == nil || file.FlatTree == nil || scanner.IsTestFile(file.Path) ||
			!bytes.Contains(file.Content, wearComposeMaterialThemeBytes) || !acuiUnderAnyRoot(file.Path, wearRoots) {
			continue
		}
		importType, nameType := "import_header", "identifier"
		if file.Language == scanner.LangJava {
			importType, nameType = "import_declaration", "scoped_identifier"
		}
		file.FlatWalkNodes(0, importType, func(n uint32) {
			name, ok := file.FlatFindChild(n, nameType)
			if !ok || file.FlatNodeText(name) != wearComposeMaterialTheme {
				return
			}
			ctx.Emit(scanner.Finding{File: file.Path, Line: file.FlatRow(n) + 1, Col: file.FlatCol(n) + 1,
				StartByte: int(file.FlatStartByte(n)), EndByte: int(file.FlatEndByte(n)),
				Message: "Don't use `" + wearComposeMaterialTheme + "` in a Wear OS project; use " +
					"`androidx.wear.compose.material.MaterialTheme` instead"})
		})
	}
}
