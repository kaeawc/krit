package rules

import (
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/rules/semantics"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

type permissionAPI struct {
	api  string
	perm string
	re   *regexp.Regexp
}

var missingPermissionAPIs = []permissionAPI{
	{api: "requestLocationUpdates", perm: "ACCESS_FINE_LOCATION", re: regexp.MustCompile(`\brequestLocationUpdates\s*\(`)},
	{api: "getLastKnownLocation", perm: "ACCESS_FINE_LOCATION", re: regexp.MustCompile(`\bgetLastKnownLocation\s*\(`)},
	{api: "getCellLocation", perm: "ACCESS_COARSE_LOCATION", re: regexp.MustCompile(`\bgetCellLocation\s*\(`)},
	{api: "Camera.open", perm: "CAMERA", re: regexp.MustCompile(`\bCamera\.open\s*\(`)},
	{api: "setAudioSource", perm: "RECORD_AUDIO", re: regexp.MustCompile(`\bsetAudioSource\s*\(`)},
}

var permissionGuardTokens = []string{
	"checkSelfPermission",
	"ContextCompat.checkSelfPermission",
	"ActivityCompat.checkSelfPermission",
	"PermissionChecker.checkSelfPermission",
	"requestPermissions(",
	"ContextCompat.requestPermissions",
	"ActivityCompat.requestPermissions",
}

type ViewConstructorRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ViewConstructorRule) Confidence() float64 { return 0.75 }

var viewSuperclasses = []string{"View", "ViewGroup", "TextView", "ImageView", "LinearLayout", "RelativeLayout", "FrameLayout", "ConstraintLayout", "RecyclerView", "SurfaceView", "EditText", "Button", "ScrollView", "HorizontalScrollView", "AppBarLayout", "CoordinatorLayout", "CardView", "Toolbar"}

func nodeAtPoint(file *scanner.File, line, column int) uint32 {
	if file == nil || file.FlatTree == nil || line <= 0 || line > len(file.Lines) {
		return 0
	}
	if column < 0 {
		column = 0
	}
	offset := file.LineOffset(line-1) + column
	if offset < 0 {
		offset = 0
	}
	if offset > len(file.Content) {
		offset = len(file.Content)
	}
	idx, ok := file.FlatNamedDescendantForByteRange(uint32(offset), uint32(offset))
	if !ok {
		return 0
	}
	return idx
}

func nodeAtLine(file *scanner.File, line int) uint32 {
	if file == nil || line <= 0 || line > len(file.Lines) {
		return 0
	}
	trimmed := strings.TrimLeft(file.Lines[line-1], " \t")
	column := len(file.Lines[line-1]) - len(trimmed)
	return nodeAtPoint(file, line, column)
}

func enclosingAncestor(file *scanner.File, idx uint32, types ...string) uint32 {
	for cur, ok := idx, idx != 0; ok; cur, ok = file.FlatParent(cur) {
		for _, typ := range types {
			if file.FlatType(cur) == typ {
				return cur
			}
		}
	}
	return 0
}

func prefixTextBeforeLine(file *scanner.File, line int, scopeTypes ...string) (string, bool) {
	scope := enclosingAncestor(file, nodeAtLine(file, line), scopeTypes...)
	if scope == 0 {
		return "", false
	}
	lineOffset := file.LineOffset(line - 1)
	start := int(file.FlatStartByte(scope))
	if lineOffset <= start {
		return "", false
	}
	return string(file.Content[start:lineOffset]), true
}

func declarationHeaderText(file *scanner.File, idx uint32) string {
	if idx == 0 {
		return ""
	}
	end := int(file.FlatEndByte(idx))
	if body, ok := file.FlatFindChild(idx, "class_body"); ok {
		end = int(file.FlatStartByte(body))
	}
	start := int(file.FlatStartByte(idx))
	if end <= start {
		return ""
	}
	return string(file.Content[start:end])
}

func containsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func viewConstructorSupertypeNameFlat(file *scanner.File, deleg uint32) string {
	ut, _ := file.FlatFindChild(deleg, "user_type")
	if ut == 0 {
		if ci, ok := file.FlatFindChild(deleg, "constructor_invocation"); ok {
			ut, _ = file.FlatFindChild(ci, "user_type")
		}
	}
	if ut == 0 {
		return ""
	}
	var lastIdent string
	for child := file.FlatFirstChild(ut); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "type_identifier" {
			lastIdent = file.FlatNodeText(child)
		}
	}
	return lastIdent
}

type ViewTagRule struct {
	FlatDispatchBase
	AndroidRule
}

func (r *ViewTagRule) NodeTypes() []string { return []string{"call_expression"} }

// Confidence reports a high-confidence semantic check. The rule now anchors
// on setTag call expressions, verifies the receiver is a View, and classifies
// the tagged value by resolved type instead of variable names.
func (r *ViewTagRule) Confidence() float64 { return 0.85 }

var viewTagReceiverTypes = []string{
	"android.view.View",
}

var viewTagFrameworkHeavyTypes = []string{
	"android.app.Activity",
	"android.app.Fragment",
	"android.content.Context",
	"android.database.Cursor",
	"android.graphics.Bitmap",
	"android.graphics.drawable.Drawable",
	"android.support.v4.app.Fragment",
	"android.support.v7.widget.RecyclerView.Adapter",
	"android.view.View",
	"android.widget.Adapter",
	"android.widget.ListAdapter",
	"androidx.fragment.app.Fragment",
	"androidx.recyclerview.widget.RecyclerView.Adapter",
}

const viewTagLeakMessage = "View.setTag() with a framework object may cause memory leaks across configuration changes."

func (r *ViewTagRule) check(ctx *v2.Context) {
	if ctx == nil || ctx.File == nil || ctx.Idx == 0 || ctx.File.FlatType(ctx.Idx) != "call_expression" {
		return
	}
	target, ok := semantics.ResolveCallTarget(ctx, ctx.Idx)
	if !ok || target.CalleeName != "setTag" || len(target.Arguments) != 1 {
		return
	}
	if !viewTagReceiverIsView(ctx, target) {
		return
	}
	arg := target.Arguments[0]
	if !arg.Valid() || !viewTagArgumentIsFrameworkHeavy(ctx, arg.Node) {
		return
	}
	ctx.EmitAt(ctx.File.FlatRow(ctx.Idx)+1, ctx.File.FlatCol(ctx.Idx)+1, viewTagLeakMessage)
}

func viewTagReceiverIsView(ctx *v2.Context, target semantics.CallTarget) bool {
	if target.Resolved {
		return viewTagCallTargetIsViewSetTag(target.QualifiedName)
	}
	if !target.Receiver.Valid() {
		return false
	}
	typ, ok := semantics.ExpressionType(ctx, target.Receiver.Node)
	if !ok {
		return false
	}
	return viewTagTypeMatches(ctx, typ.Type, viewTagReceiverTypes)
}

func viewTagArgumentIsFrameworkHeavy(ctx *v2.Context, expr uint32) bool {
	typ, ok := semantics.ExpressionType(ctx, expr)
	if !ok {
		return false
	}
	return viewTagTypeMatches(ctx, typ.Type, viewTagFrameworkHeavyTypes)
}

func viewTagCallTargetIsViewSetTag(target string) bool {
	target = strings.TrimSpace(target)
	if cut := strings.Index(target, "("); cut >= 0 {
		target = target[:cut]
	}
	for _, suffix := range []string{".setTag", "#setTag"} {
		if !strings.HasSuffix(target, suffix) {
			continue
		}
		owner := strings.TrimSuffix(target, suffix)
		return viewTagFQNMatchesAny(owner, viewTagReceiverTypes) ||
			viewTagKnownSubtypeOfAny(owner, viewTagReceiverTypes)
	}
	return false
}

func viewTagTypeMatches(ctx *v2.Context, typ *typeinfer.ResolvedType, targets []string) bool {
	if typ == nil || typ.Kind == typeinfer.TypeUnknown {
		return false
	}
	if viewTagResolvedTypeMatches(typ, targets) {
		return true
	}
	if ctx == nil || ctx.Resolver == nil || ctx.File == nil {
		return false
	}
	if typ.FQN != "" {
		if viewTagClassHierarchyMatches(ctx, typ.FQN, targets, nil) {
			return true
		}
	}
	if typ.Name != "" {
		if fqn := ctx.Resolver.ResolveImport(typ.Name, ctx.File); fqn != "" {
			return viewTagFQNMatchesAny(fqn, targets) ||
				viewTagKnownSubtypeOfAny(fqn, targets) ||
				viewTagClassHierarchyMatches(ctx, fqn, targets, nil)
		}
		if info := ctx.Resolver.ClassHierarchy(typ.Name); info != nil && info.File != "" {
			return viewTagClassInfoMatches(ctx, info, targets, map[string]bool{})
		}
	}
	return false
}

func viewTagResolvedTypeMatches(typ *typeinfer.ResolvedType, targets []string) bool {
	if typ == nil {
		return false
	}
	if viewTagFQNMatchesAny(typ.FQN, targets) || viewTagKnownSubtypeOfAny(typ.FQN, targets) {
		return true
	}
	for _, super := range typ.Supertypes {
		if viewTagFQNMatchesAny(super, targets) || viewTagKnownSubtypeOfAny(super, targets) {
			return true
		}
	}
	return false
}

func viewTagClassHierarchyMatches(ctx *v2.Context, typeName string, targets []string, visited map[string]bool) bool {
	if ctx == nil || ctx.Resolver == nil || typeName == "" {
		return false
	}
	info := ctx.Resolver.ClassHierarchy(typeName)
	if info == nil {
		return false
	}
	if visited == nil {
		visited = map[string]bool{}
	}
	return viewTagClassInfoMatches(ctx, info, targets, visited)
}

func viewTagClassInfoMatches(ctx *v2.Context, info *typeinfer.ClassInfo, targets []string, visited map[string]bool) bool {
	if info == nil {
		return false
	}
	key := info.FQN
	if key == "" {
		key = info.Name
	}
	if key != "" {
		if visited[key] {
			return false
		}
		visited[key] = true
	}
	if viewTagFQNMatchesAny(info.FQN, targets) || viewTagKnownSubtypeOfAny(info.FQN, targets) {
		return true
	}
	for _, super := range info.Supertypes {
		if viewTagFQNMatchesAny(super, targets) || viewTagKnownSubtypeOfAny(super, targets) {
			return true
		}
		if viewTagClassHierarchyMatches(ctx, super, targets, visited) {
			return true
		}
	}
	return false
}

func viewTagFQNMatchesAny(fqn string, targets []string) bool {
	if fqn == "" {
		return false
	}
	for _, target := range targets {
		if fqn == target {
			return true
		}
	}
	return false
}

func viewTagKnownSubtypeOfAny(fqn string, targets []string) bool {
	if fqn == "" {
		return false
	}
	for _, target := range targets {
		if typeinfer.IsKnownSubtype(fqn, target) {
			return true
		}
	}
	return false
}

type WrongImportRule struct {
	LineBase
	AndroidRule
}

var wrongImportRe = regexp.MustCompile(`^import\s+android\.R\b`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *WrongImportRule) Confidence() float64 { return 0.75 }

func (r *WrongImportRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if wrongImportRe.MatchString(strings.TrimSpace(line)) {
			ctx.Emit(r.Finding(file, i+1, 1,
				"Importing android.R instead of the application's R class. This may cause resource resolution errors."))
		}
	}
}

type LayoutInflationRule struct {
	LineBase
	AndroidRule
}

var layoutInflateNullRe = regexp.MustCompile(`\.inflate\s*\(\s*R\.layout\.\w+\s*,\s*null\b`)

// layoutInflationDialogContexts are class suffixes / API markers where
// passing null to inflate() is the correct idiomatic pattern because no
// parent ViewGroup is available at inflation time.
var layoutInflationDialogContexts = []string{
	": Dialog", ": DialogFragment", ": BottomSheetDialogFragment",
	": AppCompatDialog", ": AppCompatDialogFragment", ": AlertDialog",
	": PopupWindow", ": ListPopupWindow",
	"MaterialAlertDialogBuilder", "AlertDialog.Builder",
	"onCreateDialog", "setView(", ".setContentView(",
	"PopupWindow(",
	// Compose interop via AndroidView -- Compose manages attachment.
	"AndroidView(", "AndroidView {",
	// Offscreen rendering to a Bitmap/Canvas -- no parent exists by design.
	"Bitmap.createBitmap", "createBitmap(",
	"Canvas(", "drawToBitmap",
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *LayoutInflationRule) Confidence() float64 { return 0.75 }

func (r *LayoutInflationRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) || !layoutInflateNullRe.MatchString(line) {
			continue
		}
		loc := layoutInflateNullRe.FindStringIndex(line)
		if loc == nil {
			continue
		}
		if enclosingAncestor(file, nodeAtPoint(file, i+1, loc[0]+1), "call_expression") == 0 {
			continue
		}
		if containsAny(line, layoutInflationDialogContexts) {
			continue
		}
		prefix, ok := prefixTextBeforeLine(file, i+1, "function_declaration", "secondary_constructor", "anonymous_initializer", "lambda_literal")
		if ok {
			if containsAny(prefix, layoutInflationDialogContexts) {
				continue
			}
		} else {
			scopeNode := enclosingAncestor(file, nodeAtLine(file, i+1), "class_declaration", "object_declaration")
			if scopeNode == 0 {
				scopeNode = enclosingAncestor(file, nodeAtLine(file, i+1), "source_file")
			}
			if scopeNode == 0 {
				continue
			}
			if containsAny(declarationHeaderText(file, scopeNode), layoutInflationDialogContexts) {
				continue
			}
		}
		ctx.Emit(r.Finding(file, i+1, 1,
			"Avoid passing null as the parent ViewGroup. Inflate with the parent to get correct LayoutParams."))
	}
}

type TrulyRandomRule struct {
	LineBase
	AndroidRule
}

var secureRandomSeedRe = regexp.MustCompile(`SecureRandom\s*\(\s*(byteArrayOf|"[^"]*"|ByteArray)`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *TrulyRandomRule) Confidence() float64 { return 0.75 }

func (r *TrulyRandomRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if !scanner.IsCommentLine(line) && secureRandomSeedRe.MatchString(line) {
			ctx.Emit(r.Finding(file, i+1, 1,
				"SecureRandom with a hardcoded seed is not secure. Use the default constructor for cryptographic randomness."))
		}
	}
}

type MissingPermissionRule struct {
	LineBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *MissingPermissionRule) Confidence() float64 { return 0.75 }

func (r *MissingPermissionRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if scanner.IsCommentLine(line) || strings.HasPrefix(trimmed, "import ") {
			continue
		}
		var match *permissionAPI
		var loc []int
		for idx := range missingPermissionAPIs {
			api := &missingPermissionAPIs[idx]
			if candidate := api.re.FindStringIndex(line); candidate != nil {
				match = api
				loc = candidate
				break
			}
		}
		if match == nil {
			continue
		}
		if enclosingAncestor(file, nodeAtPoint(file, i+1, loc[0]), "call_expression") == 0 {
			continue
		}
		if containsAny(line, permissionGuardTokens) {
			continue
		}
		prefix, ok := prefixTextBeforeLine(file, i+1, "function_declaration", "secondary_constructor", "anonymous_initializer", "lambda_literal")
		if !ok {
			continue
		}
		if containsAny(prefix, permissionGuardTokens) {
			continue
		}
		ctx.Emit(r.Finding(file, i+1, 1,
			match.api+" requires "+match.perm+" permission. Ensure checkSelfPermission() is called before this API."))
	}
}

type WrongConstantRule struct {
	LineBase
	AndroidRule
}

type wrongConstantEntry struct {
	Pattern     *regexp.Regexp
	Description string
}

var wrongConstantEntries = []wrongConstantEntry{
	{regexp.MustCompile(`\.setVisibility\s*\(\s*\d+\s*\)`), "setVisibility() should use View.VISIBLE, View.INVISIBLE, or View.GONE constants."},
	{regexp.MustCompile(`\.setLayoutDirection\s*\(\s*\d+\s*\)`), "setLayoutDirection() should use View.LAYOUT_DIRECTION_LTR or View.LAYOUT_DIRECTION_RTL constants."},
	{regexp.MustCompile(`\.setImportantForAccessibility\s*\(\s*\d+\s*\)`), "setImportantForAccessibility() should use View.IMPORTANT_FOR_ACCESSIBILITY_* constants."},
	{regexp.MustCompile(`\.setGravity\s*\(\s*\d+\s*\)`), "setGravity() should use Gravity.* constants."},
	{regexp.MustCompile(`\.setOrientation\s*\(\s*\d+\s*\)`), "setOrientation() should use LinearLayout.HORIZONTAL or LinearLayout.VERTICAL."},
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *WrongConstantRule) Confidence() float64 { return 0.75 }

func (r *WrongConstantRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) {
			continue
		}
		for _, entry := range wrongConstantEntries {
			if entry.Pattern.MatchString(line) {
				ctx.Emit(r.Finding(file, i+1, 1, entry.Description))
				break
			}
		}
	}
}

type InstantiatableRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *InstantiatableRule) Confidence() float64 { return 0.75 }

var componentSuperclasses = []string{"Activity", "AppCompatActivity", "ComponentActivity", "FragmentActivity", "Service", "IntentService", "BroadcastReceiver", "ContentProvider", "Application"}

type RtlAwareRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *RtlAwareRule) Confidence() float64 { return 0.75 }

var rtlAwareMethods = map[string]string{".getLeft()": ".getStart()", ".getRight()": ".getEnd()", ".getPaddingLeft()": ".getPaddingStart()", ".getPaddingRight()": ".getPaddingEnd()", ".getLeft(": ".getStart(", ".getRight(": ".getEnd(", ".getPaddingLeft(": ".getPaddingStart(", ".getPaddingRight(": ".getPaddingEnd("}

type RtlFieldAccessRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *RtlFieldAccessRule) Confidence() float64 { return 0.75 }

var rtlFieldNames = []string{"mLeft", "mRight", "mPaddingLeft", "mPaddingRight"}

type GridLayoutRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *GridLayoutRule) Confidence() float64 { return 0.75 }

var gridLayoutCreateRe = regexp.MustCompile(`GridLayout\s*\(`)

type LocaleFolderRule struct {
	LineBase
	AndroidRule
}

var localeFolderBadRe = regexp.MustCompile(`values-[a-z]{2}_[A-Z]{2}`)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *LocaleFolderRule) Confidence() float64 { return 0.75 }

func (r *LocaleFolderRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) {
			continue
		}
		if localeFolderBadRe.MatchString(line) {
			ctx.Emit(r.Finding(file, i+1, 1,
				"Wrong locale folder naming `"+localeFolderBadRe.FindString(line)+"`. Use `values-<lang>-r<REGION>` format (e.g., `values-en-rUS`)."))
		}
	}
}

type UseAlpha2Rule struct {
	LineBase
	AndroidRule
}

var alpha3to2 = map[string]string{"eng": "en", "fra": "fr", "deu": "de", "spa": "es", "ita": "it", "por": "pt", "rus": "ru", "jpn": "ja", "kor": "ko", "zho": "zh", "ara": "ar", "hin": "hi", "tur": "tr", "pol": "pl", "nld": "nl", "swe": "sv", "nor": "no", "dan": "da", "fin": "fi", "tha": "th"}
var alpha3FolderRe = regexp.MustCompile(`values-([a-z]{3})\b`)

func (r *UseAlpha2Rule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		if scanner.IsCommentLine(line) {
			continue
		}
		if m := alpha3FolderRe.FindStringSubmatch(line); m != nil {
			if repl, ok := alpha3to2[m[1]]; ok {
				ctx.Emit(r.Finding(file, i+1, 1,
					"Use 2-letter ISO 639-1 code `"+repl+"` instead of 3-letter code `"+m[1]+"` in locale folder."))
			}
		}
	}
}

type MangledCRLFRule struct {
	LineBase
	AndroidRule
}

// Confidence bumps this line rule from the 0.75 line-rule default to
// 0.95 -- mixed line endings is a literal substring check on the
// file bytes. Deterministic with no heuristic path.
func (r *MangledCRLFRule) Confidence() float64 { return 0.95 }

func (r *MangledCRLFRule) check(ctx *v2.Context) {
	file := ctx.File
	content := string(file.Content)
	if strings.Contains(content, "\r\n") && strings.Contains(strings.ReplaceAll(content, "\r\n", ""), "\n") {
		ctx.Emit(r.Finding(file, 1, 1,
			"File has mixed line endings (both CRLF and LF). Use consistent line endings."))
		return
	}
}

type ResourceNameRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ResourceNameRule) Confidence() float64 { return 0.75 }

var resourceRefRe = regexp.MustCompile(`R\.(layout|drawable|string|color|dimen|style|menu|anim|xml|raw|id)\.([a-zA-Z_]\w*)`)
var snakeCaseRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type ProguardRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ProguardRule) Confidence() float64 { return 0.75 }

type ProguardSplitRule struct {
	LineBase
	AndroidRule
}

var (
	proguardGenericRe  = regexp.MustCompile(`-dontobfuscate|-dontoptimize|-dontwarn|-keepattributes`)
	proguardSpecificRe = regexp.MustCompile(`-keep\s+class\s+\w+`)
)

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *ProguardSplitRule) Confidence() float64 { return 0.75 }

func (r *ProguardSplitRule) check(ctx *v2.Context) {
	file := ctx.File
	content := strings.Join(file.Lines, "\n")
	if !strings.Contains(content, "-keep") && !strings.Contains(content, "-dontwarn") {
		return
	}
	if proguardGenericRe.MatchString(content) && proguardSpecificRe.MatchString(content) {
		ctx.Emit(r.Finding(file, 1, 1, "Proguard configuration contains both generic Android rules and project-specific rules. Consider splitting into separate files for maintainability."))
		return
	}
}

type NfcTechWhitespaceRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *NfcTechWhitespaceRule) Confidence() float64 { return 0.75 }

var nfcTechRe = regexp.MustCompile(`<tech>\s+\S+|<tech>\S+\s+</tech>`)

type LibraryCustomViewRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *LibraryCustomViewRule) Confidence() float64 { return 0.75 }

var hardcodedNsRe = regexp.MustCompile(`http://schemas\.android\.com/apk/res/[a-z][a-z0-9_.]+`)

type UnknownIdInLayoutRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence. This is an
// Android-lint port from AOSP; the detection relies on source-text
// patterns (call names, string literal contents, hardcoded allow-
// lists of API names) rather than type resolution, so project-
// specific wrapper APIs can cause false positives or negatives.
// Classified per roadmap/17.
func (r *UnknownIdInLayoutRule) Confidence() float64 { return 0.75 }

var idRefRe = regexp.MustCompile(`R\.id\.([a-zA-Z_]\w*)`)
