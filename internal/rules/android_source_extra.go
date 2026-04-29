package rules

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules/semantics"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

type permissionAPI struct {
	api             string
	callee          string
	perm            string
	callTargets     []string
	receiverTypes   []string
	staticReceivers []string
}

var missingPermissionAPIs = []permissionAPI{
	{
		api:           "requestLocationUpdates",
		callee:        "requestLocationUpdates",
		perm:          "ACCESS_FINE_LOCATION",
		callTargets:   []string{"android.location.LocationManager.requestLocationUpdates"},
		receiverTypes: []string{"android.location.LocationManager"},
	},
	{
		api:           "getLastKnownLocation",
		callee:        "getLastKnownLocation",
		perm:          "ACCESS_FINE_LOCATION",
		callTargets:   []string{"android.location.LocationManager.getLastKnownLocation"},
		receiverTypes: []string{"android.location.LocationManager"},
	},
	{
		api:           "getCellLocation",
		callee:        "getCellLocation",
		perm:          "ACCESS_COARSE_LOCATION",
		callTargets:   []string{"android.telephony.TelephonyManager.getCellLocation"},
		receiverTypes: []string{"android.telephony.TelephonyManager"},
	},
	{
		api:             "Camera.open",
		callee:          "open",
		perm:            "CAMERA",
		callTargets:     []string{"android.hardware.Camera.open"},
		staticReceivers: []string{"android.hardware.Camera"},
	},
	{
		api:           "setAudioSource",
		callee:        "setAudioSource",
		perm:          "RECORD_AUDIO",
		callTargets:   []string{"android.media.MediaRecorder.setAudioSource"},
		receiverTypes: []string{"android.media.MediaRecorder"},
	},
}

var missingPermissionAnnotatedIdentifiers = []string{"RequiresPermission"}

var missingPermissionCandidateCallees = missingPermissionCandidateCalleeSet()

func missingPermissionCandidateCalleeSet() map[string]bool {
	out := make(map[string]bool, len(missingPermissionAPIs))
	for _, api := range missingPermissionAPIs {
		if api.callee != "" {
			out[api.callee] = true
		}
	}
	return out
}

func missingPermissionCandidateCalleeNames() []string {
	out := make([]string, 0, len(missingPermissionCandidateCallees))
	for name := range missingPermissionCandidateCallees {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func missingPermissionOracleIdentifiers() []string {
	out := append([]string(nil), missingPermissionCandidateCalleeNames()...)
	out = append(out, missingPermissionAnnotatedIdentifiers...)
	sort.Strings(out)
	return out
}

var missingPermissionKnownPerms = map[string]bool{
	"ACCESS_COARSE_LOCATION": true,
	"ACCESS_FINE_LOCATION":   true,
	"CAMERA":                 true,
	"RECORD_AUDIO":           true,
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
	confidence, ok := viewTagReceiverEvidence(ctx, target)
	if !ok {
		return
	}
	arg := target.Arguments[0]
	if !arg.Valid() || !viewTagArgumentIsFrameworkHeavy(ctx, arg.Node) {
		return
	}
	ctx.Emit(scanner.Finding{
		Line:       ctx.File.FlatRow(ctx.Idx) + 1,
		Col:        ctx.File.FlatCol(ctx.Idx) + 1,
		Message:    viewTagLeakMessage,
		Confidence: confidence,
	})
}

func viewTagReceiverEvidence(ctx *v2.Context, target semantics.CallTarget) (float64, bool) {
	if target.Resolved {
		if viewTagCallTargetIsViewSetTag(target.QualifiedName) {
			return 0.85, true
		}
		return 0, false
	}
	if !target.Receiver.Valid() {
		return 0, false
	}
	typ, ok := semantics.ExpressionType(ctx, target.Receiver.Node)
	if !ok {
		return 0, false
	}
	if viewTagFrameworkTypeMatches(ctx, typ.Type, viewTagReceiverTypes) {
		return 0.85, true
	}
	if viewTagSameFileClassTypeMatches(ctx, typ.Type, viewTagReceiverTypes) {
		return 0.80, true
	}
	return 0, false
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

func viewTagFrameworkTypeMatches(ctx *v2.Context, typ *typeinfer.ResolvedType, targets []string) bool {
	if typ == nil || typ.Kind == typeinfer.TypeUnknown {
		return false
	}
	if viewTagFQNMatchesAny(typ.FQN, targets) || viewTagKnownSubtypeOfAny(typ.FQN, targets) {
		return true
	}
	if ctx == nil || ctx.Resolver == nil || ctx.File == nil || typ.Name == "" {
		return false
	}
	if fqn := ctx.Resolver.ResolveImport(typ.Name, ctx.File); fqn != "" {
		return viewTagFQNMatchesAny(fqn, targets) || viewTagKnownSubtypeOfAny(fqn, targets)
	}
	return false
}

func viewTagSameFileClassTypeMatches(ctx *v2.Context, typ *typeinfer.ResolvedType, targets []string) bool {
	if ctx == nil || ctx.Resolver == nil || ctx.File == nil || typ == nil || typ.Kind == typeinfer.TypeUnknown {
		return false
	}
	for _, candidate := range []string{typ.FQN, typ.Name} {
		if candidate == "" {
			continue
		}
		info := ctx.Resolver.ClassHierarchy(candidate)
		if info == nil || info.File != ctx.File.Path {
			continue
		}
		if viewTagClassInfoMatches(ctx, info, targets, map[string]bool{}) {
			return true
		}
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
	FlatDispatchBase
	AndroidRule
}

// Confidence: tier-1 for this rule because the detection is purely
// syntactic — an import_header's `identifier` child carries the exact
// dotted FQN. No symbol resolution is needed and no source-text
// heuristics are involved.
func (r *WrongImportRule) Confidence() float64 { return 0.95 }

// check runs per import_header node. It flags `import android.R` and
// any `import android.R.<anything>` (e.g. `android.R.layout`) because
// these refer to the framework R class rather than the application's
// generated R. Aliased imports (`import android.R as FR`) are flagged
// all the same — the alias is syntactic sugar; the underlying import
// is still wrong.
func (r *WrongImportRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	ident, ok := file.FlatFindChild(idx, "identifier")
	if !ok {
		return
	}
	fqn := file.FlatNodeText(ident)
	if fqn != "android.R" && !strings.HasPrefix(fqn, "android.R.") {
		return
	}
	ctx.EmitAt(file.FlatRow(idx)+1, 1,
		"Importing android.R instead of the application's R class. This may cause resource resolution errors.")
}

type LayoutInflationRule struct {
	FlatDispatchBase
	LayoutResourceBase
	AndroidRule
}

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

func (r *LayoutInflationRule) NodeTypes() []string { return []string{"call_expression"} }

// Confidence reports a high-confidence AST/resource-backed check. The
// remaining uncertainty is limited to intentional no-parent contexts that do
// not have a precise type signal in tree-sitter-only analysis.
func (r *LayoutInflationRule) Confidence() float64 { return 0.85 }

func (r *LayoutInflationRule) check(ctx *v2.Context) {
	if ctx == nil || ctx.File == nil || ctx.Idx == 0 {
		return
	}
	file := ctx.File
	if flatCallExpressionName(file, ctx.Idx) != "inflate" {
		return
	}
	layoutName, ok := layoutInflationLayoutName(file, ctx.Idx)
	if !ok || !layoutInflationSecondArgumentIsNull(file, ctx.Idx) {
		return
	}
	if layoutInflationHasIntentionalNullParentContext(file, ctx.Idx) {
		return
	}
	if !layoutInflationRootHasLayoutParams(ctx.ResourceIndex, layoutName) &&
		!layoutInflationCallerHasNonNullViewGroup(file, ctx.Idx) {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"Avoid passing null as the parent ViewGroup. Inflate with the parent to get correct LayoutParams."))
}

// layoutInflationCallerHasNonNullViewGroup reports whether the enclosing
// function (or any outer function for nested declarations) declares a
// non-nullable ViewGroup parameter, indicating a parent is in scope and
// should have been passed to inflate().
func layoutInflationCallerHasNonNullViewGroup(file *scanner.File, call uint32) bool {
	if file == nil || call == 0 {
		return false
	}
	for fn, ok := flatEnclosingFunction(file, call); ok; fn, ok = flatEnclosingFunction(file, fn) {
		params, _ := file.FlatFindChild(fn, "function_value_parameters")
		if params == 0 {
			continue
		}
		for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) != "parameter" {
				continue
			}
			if layoutInflationParameterIsNonNullViewGroup(file, child) {
				return true
			}
		}
	}
	return false
}

func layoutInflationParameterIsNonNullViewGroup(file *scanner.File, param uint32) bool {
	for child := file.FlatFirstChild(param); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "user_type":
			ident := flatLastChildOfType(file, child, "type_identifier")
			if ident == 0 {
				return false
			}
			return layoutInflationTypeNameIsViewGroup(file.FlatNodeText(ident))
		case "nullable_type":
			return false
		}
	}
	return false
}

func layoutInflationTypeNameIsViewGroup(name string) bool {
	name = strings.TrimSpace(name)
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}
	if idx := strings.Index(name, "<"); idx >= 0 {
		name = name[:idx]
	}
	return name == "ViewGroup"
}

func layoutInflationLayoutName(file *scanner.File, call uint32) (string, bool) {
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return "", false
	}
	arg := flatNamedValueArgument(file, args, "resource")
	if arg == 0 {
		arg = flatPositionalValueArgument(file, args, 0)
	}
	expr := flatValueArgumentExpression(file, arg)
	if expr == 0 {
		return "", false
	}
	ids := layoutInflationIdentifierChain(file, flatUnwrapParenExpr(file, expr))
	if len(ids) != 3 || ids[0] != "R" || ids[1] != "layout" || ids[2] == "" {
		return "", false
	}
	return ids[2], true
}

func layoutInflationSecondArgumentIsNull(file *scanner.File, call uint32) bool {
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return false
	}
	arg := flatNamedValueArgument(file, args, "root")
	if arg == 0 {
		arg = flatNamedValueArgument(file, args, "parent")
	}
	if arg == 0 {
		arg = flatPositionalValueArgument(file, args, 1)
	}
	expr := flatValueArgumentExpression(file, arg)
	if expr == 0 {
		return strings.TrimSpace(file.FlatNodeText(arg)) == "null"
	}
	expr = flatUnwrapParenExpr(file, expr)
	return expr != 0 && strings.TrimSpace(file.FlatNodeText(expr)) == "null"
}

func layoutInflationIdentifierChain(file *scanner.File, idx uint32) []string {
	if file == nil || idx == 0 {
		return nil
	}
	switch file.FlatType(idx) {
	case "simple_identifier":
		return []string{file.FlatNodeText(idx)}
	case "navigation_expression":
		var ids []string
		var walk func(uint32)
		walk = func(node uint32) {
			for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
				if !file.FlatIsNamed(child) {
					continue
				}
				switch file.FlatType(child) {
				case "simple_identifier":
					ids = append(ids, file.FlatNodeText(child))
				case "navigation_expression", "navigation_suffix":
					walk(child)
				}
			}
		}
		walk(idx)
		return ids
	default:
		return nil
	}
}

func layoutInflationRootHasLayoutParams(idx *android.ResourceIndex, layoutName string) bool {
	if idx == nil || layoutName == "" {
		return false
	}
	if configs := idx.LayoutConfigs[layoutName]; len(configs) > 0 {
		for _, layout := range configs {
			if layoutInflationViewHasLayoutParams(layoutRootView(layout)) {
				return true
			}
		}
		return false
	}
	return layoutInflationViewHasLayoutParams(layoutRootView(idx.Layouts[layoutName]))
}

func layoutRootView(layout *android.Layout) *android.View {
	if layout == nil {
		return nil
	}
	return layout.RootView
}

func layoutInflationViewHasLayoutParams(root *android.View) bool {
	if root == nil {
		return false
	}
	for attr := range root.Attributes {
		prefix, local := "", attr
		if idx := strings.LastIndex(attr, ":"); idx >= 0 {
			prefix = attr[:idx]
			local = attr[idx+1:]
		}
		if prefix == "tools" {
			continue
		}
		if strings.HasPrefix(local, "layout_") {
			return true
		}
	}
	return false
}

func layoutInflationHasIntentionalNullParentContext(file *scanner.File, idx uint32) bool {
	for cur, ok := idx, idx != 0; ok; cur, ok = file.FlatParent(cur) {
		if file.FlatType(cur) != "call_expression" {
			continue
		}
		switch flatCallNameAny(file, cur) {
		case "AndroidView", "PopupWindow", "setView", "setContentView", "createBitmap", "drawToBitmap":
			return true
		}
	}
	for _, scopeType := range []string{"function_declaration", "class_declaration", "object_declaration"} {
		if scope := enclosingAncestor(file, idx, scopeType); scope != 0 && containsAny(declarationHeaderText(file, scope), layoutInflationDialogContexts) {
			return true
		}
	}
	line := file.FlatRow(idx) + 1
	if prefix, ok := prefixTextBeforeLine(file, line, "function_declaration", "secondary_constructor", "anonymous_initializer", "lambda_literal"); ok && containsAny(prefix, layoutInflationDialogContexts) {
		return true
	}
	if fn := enclosingAncestor(file, idx, "function_declaration", "secondary_constructor", "anonymous_initializer", "lambda_literal"); fn != 0 && containsAny(file.FlatNodeText(fn), layoutInflationDialogContexts) {
		return true
	}
	return false
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
	FlatDispatchBase
	AndroidRule
}

func (r *MissingPermissionRule) NodeTypes() []string { return []string{"call_expression"} }

// Confidence reports a tier-2 (medium) base confidence. The rule requires a
// structural or resolved Android API anchor and a same-permission guard proof,
// but can still run with source-level type evidence when KAA call targets are
// unavailable.
func (r *MissingPermissionRule) Confidence() float64 { return 0.75 }

func (r *MissingPermissionRule) check(ctx *v2.Context) {
	if ctx == nil || ctx.File == nil || ctx.Idx == 0 {
		return
	}
	file := ctx.File
	if first := file.FlatChild(ctx.Idx, 0); first != 0 && file.FlatType(first) == "call_expression" {
		return
	}
	callee := flatCallExpressionName(file, ctx.Idx)
	if !missingPermissionCandidateCallees[callee] && !missingPermissionHasAnnotatedSameFileCandidate(ctx, ctx.Idx, callee) {
		return
	}
	match, evidence, ok := missingPermissionRequiredAPI(ctx, ctx.Idx)
	if !ok {
		return
	}
	confidence, ok := semantics.ConfidenceForEvidence(r.Confidence(), evidence)
	if !ok {
		return
	}
	if missingPermissionHasStructuralGuard(ctx, ctx.Idx, match.perm) {
		return
	}
	line := file.FlatRow(ctx.Idx) + 1
	col := file.FlatCol(ctx.Idx) + 1
	f := r.Finding(file, line, col,
		match.api+" requires "+match.perm+" permission. Ensure checkSelfPermission() is called before this API.")
	f.Confidence = confidence
	ctx.Emit(f)
}

func missingPermissionRequiredAPI(ctx *v2.Context, call uint32) (permissionAPI, semantics.SemanticEvidence, bool) {
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok {
		return permissionAPI{}, semantics.EvidenceUnresolved, false
	}
	for i := range missingPermissionAPIs {
		api := missingPermissionAPIs[i]
		if target.CalleeName != api.callee {
			continue
		}
		if target.Resolved && missingPermissionCallTargetMatches(target.QualifiedName, api.callTargets) {
			return api, semantics.EvidenceResolved, true
		}
		if missingPermissionStaticReceiverMatches(ctx.File, target, api.staticReceivers) {
			return api, semantics.EvidenceQualifiedReceiver, true
		}
		if semantics.MatchQualifiedReceiver(ctx, call, api.receiverTypes...) {
			return api, semantics.EvidenceQualifiedReceiver, true
		}
		if missingPermissionReceiverDeclarationHasType(ctx, target.Receiver.Node, api.receiverTypes) {
			return api, semantics.EvidenceSameOwner, true
		}
	}
	if api, ok := missingPermissionAnnotatedSameFileTarget(ctx, call, target); ok {
		return api, semantics.EvidenceSameFileDeclaration, true
	}
	return permissionAPI{}, semantics.EvidenceUnresolved, false
}

func missingPermissionHasAnnotatedSameFileCandidate(ctx *v2.Context, call uint32, callee string) bool {
	if ctx == nil || ctx.File == nil || call == 0 || callee == "" {
		return false
	}
	file := ctx.File
	if !strings.Contains(string(file.Content), "RequiresPermission") {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if found || !missingPermissionFunctionHasName(file, fn, callee) {
			return
		}
		if _, ok := missingPermissionRequiredPermissionAnnotation(ctx, fn); ok && semantics.SameFileDeclarationMatch(ctx, fn, call) {
			found = true
		}
	})
	return found
}

func missingPermissionFunctionHasName(file *scanner.File, fn uint32, name string) bool {
	if file == nil || fn == 0 || name == "" {
		return false
	}
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "simple_identifier" && file.FlatNodeString(child, nil) == name {
			return true
		}
	}
	return false
}

func missingPermissionCallTargetMatches(got string, wants []string) bool {
	got = strings.ReplaceAll(strings.TrimSpace(got), "#", ".")
	for _, want := range wants {
		want = strings.ReplaceAll(strings.TrimSpace(want), "#", ".")
		if got == want || strings.HasSuffix(got, "."+want) {
			return true
		}
	}
	return false
}

func missingPermissionStaticReceiverMatches(file *scanner.File, target semantics.CallTarget, receivers []string) bool {
	if file == nil || !target.Receiver.Valid() || len(receivers) == 0 {
		return false
	}
	receiverPath := missingPermissionIdentifierPath(file, target.Receiver.Node)
	for _, receiver := range receivers {
		if receiverPath == receiver {
			return true
		}
		if receiverPath == missingPermissionSimpleName(receiver) && missingPermissionHasImport(file, receiver) {
			return true
		}
	}
	return false
}

func missingPermissionReceiverDeclarationHasType(ctx *v2.Context, receiver uint32, types []string) bool {
	if ctx == nil || ctx.File == nil || receiver == 0 || len(types) == 0 {
		return false
	}
	file := ctx.File
	name := semantics.ReferenceName(file, receiver)
	if name == "" {
		return false
	}
	for owner, ok := file.FlatParent(receiver); ok; owner, ok = file.FlatParent(owner) {
		switch file.FlatType(owner) {
		case "function_declaration", "secondary_constructor", "class_declaration", "object_declaration", "source_file":
			if missingPermissionOwnerDeclaresTypedName(file, owner, name, types) {
				return true
			}
		}
	}
	return false
}

func missingPermissionOwnerDeclaresTypedName(file *scanner.File, owner uint32, name string, types []string) bool {
	found := false
	file.FlatWalkAllNodes(owner, func(node uint32) {
		if found {
			return
		}
		switch file.FlatType(node) {
		case "parameter", "class_parameter", "property_declaration", "variable_declaration":
			if extractIdentifierFlat(file, node) == name && missingPermissionNodeMentionsType(file, node, types) {
				found = true
			}
		}
	})
	return found
}

func missingPermissionNodeMentionsType(file *scanner.File, node uint32, types []string) bool {
	found := false
	file.FlatWalkAllNodes(node, func(candidate uint32) {
		if found {
			return
		}
		switch file.FlatType(candidate) {
		case "type_identifier", "user_type":
			path := missingPermissionIdentifierPath(file, candidate)
			for _, typ := range types {
				if path == typ || path == missingPermissionSimpleName(typ) && missingPermissionHasImport(file, typ) {
					found = true
					return
				}
			}
		}
	})
	return found
}

func missingPermissionAnnotatedSameFileTarget(ctx *v2.Context, call uint32, target semantics.CallTarget) (permissionAPI, bool) {
	if ctx == nil || ctx.File == nil || target.CalleeName == "" {
		return permissionAPI{}, false
	}
	file := ctx.File
	var out permissionAPI
	found := false
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if found || !semantics.SameFileDeclarationMatch(ctx, fn, call) {
			return
		}
		perm, ok := missingPermissionRequiredPermissionAnnotation(ctx, fn)
		if !ok {
			return
		}
		out = permissionAPI{api: target.CalleeName, callee: target.CalleeName, perm: perm}
		found = true
	})
	return out, found
}

func missingPermissionHasStructuralGuard(ctx *v2.Context, call uint32, perm string) bool {
	return missingPermissionEnclosingPermissionCondition(ctx, call, perm) ||
		missingPermissionHasPrecedingGrantedReturnGuard(ctx, call, perm) ||
		missingPermissionEnclosingAnnotationRequires(ctx, call, perm) ||
		missingPermissionEnclosingCatchesSecurityException(ctx, call)
}

type missingPermissionGuardState int

const (
	missingPermissionGuardUnknown missingPermissionGuardState = iota
	missingPermissionGuardGranted
	missingPermissionGuardDenied
)

func missingPermissionEnclosingPermissionCondition(ctx *v2.Context, call uint32, perm string) bool {
	file := ctx.File
	for body, ok := file.FlatParent(call); ok; body, ok = file.FlatParent(body) {
		if file.FlatType(body) != "control_structure_body" {
			continue
		}
		parent, ok := file.FlatParent(body)
		if !ok || file.FlatType(parent) != "if_expression" {
			continue
		}
		cond, thenBody, elseBody := missingPermissionIfParts(file, parent)
		if cond == 0 || body != thenBody && body != elseBody {
			continue
		}
		state := missingPermissionConditionPermissionState(ctx, cond, perm)
		if state == missingPermissionGuardGranted && body == thenBody {
			return true
		}
		if state == missingPermissionGuardDenied && body == elseBody {
			return true
		}
	}
	return false
}

func missingPermissionConditionPermissionState(ctx *v2.Context, cond uint32, perm string) missingPermissionGuardState {
	if ctx == nil || ctx.File == nil || cond == 0 {
		return missingPermissionGuardUnknown
	}
	file := ctx.File
	cond = flatUnwrapParenExpr(file, cond)
	switch file.FlatType(cond) {
	case "equality_expression":
		return missingPermissionEqualityPermissionState(ctx, cond, perm)
	case "conjunction_expression":
		for child := file.FlatFirstChild(cond); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			if missingPermissionConditionPermissionState(ctx, child, perm) == missingPermissionGuardGranted {
				return missingPermissionGuardGranted
			}
		}
	}
	return missingPermissionGuardUnknown
}

func missingPermissionEqualityPermissionState(ctx *v2.Context, expr uint32, perm string) missingPermissionGuardState {
	file := ctx.File
	if file.FlatType(expr) != "equality_expression" || file.FlatChildCount(expr) < 3 {
		return missingPermissionGuardUnknown
	}
	left := flatUnwrapParenExpr(file, file.FlatChild(expr, 0))
	op := strings.TrimSpace(file.FlatNodeText(file.FlatChild(expr, 1)))
	right := flatUnwrapParenExpr(file, file.FlatChild(expr, file.FlatChildCount(expr)-1))
	if left == 0 || op == "" || right == 0 {
		return missingPermissionGuardUnknown
	}

	if missingPermissionCheckSelfPermissionCall(ctx, left, perm) {
		return missingPermissionComparePermissionResult(file, op, right)
	}
	if missingPermissionCheckSelfPermissionCall(ctx, right, perm) {
		return missingPermissionComparePermissionResult(file, op, left)
	}
	return missingPermissionGuardUnknown
}

func missingPermissionComparePermissionResult(file *scanner.File, op string, result uint32) missingPermissionGuardState {
	granted, ok := missingPermissionPermissionResultConstant(file, result)
	if !ok {
		return missingPermissionGuardUnknown
	}
	switch op {
	case "==":
		if granted {
			return missingPermissionGuardGranted
		}
		return missingPermissionGuardDenied
	case "!=":
		if granted {
			return missingPermissionGuardDenied
		}
		return missingPermissionGuardGranted
	default:
		return missingPermissionGuardUnknown
	}
}

func missingPermissionPermissionResultConstant(file *scanner.File, expr uint32) (granted bool, ok bool) {
	expr = flatUnwrapParenExpr(file, expr)
	path := missingPermissionIdentifierPath(file, expr)
	switch missingPermissionSimpleName(path) {
	case "PERMISSION_GRANTED", "GRANTED":
		return true, true
	case "PERMISSION_DENIED", "DENIED":
		return false, true
	default:
		return false, false
	}
}

func missingPermissionCheckSelfPermissionCall(ctx *v2.Context, call uint32, perm string) bool {
	if ctx == nil || ctx.File == nil || ctx.File.FlatType(call) != "call_expression" {
		return false
	}
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || target.CalleeName != "checkSelfPermission" {
		return false
	}
	for _, arg := range target.Arguments {
		if missingPermissionExprMatchesPermission(ctx, arg.Node, perm) {
			return true
		}
	}
	return false
}

func missingPermissionHasPrecedingGrantedReturnGuard(ctx *v2.Context, call uint32, perm string) bool {
	if ctx == nil || ctx.File == nil || call == 0 {
		return false
	}
	file := ctx.File
	scope := enclosingAncestor(file, call, "function_declaration", "secondary_constructor", "anonymous_initializer", "lambda_literal")
	if scope == 0 {
		return false
	}
	targetStart := file.FlatStartByte(call)
	guarded := false
	file.FlatWalkNodes(scope, "if_expression", func(ifExpr uint32) {
		if guarded || file.FlatStartByte(ifExpr) >= targetStart {
			return
		}
		ifStmt, ifContainer := missingPermissionContainingStatement(file, ifExpr)
		callStmt, callContainer := missingPermissionContainingStatement(file, call)
		if ifContainer == 0 || ifContainer != callContainer || file.FlatEndByte(ifStmt) > file.FlatStartByte(callStmt) {
			return
		}
		cond, thenBody, _ := missingPermissionIfParts(file, ifExpr)
		if cond == 0 || thenBody == 0 {
			return
		}
		if missingPermissionConditionPermissionState(ctx, cond, perm) != missingPermissionGuardDenied {
			return
		}
		if missingPermissionBodyTerminates(file, thenBody) {
			guarded = true
		}
	})
	return guarded
}

func missingPermissionContainingStatement(file *scanner.File, idx uint32) (stmt uint32, container uint32) {
	if file == nil || idx == 0 {
		return 0, 0
	}
	for cur, ok := idx, true; ok; cur, ok = file.FlatParent(cur) {
		parent, hasParent := file.FlatParent(cur)
		if !hasParent {
			return cur, 0
		}
		switch file.FlatType(parent) {
		case "statements", "function_body", "control_structure_body", "lambda_literal", "anonymous_initializer":
			return cur, parent
		}
	}
	return 0, 0
}

func missingPermissionBodyTerminates(file *scanner.File, body uint32) bool {
	if file == nil || body == 0 {
		return false
	}
	for child := file.FlatFirstChild(body); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if missingPermissionNodeTerminates(file, child) {
			return true
		}
	}
	return missingPermissionNodeTerminates(file, body)
}

func missingPermissionNodeTerminates(file *scanner.File, node uint32) bool {
	switch file.FlatType(node) {
	case "jump_expression":
		text := strings.TrimSpace(file.FlatNodeText(node))
		return strings.HasPrefix(text, "return") || strings.HasPrefix(text, "throw")
	case "call_expression":
		target, ok := semantics.ResolveCallTarget(&v2.Context{File: file}, node)
		return ok && (target.CalleeName == "error" || target.CalleeName == "TODO")
	default:
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) {
				return missingPermissionNodeTerminates(file, child)
			}
		}
		return false
	}
}

func missingPermissionEnclosingAnnotationRequires(ctx *v2.Context, call uint32, perm string) bool {
	for owner, ok := ctx.File.FlatParent(call); ok; owner, ok = ctx.File.FlatParent(owner) {
		switch ctx.File.FlatType(owner) {
		case "function_declaration", "class_declaration", "object_declaration":
			if required, ok := missingPermissionRequiredPermissionAnnotation(ctx, owner); ok && required == perm {
				return true
			}
		}
	}
	return false
}

func missingPermissionRequiredPermissionAnnotation(ctx *v2.Context, node uint32) (string, bool) {
	file := ctx.File
	if mods, ok := file.FlatFindChild(node, "modifiers"); ok {
		if perm, ok := missingPermissionAnnotationContainerPermission(ctx, mods); ok {
			return perm, true
		}
	}
	for prev, ok := file.FlatPrevSibling(node); ok; prev, ok = file.FlatPrevSibling(prev) {
		switch file.FlatType(prev) {
		case "prefix_expression", "annotation", "modifiers":
			if perm, ok := missingPermissionAnnotationContainerPermission(ctx, prev); ok {
				return perm, true
			}
		default:
			if strings.TrimSpace(file.FlatNodeText(prev)) != "" {
				return "", false
			}
		}
	}
	return "", false
}

func missingPermissionAnnotationContainerPermission(ctx *v2.Context, container uint32) (string, bool) {
	file := ctx.File
	var perm string
	if !strings.Contains(file.FlatNodeText(container), "RequiresPermission") {
		return "", false
	}
	file.FlatWalkAllNodes(container, func(candidate uint32) {
		if perm != "" {
			return
		}
		if p, ok := missingPermissionExprPermissionName(ctx, candidate); ok {
			perm = p
		}
	})
	return perm, perm != ""
}

func missingPermissionEnclosingCatchesSecurityException(ctx *v2.Context, call uint32) bool {
	for _, handler := range semantics.EnclosingCaughtExceptionHandlers(ctx, call) {
		if handler.TypeName == "SecurityException" || handler.TypeName == "java.lang.SecurityException" {
			return true
		}
	}
	return false
}

func missingPermissionExprMatchesPermission(ctx *v2.Context, expr uint32, perm string) bool {
	if p, ok := missingPermissionExprPermissionName(ctx, expr); ok {
		return p == perm
	}
	if ctx == nil || ctx.File == nil || expr == 0 {
		return false
	}
	found := false
	ctx.File.FlatWalkAllNodes(expr, func(node uint32) {
		if found {
			return
		}
		if p, ok := missingPermissionExprPermissionName(ctx, node); ok && p == perm {
			found = true
		}
	})
	return found
}

func missingPermissionExprPermissionName(ctx *v2.Context, expr uint32) (string, bool) {
	if ctx == nil || ctx.File == nil || expr == 0 {
		return "", false
	}
	if val, ok := semantics.EvalConst(ctx, expr); ok && val.Kind == "string" {
		return missingPermissionStringPermissionName(val.String)
	}
	file := ctx.File
	switch file.FlatType(expr) {
	case "simple_identifier", "navigation_expression", "user_type", "type_identifier":
		path := missingPermissionIdentifierPath(file, expr)
		if path == "" {
			return "", false
		}
		last := missingPermissionSimpleName(path)
		if !missingPermissionKnownPerms[last] {
			return "", false
		}
		if strings.Contains(path, ".permission.") || strings.Contains(path, "Manifest.permission.") || path == last {
			return last, true
		}
	}
	return "", false
}

func missingPermissionStringPermissionName(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "android.permission.") {
		value = strings.TrimPrefix(value, "android.permission.")
	}
	if missingPermissionKnownPerms[value] {
		return value, true
	}
	return "", false
}

func missingPermissionIdentifierPath(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	var parts []string
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		switch file.FlatType(candidate) {
		case "simple_identifier", "type_identifier":
			parts = append(parts, file.FlatNodeString(candidate, nil))
		}
	})
	return strings.Join(parts, ".")
}

func missingPermissionSimpleName(name string) string {
	name = strings.TrimSpace(strings.Trim(name, "`"))
	if idx := strings.LastIndex(name, "."); idx >= 0 && idx+1 < len(name) {
		return name[idx+1:]
	}
	return name
}

func missingPermissionHasImport(file *scanner.File, fqn string) bool {
	found := false
	file.FlatWalkNodes(0, "import_header", func(node uint32) {
		if found {
			return
		}
		path := missingPermissionIdentifierPath(file, node)
		found = path == fqn
	})
	return found
}

func missingPermissionIfParts(file *scanner.File, idx uint32) (cond uint32, thenBody uint32, elseBody uint32) {
	foundElse := false
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "control_structure_body":
			if !foundElse && thenBody == 0 {
				thenBody = child
			} else if foundElse && elseBody == 0 {
				elseBody = child
			}
		case "else":
			foundElse = true
		default:
			if cond == 0 && file.FlatIsNamed(child) {
				cond = child
			}
		}
	}
	return cond, thenBody, elseBody
}

type WrongConstantRule struct {
	FlatDispatchBase
	AndroidRule
}

type wrongConstantEntry struct {
	Method      string
	Description string
	Values      map[int64]bool
	Flag        bool
}

var wrongConstantEntries = []wrongConstantEntry{
	{Method: "setVisibility", Description: "setVisibility() should use View.VISIBLE, View.INVISIBLE, or View.GONE constants.", Values: int64Set(0, 4, 8)},
	{Method: "setLayoutDirection", Description: "setLayoutDirection() should use View.LAYOUT_DIRECTION_* constants.", Values: int64Set(0, 1, 2, 3)},
	{Method: "setImportantForAccessibility", Description: "setImportantForAccessibility() should use View.IMPORTANT_FOR_ACCESSIBILITY_* constants.", Values: int64Set(0, 1, 2, 4)},
	{Method: "setGravity", Description: "setGravity() should use Gravity.* constants.", Values: int64Set(0, 1, 2, 3, 4, 5, 7, 8, 16, 17, 48, 80, 112, 119, 128, 8388611, 8388613), Flag: true},
	{Method: "setOrientation", Description: "setOrientation() should use LinearLayout.HORIZONTAL or LinearLayout.VERTICAL.", Values: int64Set(0, 1)},
}

func int64Set(values ...int64) map[int64]bool {
	out := make(map[int64]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func (r *WrongConstantRule) NodeTypes() []string { return []string{"call_expression"} }

// Confidence is medium-high: findings require a structural call plus either a
// resolved Android framework target or same-file constant-set annotation.
func (r *WrongConstantRule) Confidence() float64 { return 0.85 }

func (r *WrongConstantRule) check(ctx *v2.Context) {
	if ctx == nil || ctx.File == nil || ctx.Idx == 0 || ctx.File.FlatType(ctx.Idx) != "call_expression" {
		return
	}
	file := ctx.File
	method := flatCallExpressionName(file, ctx.Idx)
	entry, ok := wrongConstantEntryForMethod(method)
	if !ok {
		return
	}
	arg := wrongConstantFirstValueArgument(file, ctx.Idx)
	if arg == 0 {
		return
	}
	expr := wrongConstantValueArgumentExpression(file, arg)
	if expr == 0 {
		return
	}
	allowed, anchored := wrongConstantAllowedConstants(ctx, ctx.Idx, entry)
	if !anchored || len(allowed.Values) == 0 {
		return
	}
	if literal := wrongConstantFirstIntegerLiteral(file, expr); literal != 0 {
		ctx.EmitAt(file.FlatRow(literal)+1, file.FlatCol(literal)+1, entry.Description)
		return
	}
	value, ok := wrongConstantEvalIntExpr(file, expr, wrongConstantFileConstants(file))
	if !ok {
		return
	}
	if !allowed.ValueAllowed(value) {
		ctx.EmitAt(file.FlatRow(expr)+1, file.FlatCol(expr)+1, entry.Description)
	}
}

type wrongConstantAllowedSet struct {
	Values map[int64]bool
	Flag   bool
}

func (s wrongConstantAllowedSet) ValueAllowed(value int64) bool {
	if s.Values[value] {
		return true
	}
	if !s.Flag {
		return false
	}
	var allowedBits int64
	for v := range s.Values {
		allowedBits |= v
	}
	return value&^allowedBits == 0
}

func wrongConstantEntryForMethod(method string) (wrongConstantEntry, bool) {
	for _, entry := range wrongConstantEntries {
		if entry.Method == method {
			return entry, true
		}
	}
	return wrongConstantEntry{}, false
}

func wrongConstantFirstValueArgument(file *scanner.File, call uint32) uint32 {
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return 0
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) == "value_argument" {
			return arg
		}
	}
	return 0
}

func wrongConstantValueArgumentExpression(file *scanner.File, arg uint32) uint32 {
	return flatLastNamedChild(file, arg)
}

func wrongConstantAllowedConstants(ctx *v2.Context, call uint32, entry wrongConstantEntry) (wrongConstantAllowedSet, bool) {
	if target := wrongConstantOracleCallTarget(ctx, call); target != "" {
		if wrongConstantFrameworkTargetMatches(target, entry.Method) {
			return wrongConstantAllowedSet{Values: entry.Values, Flag: entry.Flag}, true
		}
	}
	if set, ok := wrongConstantSameFileAllowedConstants(ctx, call, entry.Method); ok {
		return set, true
	}
	return wrongConstantAllowedSet{}, false
}

func wrongConstantOracleCallTarget(ctx *v2.Context, idx uint32) string {
	if ctx == nil || ctx.Resolver == nil || ctx.File == nil {
		return ""
	}
	var lookup oracle.Lookup
	if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
		lookup = cr.Oracle()
	}
	if lookup == nil {
		return ""
	}
	return oracleLookupCallTargetFlat(lookup, ctx.File, idx)
}

func wrongConstantFrameworkTargetMatches(target, method string) bool {
	if target == "" || method == "" {
		return false
	}
	if !(strings.HasSuffix(target, "."+method) || strings.HasSuffix(target, "#"+method)) {
		return false
	}
	switch method {
	case "setVisibility", "setLayoutDirection", "setImportantForAccessibility":
		return strings.Contains(target, "android.view.View.")
	case "setOrientation":
		return strings.Contains(target, "android.widget.LinearLayout.") ||
			strings.Contains(target, "androidx.recyclerview.widget.LinearLayoutManager.")
	case "setGravity":
		return strings.HasPrefix(target, "android.") || strings.HasPrefix(target, "androidx.")
	default:
		return false
	}
}

func wrongConstantSameFileAllowedConstants(ctx *v2.Context, call uint32, method string) (wrongConstantAllowedSet, bool) {
	file := ctx.File
	receiver := wrongConstantReceiverNode(file, call)
	if receiver == 0 {
		return wrongConstantAllowedSet{}, false
	}
	className := wrongConstantReceiverClassName(ctx, receiver)
	if className == "" {
		return wrongConstantAllowedSet{}, false
	}
	classNode := flatFindSameFileClassLikeDeclaration(file, className)
	if classNode == 0 {
		return wrongConstantAllowedSet{}, false
	}
	fn := wrongConstantClassFunction(file, classNode, method)
	if fn == 0 {
		return wrongConstantAllowedSet{}, false
	}
	_, mods := wrongConstantFunctionParameter(file, fn, 0)
	if mods == 0 {
		return wrongConstantAllowedSet{}, false
	}
	return wrongConstantAllowedSetFromModifiers(file, mods, classNode)
}

func wrongConstantReceiverNode(file *scanner.File, call uint32) uint32 {
	nav, _ := flatCallExpressionParts(file, call)
	if nav == 0 || file.FlatNamedChildCount(nav) == 0 {
		return 0
	}
	return file.FlatNamedChild(nav, 0)
}

func wrongConstantReceiverClassName(ctx *v2.Context, receiver uint32) string {
	file := ctx.File
	if ctx.Resolver != nil {
		if typ := ctx.Resolver.ResolveFlatNode(receiver, file); typ != nil && typ.Name != "" {
			return typ.Name
		}
	}
	if file.FlatType(receiver) != "simple_identifier" {
		return ""
	}
	decl := resolveSimpleReferenceDeclarationFlat(file, receiver)
	if decl == 0 {
		return ""
	}
	return wrongConstantDeclaredTypeName(file, decl)
}

func wrongConstantDeclaredTypeName(file *scanner.File, decl uint32) string {
	var typeNode uint32
	file.FlatWalkAllNodes(decl, func(candidate uint32) {
		if typeNode != 0 || file.FlatType(candidate) != "user_type" {
			return
		}
		typeNode = candidate
	})
	if typeNode == 0 {
		return ""
	}
	return wrongConstantTypeName(file, typeNode)
}

func wrongConstantClassFunction(file *scanner.File, classNode uint32, method string) uint32 {
	var found uint32
	file.FlatWalkAllNodes(classNode, func(candidate uint32) {
		if found != 0 || file.FlatType(candidate) != "function_declaration" || extractIdentifierFlat(file, candidate) != method {
			return
		}
		owner, ok := flatEnclosingAncestor(file, candidate, "class_declaration", "object_declaration")
		if ok && owner == classNode {
			found = candidate
		}
	})
	return found
}

func wrongConstantFunctionParameter(file *scanner.File, fn uint32, index int) (uint32, uint32) {
	params, _ := file.FlatFindChild(fn, "function_value_parameters")
	if params == 0 {
		return 0, 0
	}
	seen := 0
	var pendingMods uint32
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "parameter_modifiers":
			pendingMods = child
		case "parameter":
			if seen == index {
				return child, pendingMods
			}
			seen++
			pendingMods = 0
		}
	}
	return 0, 0
}

func wrongConstantAllowedSetFromModifiers(file *scanner.File, mods, classNode uint32) (wrongConstantAllowedSet, bool) {
	var found wrongConstantAllowedSet
	ok := false
	file.FlatWalkAllNodes(mods, func(ann uint32) {
		if ok || file.FlatType(ann) != "annotation" {
			return
		}
		if set, setOK := wrongConstantAllowedSetFromAnnotation(file, ann, classNode); setOK {
			found = set
			ok = true
		}
	})
	return found, ok
}

func wrongConstantAllowedSetFromAnnotation(file *scanner.File, ann, classNode uint32) (wrongConstantAllowedSet, bool) {
	name := wrongConstantAnnotationName(file, ann)
	if name == "" {
		return wrongConstantAllowedSet{}, false
	}
	if name == "IntDef" || name == "LongDef" {
		values := wrongConstantAnnotationValues(file, ann, classNode)
		return wrongConstantAllowedSet{Values: values, Flag: wrongConstantAnnotationFlag(file, ann)}, len(values) > 0
	}
	annotationClass := flatFindSameFileClassLikeDeclaration(file, name)
	if annotationClass == 0 || !file.FlatHasModifier(annotationClass, "annotation") {
		return wrongConstantAllowedSet{}, false
	}
	if mods, ok := file.FlatFindChild(annotationClass, "modifiers"); ok {
		return wrongConstantAllowedSetFromModifiers(file, mods, classNode)
	}
	return wrongConstantAllowedSet{}, false
}

func wrongConstantAnnotationName(file *scanner.File, ann uint32) string {
	var name string
	file.FlatWalkAllNodes(ann, func(candidate uint32) {
		if name != "" || file.FlatType(candidate) != "user_type" {
			return
		}
		name = wrongConstantTypeName(file, candidate)
	})
	return name
}

func wrongConstantTypeName(file *scanner.File, idx uint32) string {
	var last string
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		switch file.FlatType(candidate) {
		case "simple_identifier", "type_identifier":
			last = file.FlatNodeText(candidate)
		}
	})
	return last
}

func wrongConstantAnnotationValues(file *scanner.File, ann, classNode uint32) map[int64]bool {
	args := wrongConstantAnnotationValueArguments(file, ann)
	if args == 0 {
		return nil
	}
	consts := wrongConstantFileConstants(file)
	out := map[int64]bool{}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		label := flatValueArgumentLabel(file, arg)
		if label != "" && label != "value" {
			continue
		}
		expr := wrongConstantValueArgumentExpression(file, arg)
		if expr == 0 {
			continue
		}
		if value, ok := wrongConstantEvalIntExpr(file, expr, consts); ok {
			out[value] = true
			continue
		}
		if file.FlatType(expr) == "simple_identifier" && classNode != 0 {
			if value, ok := consts[extractIdentifierFlat(file, classNode)+"."+file.FlatNodeText(expr)]; ok {
				out[value] = true
			}
		}
	}
	return out
}

func wrongConstantAnnotationFlag(file *scanner.File, ann uint32) bool {
	args := wrongConstantAnnotationValueArguments(file, ann)
	if args == 0 {
		return false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" || flatValueArgumentLabel(file, arg) != "flag" {
			continue
		}
		expr := wrongConstantValueArgumentExpression(file, arg)
		return expr != 0 && file.FlatNodeText(expr) == "true"
	}
	return false
}

func wrongConstantAnnotationValueArguments(file *scanner.File, ann uint32) uint32 {
	var args uint32
	file.FlatWalkAllNodes(ann, func(candidate uint32) {
		if args == 0 && file.FlatType(candidate) == "value_arguments" {
			args = candidate
		}
	})
	return args
}

func wrongConstantFileConstants(file *scanner.File) map[string]int64 {
	consts := map[string]int64{}
	file.FlatWalkAllNodes(0, func(candidate uint32) {
		if file.FlatType(candidate) != "property_declaration" || !file.FlatHasModifier(candidate, "const") {
			return
		}
		name := extractIdentifierFlat(file, candidate)
		if name == "" {
			return
		}
		expr := flatLastNamedChild(file, candidate)
		if expr == 0 || expr == candidate {
			return
		}
		value, ok := wrongConstantEvalIntExpr(file, expr, consts)
		if !ok {
			return
		}
		consts[name] = value
		if owner := wrongConstantConstantOwnerName(file, candidate); owner != "" {
			consts[owner+"."+name] = value
		}
	})
	return consts
}

func wrongConstantConstantOwnerName(file *scanner.File, idx uint32) string {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration", "object_declaration":
			if name := extractIdentifierFlat(file, cur); name != "" {
				return name
			}
		}
	}
	return ""
}

func wrongConstantEvalIntExpr(file *scanner.File, idx uint32, consts map[string]int64) (int64, bool) {
	idx = flatUnwrapParenExpr(file, idx)
	switch file.FlatType(idx) {
	case "integer_literal":
		return parseKotlinIntegerLiteral(file.FlatNodeText(idx))
	case "prefix_expression":
		lit := wrongConstantFirstIntegerLiteral(file, idx)
		if lit == 0 {
			return 0, false
		}
		value, ok := parseKotlinIntegerLiteral(file.FlatNodeText(lit))
		if !ok {
			return 0, false
		}
		if strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(idx)), "-") {
			return -value, true
		}
		return value, true
	case "simple_identifier":
		value, ok := consts[file.FlatNodeText(idx)]
		return value, ok
	case "navigation_expression":
		path, ok := flatReferencePathFromExpr(file, idx)
		if !ok {
			return 0, false
		}
		value, ok := consts[strings.Join(path.parts, ".")]
		return value, ok
	case "infix_expression", "additive_expression", "multiplicative_expression":
		return wrongConstantEvalInfixExpr(file, idx, consts)
	default:
		return 0, false
	}
}

func wrongConstantEvalInfixExpr(file *scanner.File, idx uint32, consts map[string]int64) (int64, bool) {
	var values []int64
	var op string
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatType(child) == "simple_identifier" {
			text := file.FlatNodeText(child)
			if text == "or" || text == "and" || text == "shl" || text == "shr" {
				op = text
				continue
			}
		}
		value, ok := wrongConstantEvalIntExpr(file, child, consts)
		if !ok {
			return 0, false
		}
		values = append(values, value)
	}
	if len(values) != 2 || op == "" {
		return 0, false
	}
	switch op {
	case "or":
		return values[0] | values[1], true
	case "and":
		return values[0] & values[1], true
	case "shl":
		return values[0] << values[1], true
	case "shr":
		return values[0] >> values[1], true
	default:
		return 0, false
	}
}

func wrongConstantFirstIntegerLiteral(file *scanner.File, idx uint32) uint32 {
	var found uint32
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		if found == 0 && file.FlatType(candidate) == "integer_literal" {
			found = candidate
		}
	})
	return found
}

func parseKotlinIntegerLiteral(text string) (int64, bool) {
	text = strings.TrimSpace(strings.ReplaceAll(text, "_", ""))
	text = strings.TrimSuffix(strings.TrimSuffix(text, "L"), "l")
	if text == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(text, 0, 64)
	if err != nil {
		return 0, false
	}
	return value, true
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

// rtlAwareMethods maps View member names to their RTL-aware replacements.
// Keys are bare callee identifiers; the rule uses flatCallExpressionName
// to match, so there is no need to encode leading `.` / trailing `(`.
var rtlAwareMethods = map[string]string{
	"getLeft":         "getStart()",
	"getRight":        "getEnd()",
	"getPaddingLeft":  "getPaddingStart()",
	"getPaddingRight": "getPaddingEnd()",
}

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
func (r *GridLayoutRule) Confidence() float64 { return 0.85 }

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
func (r *ResourceNameRule) Confidence() float64 { return 0.9 }

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
func (r *UnknownIdInLayoutRule) Confidence() float64 { return 0.9 }
