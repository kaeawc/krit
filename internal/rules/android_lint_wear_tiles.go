package rules

import (
	"image"
	// Register the decoders for the preview formats javax.imageio reads too.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of Android Lint 32.4.1's TileProviderDetector (SquareAndRoundTilePreviews, TilePreviewImageFormat) and
// TileServiceActivityDetector (StartActivityAndCollapseDeprecated).

const (
	tilePreviewMetaData   = "androidx.wear.tiles.PREVIEW"
	tilePreviewMinSize    = 384
	tileServiceFQN        = "android.service.quicksettings.TileService"
	tileIntentFQN         = "android.content.Intent"
	tileCollapseMethod    = "startActivityAndCollapse"
	tileCollapseMessage   = "TileService#startActivityAndCollapse(Intent) is deprecated. Use TileService#startActivityAndCollapse(PendingIntent) instead."
	tileUpsideDownCakeSdk = 34
)

// SquareAndRoundTilePreviewsRule mirrors Lint 32.4.1's SquareAndRoundTilePreviews: a tile service without a preview,
// or a preview drawable found only in round or only in non-round drawable folders.
type SquareAndRoundTilePreviewsRule struct {
	ManifestBase
	AndroidRule
}

// TilePreviewImageFormatRule mirrors Lint's TilePreviewImageFormat: a tile preview image that is not square or is
// smaller than 384px.
type TilePreviewImageFormatRule struct {
	ManifestBase
	AndroidRule
}

// StartActivityAndCollapseDeprecatedRule mirrors Lint's StartActivityAndCollapseDeprecated: TileService's
// startActivityAndCollapse called with an Intent.
type StartActivityAndCollapseDeprecatedRule struct {
	AndroidRule
}

// Confidence reports a high base confidence: the checks read the manifest and the preview image headers.
func (r *SquareAndRoundTilePreviewsRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the checks read the manifest and the preview image headers.
func (r *TilePreviewImageFormatRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the receiver and the argument must both be proven.
func (r *StartActivityAndCollapseDeprecatedRule) Confidence() float64 { return api.ConfidenceHigh }

// tilePreview is TileProviderDetector's IconInfo for one preview drawable name: where the last metadata naming it is,
// and what the binary drawables with that name looked like.
type tilePreview struct {
	at                manifest.Attr
	round, square     bool
	wrongRatio, small bool
}

// tileServices lists the <service> elements with a BIND_TILE_PROVIDER action, once per such action as the detector
// visits them.
func tileServices(root *manifest.Node) []*manifest.Node {
	var out []*manifest.Node
	manifestWalk(root, func(n, _ *manifest.Node) {
		if n.Tag != "service" {
			return
		}
		for _, filter := range n.ChildrenByTag("intent-filter") {
			for _, action := range filter.ChildrenByTag("action") {
				if action.AttrValue("android:name") == tileProviderBindAction {
					out = append(out, n)
				}
			}
		}
	})
	return out
}

// tilePreviews collects the @drawable/ previews of the tile services, keyed by drawable name, and the services that
// declare no preview metadata at all.
func tilePreviews(m *manifest.Manifest) (map[string]*tilePreview, []string, []*manifest.Node) {
	previews := map[string]*tilePreview{}
	var order []string
	var missing []*manifest.Node
	for _, service := range tileServices(m.Root) {
		found := false
		for _, meta := range service.ChildrenByTag("meta-data") {
			if meta.AttrValue("android:name") != tilePreviewMetaData {
				continue
			}
			found = true
			res, ok := meta.Attr("android:resource")
			if !ok || !strings.HasPrefix(res.Value, "@drawable/") {
				continue
			}
			name := strings.TrimPrefix(res.Value, "@drawable/")
			if previews[name] == nil {
				order = append(order, name)
			}
			previews[name] = &tilePreview{at: res}
		}
		if !found {
			missing = append(missing, service)
		}
	}
	return previews, order, missing
}

// tileScanPreviewImages reads the binary drawables named like a preview in the module's resource folders, as
// checkBinaryResource does in a Wear project.
func tileScanPreviewImages(m *manifest.Manifest, previews map[string]*tilePreview) {
	if len(previews) == 0 || m.Path == "" || !wearIsWearProject(m) {
		return
	}
	for _, resDir := range tileModuleResDirs(m.Path) {
		folders, err := os.ReadDir(resDir)
		if err != nil {
			continue
		}
		for _, folder := range folders {
			if !folder.IsDir() || !tileIsDrawableFolder(folder.Name()) {
				continue
			}
			files, err := os.ReadDir(filepath.Join(resDir, folder.Name()))
			if err != nil {
				continue
			}
			for _, f := range files {
				name := f.Name()
				ext := filepath.Ext(name)
				p := previews[strings.TrimSuffix(name, ext)]
				if p == nil || f.IsDir() || strings.EqualFold(ext, ".xml") {
					continue
				}
				w, h, ok := tileImageSize(filepath.Join(resDir, folder.Name(), name), ext)
				if !ok {
					continue
				}
				if w != h {
					p.wrongRatio = true
				}
				if w < tilePreviewMinSize || h < tilePreviewMinSize {
					p.small = true
				}
				if strings.Contains(folder.Name(), "round") {
					p.round = true
				} else {
					p.square = true
				}
			}
		}
	}
}

// tileIsDrawableFolder reports a drawable resource folder: drawable or drawable-<qualifiers>.
func tileIsDrawableFolder(name string) bool {
	return name == "drawable" || strings.HasPrefix(name, "drawable-")
}

// tileModuleResDirs lists the res directories of every source set of the manifest's module, which make up Lint's
// project resource folders.
func tileModuleResDirs(manifestPath string) []string {
	dir := filepath.Dir(manifestPath)
	if filepath.Base(filepath.Dir(dir)) != "src" {
		return []string{filepath.Join(dir, "res")}
	}
	sets, err := os.ReadDir(filepath.Dir(dir))
	if err != nil {
		return nil
	}
	var out []string
	for _, set := range sets {
		if set.IsDir() {
			out = append(out, filepath.Join(filepath.Dir(dir), set.Name(), "res"))
		}
	}
	sort.Strings(out)
	return out
}

// tileImageSize reads an image header as javax.imageio does for the formats Lint's classpath supports. A WebP file has
// no reader there, so Lint sees 0x0; a header that cannot be read is 0x0 too. Formats whose ImageIO result krit cannot
// reproduce (BMP, TIFF, ...) report false and are skipped.
func tileImageSize(path, ext string) (int, int, bool) {
	switch strings.ToLower(ext) {
	case ".webp":
		return 0, 0, true
	case ".png", ".jpg", ".jpeg", ".gif":
	default:
		return 0, 0, false
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, true
	}
	return cfg.Width, cfg.Height, true
}

func (r *SquareAndRoundTilePreviewsRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m == nil || m.Root == nil {
		return
	}
	previews, order, missing := tilePreviews(m)
	for _, service := range missing {
		emitManifestAtNode(ctx, service, "Tiles need preview assets")
	}
	tileScanPreviewImages(m, previews)
	for _, name := range order {
		if p := previews[name]; p.round != p.square {
			emitManifestAtValue(ctx, p.at, "Tiles need a preview asset in both drawable-round and drawable")
		}
	}
}

func (r *TilePreviewImageFormatRule) check(ctx *api.Context) {
	m := ctx.Manifest
	if m == nil || m.Root == nil {
		return
	}
	previews, order, _ := tilePreviews(m)
	tileScanPreviewImages(m, previews)
	for _, name := range order {
		p := previews[name]
		if p.wrongRatio {
			emitManifestAtValue(ctx, p.at, "Tile previews should have 1:1 aspect ratio")
		}
		if p.small {
			emitManifestAtValue(ctx, p.at, "Tile previews should be at least 384px by 384px")
		}
	}
}

func (r *StartActivityAndCollapseDeprecatedRule) check(ctx *api.Context) {
	file := ctx.File
	// Compare the callee name before building the call view; almost every call is something else.
	name := ""
	if file.FlatType(ctx.Idx) == "method_invocation" {
		name = javaMethodInvocationName(file, ctx.Idx)
	} else {
		name = flatCallExpressionName(file, ctx.Idx)
	}
	if name != tileCollapseMethod {
		return
	}
	call, ok := lintCallAt(file, ctx.Idx)
	if !ok || call.Name != tileCollapseMethod || len(call.Args) != 1 || call.Args[0] == 0 {
		return
	}
	if !tileCallOnTileService(ctx, call) || !tileIsIntentExpr(ctx, call.Args[0], 0) {
		return
	}
	// Lint reports a warning below target SDK 34 (where the call still works) and an error from 34 on.
	severity := ""
	if lintTargetSdk(ctx) < tileUpsideDownCakeSdk {
		severity = "warning"
	}
	ctx.Emit(lintFindingAtNode(ctx, call.Args[0], severity, tileCollapseMessage))
}

// tileCallOnTileService reports whether the call resolves to a method of TileService or one of its subclasses.
func tileCallOnTileService(ctx *api.Context, call lintCall) bool {
	file := ctx.File
	if call.ReceiverNode != 0 && strings.TrimSpace(call.ReceiverText) != "this" {
		if lintReceiverIsType(ctx, call, tileServiceFQN) {
			return true
		}
		typ := lintReceiverTypeText(file, call)
		decl := lintSameFileClass(file, typ)
		return decl != 0 && lintHasSupertype(file, decl, []string{tileServiceFQN}, false, 0)
	}
	// An implicit (or `this`) receiver is the innermost enclosing class that declares the method or extends
	// TileService; object literals, anonymous classes and inner classes defer to the class around them.
	for cur, ok := file.FlatParent(call.Idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration", "object_declaration", "object_literal", "object_creation_expression", "enum_declaration":
			if file.FlatType(cur) == "object_creation_expression" {
				if _, body := file.FlatFindChild(cur, "class_body"); !body {
					continue
				}
			}
			if lintHasSupertype(file, cur, []string{tileServiceFQN}, false, 0) {
				return true
			}
			if tileDeclaresMethod(file, cur, tileCollapseMethod) || !tileDefersToOuterClass(file, cur) {
				return false
			}
		}
	}
	return false
}

// tileDeclaresMethod reports whether the class body declares a function or method called name.
func tileDeclaresMethod(file *scanner.File, cls uint32, name string) bool {
	body, ok := file.FlatFindChild(cls, "class_body")
	if !ok {
		return false
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "function_declaration":
			if file.FlatChildTextOrEmpty(child, "simple_identifier") == name {
				return true
			}
		case "method_declaration":
			if file.FlatChildTextOrEmpty(child, "identifier") == name {
				return true
			}
		}
	}
	return false
}

// tileDefersToOuterClass reports whether an unresolved implicit-receiver call in cls can reach the enclosing class:
// object literals, anonymous classes, Kotlin inner classes and non-static Java member classes.
func tileDefersToOuterClass(file *scanner.File, cls uint32) bool {
	switch file.FlatType(cls) {
	case "object_literal", "object_creation_expression":
		return true
	case "class_declaration":
		mods, ok := file.FlatFindChild(cls, "modifiers")
		if !ok {
			// A Java member class without modifiers is an inner class; a Kotlin one without `inner` is nested.
			return file.Language == scanner.LangJava
		}
		text := " " + strings.Join(strings.Fields(file.FlatNodeText(mods)), " ") + " "
		if file.Language == scanner.LangJava {
			return !strings.Contains(text, " static ")
		}
		return strings.Contains(text, " inner ")
	}
	return false
}

// tileIntentReturningMethods are the android.content.Intent methods that return the Intent itself.
var tileIntentReturningMethods = map[string]bool{
	"addCategory": true, "addFlags": true, "putCharSequenceArrayListExtra": true, "putExtra": true,
	"putExtras": true, "putIntegerArrayListExtra": true, "putParcelableArrayListExtra": true,
	"putStringArrayListExtra": true, "removeCategory": true, "removeFlags": true, "replaceExtras": true,
	"setAction": true, "setClass": true, "setClassName": true, "setComponent": true, "setData": true,
	"setDataAndNormalize": true, "setDataAndType": true, "setDataAndTypeAndNormalize": true, "setFlags": true,
	"setIdentifier": true, "setPackage": true, "setType": true, "setTypeAndNormalize": true,
}

// tileIsIntentExpr reports whether the expression is proven to have type android.content.Intent: an Intent
// constructor call, a variable or parameter declared (or initialized) as one, a resolver-typed expression, or an
// Intent method that returns its receiver.
func tileIsIntentExpr(ctx *api.Context, expr uint32, depth int) bool {
	file := ctx.File
	if expr == 0 || depth > 4 {
		return false
	}
	switch file.FlatType(expr) {
	case "parenthesized_expression":
		return tileIsIntentExpr(ctx, file.FlatNamedChild(expr, 0), depth+1)
	case "simple_identifier", "identifier":
		return lintTypeRefMatches(file, lintDeclaredTypeText(file, expr, file.FlatNodeText(expr)), tileIntentFQN)
	case "call_expression", "method_invocation", "object_creation_expression":
		call, ok := lintCallAt(file, expr)
		if !ok {
			return false
		}
		if file.FlatType(expr) == "object_creation_expression" || (call.ReceiverNode == 0 && call.Name == "Intent") {
			return lintTypeRefMatches(file, call.TypeText, tileIntentFQN)
		}
		if tileIntentReturningMethods[call.Name] && call.ReceiverNode != 0 {
			return tileIsIntentExpr(ctx, call.ReceiverNode, depth+1)
		}
	}
	return false
}
