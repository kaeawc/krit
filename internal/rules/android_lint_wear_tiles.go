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

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
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
// tilePreviewFlags is what the drawable scan found for one preview name.
type tilePreviewFlags struct {
	round, square, wrongRatio, small bool
}

// slotTilePreviewScan memoizes the drawable scan per manifest, which both preview rules need.
const slotTilePreviewScan = "lintTilePreviewScan"

func tileScanPreviewImages(m *manifest.Manifest, previews map[string]*tilePreview) {
	if len(previews) == 0 || m.Path == "" {
		return
	}
	names := make([]string, 0, len(previews))
	for name := range previews {
		names = append(names, name)
	}
	sort.Strings(names)
	scan := filefacts.RequestFact(fileFactsCache(), m.Path+"\x00"+strings.Join(names, ","), slotTilePreviewScan,
		func() map[string]tilePreviewFlags { return tileScanDrawables(m, names) })
	for name, flags := range scan {
		p := previews[name]
		p.round, p.square, p.wrongRatio, p.small = flags.round, flags.square, flags.wrongRatio, flags.small
	}
}

// tileScanDrawables reads the preview images of a Wear module's drawable folders, in every res directory the module
// merges.
func tileScanDrawables(m *manifest.Manifest, names []string) map[string]tilePreviewFlags {
	out := map[string]tilePreviewFlags{}
	if !wearIsWearProject(m) {
		return out
	}
	wanted := map[string]bool{}
	for _, n := range names {
		wanted[n] = true
	}
	for _, resDir := range android.ManifestModuleResDirs(m.Path) {
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
				// Lint 32.4.1 does not match a nine-patch foo.9.png to the preview foo (recorded in nine_patch).
				key := strings.TrimSuffix(name, ext)
				if !wanted[key] || f.IsDir() || strings.EqualFold(ext, ".xml") {
					continue
				}
				p := out[key]
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
				// Lint tests the folder name with contains("round"), so drawable-notround counts as round too.
				if strings.Contains(folder.Name(), "round") {
					p.round = true
				} else {
					p.square = true
				}
				out[key] = p
			}
		}
	}
	return out
}

// tileIsDrawableFolder reports a drawable resource folder: drawable or drawable-<qualifiers>.
func tileIsDrawableFolder(name string) bool {
	return name == "drawable" || strings.HasPrefix(name, "drawable-")
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
	// The file's own module decides, as Lint checks each module with its own target SDK.
	target := android.ResolveSDKLevels(file.Path).TargetSdk
	if target <= 0 {
		target = lintTargetSdk(ctx)
	}
	if target < tileUpsideDownCakeSdk {
		severity = "warning"
	}
	ctx.Emit(lintFindingAtNode(ctx, call.Args[0], severity, tileCollapseMessage))
}

// tileCallOnTileService reports whether the call resolves to a method of TileService or one of its subclasses.
func tileCallOnTileService(ctx *api.Context, call lintCall) bool {
	file := ctx.File
	recv := strings.TrimSpace(call.ReceiverText)
	if call.ReceiverNode == 0 || recv == "this" || recv == "super" || strings.HasPrefix(recv, "this@") ||
		strings.HasSuffix(recv, ".this") {
		// `super.`, a labeled `this@Tile.` and Java's `Outer.this.` name an enclosing class, as an implicit receiver does.
		return tileImplicitReceiverIsTileService(file, call)
	}
	typ := lintTrimGenerics(lintReceiverTypeText(file, call))
	if typ == "TileService" && !lintTypeRefMatches(file, typ, tileServiceFQN) {
		// A bare TileService with no import, star import or same-file class may be a class of the file's package.
		return false
	}
	if lintReceiverIsType(ctx, call, tileServiceFQN) {
		return true
	}
	decl := lintSameFileClass(file, typ)
	return decl != 0 && lintHasSupertype(file, decl, []string{tileServiceFQN}, false, 0)
}

// tileImplicitReceiverIsTileService resolves an implicit receiver: the innermost enclosing scope that can supply the
// method. An extension function decides from its receiver type, a receiver lambda from its receiver, and a class from
// whether it extends TileService; object literals, anonymous classes and inner classes defer to the class around
// them.
func tileImplicitReceiverIsTileService(file *scanner.File, call lintCall) bool {
	unqualified := call.ReceiverNode == 0
	for cur, ok := file.FlatParent(call.Idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "lambda_literal":
			// with(x) { }, x.apply { } and x.run { } give the lambda another implicit receiver; plain run { } does not.
			if recv, ok := tileReceiverLambdaReceiver(file, cur); ok && !tileIsIntentValue(file, recv) {
				return false
			}
		case "function_declaration", "method_declaration":
			// A local function of the same name shadows the member for an unqualified call; `this.` still names it.
			if unqualified && tileDeclaresLocalFunction(file, cur, tileCollapseMethod) {
				return false
			}
			if ext := tileExtensionReceiverType(file, cur); ext != "" {
				if lintTypeRefMatches(file, ext, tileServiceFQN) {
					return true
				}
				decl := lintSameFileClass(file, ext)
				return decl != 0 && lintHasSupertype(file, decl, []string{tileServiceFQN}, false, 0)
			}
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

// tileExtensionReceiverType returns the receiver type of a Kotlin extension function (`fun T.f()`), or "".
func tileExtensionReceiverType(file *scanner.File, fn uint32) string {
	if file.FlatType(fn) != "function_declaration" {
		return ""
	}
	for c := file.FlatFirstChild(fn); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "user_type", "nullable_type":
			if next := file.FlatNextSib(c); next != 0 && file.FlatNodeText(next) == "." {
				return lintTrimGenerics(strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(c)), "?"))
			}
		case "simple_identifier", "function_value_parameters":
			return ""
		}
	}
	return ""
}

// tileIsIntentValue reports an expression proven to be an Intent: a constructor call or a variable declared as one.
// An Intent has no startActivityAndCollapse, so a scope lambda on it leaves the call to the enclosing receiver.
func tileIsIntentValue(file *scanner.File, n uint32) bool {
	if n == 0 {
		return false
	}
	for n != 0 && file.FlatType(n) == "parenthesized_expression" {
		n = file.FlatNamedChild(n, 0)
	}
	switch file.FlatType(n) {
	case "call_expression":
		c, ok := lintCallAt(file, n)
		return ok && c.Name == "Intent" && lintTypeRefMatches(file, c.TypeText, tileIntentFQN)
	case "simple_identifier":
		return lintTypeRefMatches(file, lintDeclaredTypeText(file, n, file.FlatNodeText(n)), tileIntentFQN)
	}
	return false
}

// tileReceiverScopeFunctions are the standard functions whose lambda runs with a new implicit receiver.
var tileReceiverScopeFunctions = map[string]bool{"with": true, "apply": true, "run": true}

// tileReceiverLambdaReceiver reports a lambda passed to with(x), x.apply or x.run, and returns x. A plain run { }
// has no receiver, so it is not one.
func tileReceiverLambdaReceiver(file *scanner.File, lambda uint32) (uint32, bool) {
	for cur, ok := file.FlatParent(lambda); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "annotated_lambda", "call_suffix", "value_argument", "value_arguments":
			continue
		case "call_expression":
			// `with(x) { }` parses as a call whose callee is the call `with(x)`.
			target := cur
			if inner := file.FlatFirstChild(cur); file.FlatType(inner) == "call_expression" {
				target = inner
			}
			c, ok := lintCallAt(file, target)
			if !ok || !tileReceiverScopeFunctions[c.Name] {
				return 0, false
			}
			if c.Name == "with" {
				if len(c.Args) > 0 {
					return c.Args[0], true
				}
				return 0, true
			}
			if c.ReceiverNode == 0 {
				return 0, false
			}
			return c.ReceiverNode, true
		}
		return 0, false
	}
	return 0, false
}

// tileDeclaresLocalFunction reports a function declared directly in fn's body under name.
func tileDeclaresLocalFunction(file *scanner.File, fn uint32, name string) bool {
	found := false
	file.FlatWalkNodes(fn, "function_declaration", func(local uint32) {
		if local != fn && file.FlatChildTextOrEmpty(local, "simple_identifier") == name {
			found = true
		}
	})
	return found
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

// tileNearestDeclaration finds the parameter or earlier local of the enclosing functions that a name refers to, and
// returns its declared type text and initializer. found is false when the name is not declared in a function.
func tileNearestDeclaration(file *scanner.File, ref uint32, name string) (typ string, init uint32, found bool) {
	for fn, ok := flatEnclosingCallable(file, ref); ok; fn, ok = flatEnclosingCallable(file, fn) {
		var local uint32
		for _, t := range []string{"property_declaration", "local_variable_declaration"} {
			file.FlatWalkNodes(fn, t, func(decl uint32) {
				if file.FlatStartByte(decl) >= file.FlatStartByte(ref) {
					return
				}
				// Only a local whose block holds the reference is in scope there.
				if block, ok := file.FlatParent(decl); !ok || file.FlatStartByte(ref) >= file.FlatEndByte(block) {
					return
				}
				for _, v := range []string{"variable_declaration", "variable_declarator"} {
					if vd, ok := file.FlatFindChild(decl, v); ok && lintDeclaredName(file, vd) == name {
						local = decl
					}
				}
			})
		}
		if local != 0 {
			if vd, ok := file.FlatFindChild(local, "variable_declaration"); ok {
				return lintChildTypeText(file, vd), lintInitializerAfterEquals(file, local), true
			}
			vd, _ := file.FlatFindChild(local, "variable_declarator")
			return lintChildTypeText(file, local), file.FlatNamedChild(vd, 1), true
		}
		for _, list := range []string{"function_value_parameters", "formal_parameters"} {
			params, ok := file.FlatFindChild(fn, list)
			if !ok {
				continue
			}
			for p := file.FlatFirstChild(params); p != 0; p = file.FlatNextSib(p) {
				if t := file.FlatType(p); (t == "parameter" || t == "formal_parameter") && lintDeclaredName(file, p) == name {
					return lintChildTypeText(file, p), 0, true
				}
			}
		}
	}
	return "", 0, false
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
	case "this_expression":
		// `Intent().apply { startActivityAndCollapse(this) }`: this is the scope lambda's Intent receiver.
		for cur, ok := file.FlatParent(expr); ok; cur, ok = file.FlatParent(cur) {
			switch file.FlatType(cur) {
			case "lambda_literal":
				if recv, ok := tileReceiverLambdaReceiver(file, cur); ok {
					return tileIsIntentValue(file, recv)
				}
			case "function_declaration", "class_body":
				return false
			}
		}
		return false
	case "simple_identifier", "identifier":
		name := file.FlatNodeText(expr)
		if typ, init, found := tileNearestDeclaration(file, expr, name); found {
			// The nearest declaration decides: an outer one of the same name is shadowed even when this one's type
			// is unknown.
			if typ != "" {
				return lintTypeRefMatches(file, typ, tileIntentFQN)
			}
			return init != 0 && tileIsIntentExpr(ctx, init, depth+1)
		}
		return lintTypeRefMatches(file, lintDeclaredTypeText(file, expr, name), tileIntentFQN)
	case "call_expression", "method_invocation", "object_creation_expression":
		call, ok := lintCallAt(file, expr)
		if !ok {
			return false
		}
		// `Intent()`, `android.content.Intent()` or Java's `new Intent()`; a PendingIntent is another type.
		if file.FlatType(expr) == "object_creation_expression" || call.Name == "Intent" {
			if lintTypeRefMatches(file, call.TypeText, tileIntentFQN) {
				return true
			}
		}
		if tileIntentReturningMethods[call.Name] && call.ReceiverNode != 0 && tileIsIntentExpr(ctx, call.ReceiverNode, depth+1) {
			return true
		}
	}
	// With type information, any expression of type Intent: getIntent(), Intent.createChooser(...).
	if typ, ok := semantics.ExpressionType(ctx, expr); ok && typ.FQN != "" {
		return typ.FQN == tileIntentFQN
	}
	return false
}
