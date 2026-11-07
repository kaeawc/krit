package rules

import (
	"bytes"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// AnimatorDurationIgnoresScaleRule flags animator durations that ignore the
// system animation scale preference. This initial implementation uses a
// file-level opt-out on ANIMATOR_DURATION_SCALE references and targets the
// common ValueAnimator/ObjectAnimator call patterns in fixtures.
type AnimatorDurationIgnoresScaleRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Accessibility rule. Detection matches on Compose/View API call shapes
// and argument labels by name rather than by resolved type. Classified per
// roadmap/17.
func (r *AnimatorDurationIgnoresScaleRule) Confidence() float64 { return api.ConfidenceMedium }

func animatorDurationScaleReferenced(ctx *api.Context, anchor uint32) bool {
	if ctx.File == nil {
		return false
	}
	file := ctx.File
	root := uint32(0)
	if fn, ok := flatEnclosingFunction(file, anchor); ok {
		root = fn
	}
	found := false
	file.FlatWalkNodes(root, "navigation_expression", func(idx uint32) {
		if found {
			return
		}
		segments := flatNavigationChainIdentifiers(file, idx)
		if len(segments) < 2 || segments[len(segments)-1] != "ANIMATOR_DURATION_SCALE" {
			return
		}
		if len(segments) >= 3 && segments[len(segments)-3] == "Settings" && segments[len(segments)-2] == "Global" {
			found = true
			return
		}
		if ctx.Resolver != nil && ctx.Resolver.ResolveImport("Settings", file) == "android.provider.Settings" {
			found = true
		}
	})
	return found
}

func animatorReceiverConfirmed(ctx *api.Context, call uint32) bool {
	if ctx.File == nil {
		return false
	}
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || target.CalleeName != "setDuration" {
		return false
	}
	if target.Resolved && animatorCallTargetIsAnimator(target.QualifiedName) {
		return true
	}
	if semantics.MatchQualifiedReceiver(ctx, call,
		"android.animation.ValueAnimator",
		"android.animation.ObjectAnimator",
		"ValueAnimator",
		"ObjectAnimator",
	) {
		return true
	}
	receiver := target.Receiver.Node
	if receiver != 0 && animatorExpressionProducesAnimator(ctx, receiver) {
		return true
	}
	return false
}

func accessibilityCallExpressionParts(file *scanner.File, idx uint32) (uint32, uint32) {
	navExpr, args := flatCallExpressionParts(file, idx)
	if navExpr != 0 || args != 0 {
		return navExpr, args
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) && file.FlatType(child) == "call_expression" {
			return accessibilityCallExpressionParts(file, child)
		}
	}
	return 0, 0
}

func animatorAssignmentTargetConfirmed(ctx *api.Context, assignment uint32) bool {
	if ctx.File == nil || ctx.File.FlatType(assignment) != "assignment" {
		return false
	}
	file := ctx.File
	if file.FlatNamedChildCount(assignment) == 0 {
		return false
	}
	lhs := file.FlatNamedChild(assignment, 0)
	if flatNavigationExpressionLastIdentifier(file, lhs) != "duration" && !file.FlatNodeTextEquals(lhs, "duration") {
		return false
	}
	for parent, ok := file.FlatParent(assignment); ok; parent, ok = file.FlatParent(parent) {
		if animatorExpressionProducesAnimator(ctx, parent) {
			return true
		}
		switch file.FlatType(parent) {
		case "function_declaration", "class_declaration", "source_file":
			return false
		}
	}
	return false
}

func animatorExpressionProducesAnimator(ctx *api.Context, idx uint32) bool {
	if ctx.File == nil || idx == 0 {
		return false
	}
	file := ctx.File
	if ctx.Resolver != nil {
		if typ := ctx.Resolver.ResolveFlatNode(idx, file); animatorTypeMatches(typ) {
			return true
		}
	}
	switch file.FlatType(idx) {
	case "call_expression":
		target, ok := semantics.ResolveCallTarget(ctx, idx)
		if ok && target.Resolved && animatorCallTargetIsAnimator(target.QualifiedName) {
			return true
		}
		navExpr, _ := accessibilityCallExpressionParts(file, idx)
		if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
			return false
		}
		receiver := file.FlatNamedChild(navExpr, 0)
		callee := flatNavigationExpressionLastIdentifier(file, navExpr)
		if animatorScopeFunctionReturnsReceiver(callee) && animatorExpressionProducesAnimator(ctx, receiver) {
			return true
		}
		if (callee == "ofFloat" || callee == "ofInt" || callee == "ofArgb" || callee == "ofObject") &&
			animatorStaticReceiverConfirmed(ctx, receiver, "android.animation.ValueAnimator", "ValueAnimator") {
			return true
		}
		if (callee == "ofFloat" || callee == "ofInt" || callee == "ofArgb" || callee == "ofObject" || strings.HasPrefix(callee, "of")) &&
			animatorStaticReceiverConfirmed(ctx, receiver, "android.animation.ObjectAnimator", "ObjectAnimator") {
			return true
		}
	case "navigation_expression", "simple_identifier":
		return animatorStaticReceiverConfirmed(ctx, idx, "android.animation.ValueAnimator", "ValueAnimator") ||
			animatorStaticReceiverConfirmed(ctx, idx, "android.animation.ObjectAnimator", "ObjectAnimator")
	}
	return false
}

func animatorScopeFunctionReturnsReceiver(callee string) bool {
	switch callee {
	case "also", "apply":
		return true
	default:
		return false
	}
}

func animatorStaticReceiverConfirmed(ctx *api.Context, idx uint32, fqn string, simple string) bool {
	if ctx.File == nil || idx == 0 {
		return false
	}
	file := ctx.File
	if ctx.Resolver != nil {
		if typ := ctx.Resolver.ResolveFlatNode(idx, file); animatorTypeMatches(typ) {
			return true
		}
		name := semantics.ReferenceName(file, idx)
		if name != "" {
			if imported := ctx.Resolver.ResolveImport(name, file); imported == fqn {
				return true
			}
		}
	}
	segments := accessibilityReferenceSegments(file, idx)
	if len(segments) == 1 && segments[0] == simple {
		return fileImportsFQN(file, fqn)
	}
	return strings.Join(segments, ".") == fqn
}

func accessibilityReferenceSegments(file *scanner.File, idx uint32) []string {
	if file == nil || idx == 0 {
		return nil
	}
	switch file.FlatType(idx) {
	case "simple_identifier":
		return []string{file.FlatNodeText(idx)}
	case "navigation_expression":
		return flatNavigationChainIdentifiers(file, idx)
	default:
		return nil
	}
}

func animatorTypeMatches(typ *typeinfer.ResolvedType) bool {
	if typ == nil || typ.Kind == typeinfer.TypeUnknown {
		return false
	}
	switch typ.FQN {
	case "android.animation.ValueAnimator", "android.animation.ObjectAnimator":
		return true
	}
	for _, st := range typ.Supertypes {
		if st == "android.animation.ValueAnimator" || st == "android.animation.ObjectAnimator" {
			return true
		}
	}
	return false
}

func animatorCallTargetIsAnimator(target string) bool {
	target = strings.ReplaceAll(target, "#", ".")
	return strings.HasPrefix(target, "android.animation.ValueAnimator.") ||
		strings.HasPrefix(target, "android.animation.ObjectAnimator.")
}

// ComposeClickableWithoutMinTouchTargetRule flags clickable Modifier chains
// with explicit width/height/size dp literals below the 48.dp minimum touch
// target. Implicit-size cases are too context-dependent to flag reliably
// without layout awareness.
type ComposeClickableWithoutMinTouchTargetRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Accessibility rule. Detection matches on Compose/View API call shapes
// and argument labels by name rather than by resolved type. Classified per
// roadmap/17.
func (r *ComposeClickableWithoutMinTouchTargetRule) Confidence() float64 { return api.ConfidenceMedium }

type composeModifierCall struct {
	name string
	args uint32
}

var (
	composeMinTouchTargetSizeCalls = map[string]bool{
		"height": true,
		"size":   true,
		"width":  true,
	}
)

func composeModifierCallChainFlat(file *scanner.File, idx uint32) ([]composeModifierCall, bool) {
	if idx == 0 {
		return nil, false
	}

	switch file.FlatType(idx) {
	case "call_expression":
		navExpr, args := accessibilityCallExpressionParts(file, idx)
		if navExpr == 0 {
			return nil, false
		}

		chain, rootedAtModifier := composeModifierCallChainFlat(file, composeModifierChainReceiverFlat(file, navExpr))
		name := flatNavigationExpressionLastIdentifier(file, navExpr)
		if name == "" {
			return chain, rootedAtModifier
		}
		return append(chain, composeModifierCall{name: name, args: args}), rootedAtModifier
	case "navigation_expression":
		return nil, composeModifierRootImported(file, idx)
	case "parenthesized_expression":
		if file.FlatNamedChildCount(idx) == 1 {
			return composeModifierCallChainFlat(file, file.FlatNamedChild(idx, 0))
		}
		return nil, false
	case "simple_identifier":
		return nil, composeModifierRootImported(file, idx)
	default:
		return nil, false
	}
}

func composeModifierCallChainImportsConfirmed(file *scanner.File, chain []composeModifierCall) bool {
	if file == nil {
		return false
	}
	for _, call := range chain {
		switch call.name {
		case "clickable":
			if fileImportsFQN(file, "androidx.compose.foundation.clickable") {
				return true
			}
		case "minimumInteractiveComponentSize":
			if fileImportsFQN(file, "androidx.compose.foundation.minimumInteractiveComponentSize") {
				return true
			}
		case "height", "size", "width":
			if fileImportsFQN(file, "androidx.compose.foundation.layout."+call.name) {
				return true
			}
		}
	}
	return false
}

func composeModifierRootImported(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	segments := accessibilityReferenceSegments(file, idx)
	return len(segments) >= 1 && segments[0] == "Modifier" && fileImportsFQN(file, "androidx.compose.ui.Modifier")
}

func composeModifierChainReceiverFlat(file *scanner.File, navExpr uint32) uint32 {
	if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
		return 0
	}
	return file.FlatNamedChild(navExpr, 0)
}

func composeModifierChainContainsCall(chain []composeModifierCall, target string) bool {
	for _, call := range chain {
		if call.name == target {
			return true
		}
	}
	return false
}

func composeModifierChainSmallestDpFlat(file *scanner.File, chain []composeModifierCall) (float64, bool) {
	var minVal float64
	found := false

	for _, call := range chain {
		if !composeMinTouchTargetSizeCalls[call.name] {
			continue
		}
		value, ok := composeSmallestDpValueFlat(file, call.args)
		if !ok {
			continue
		}
		if !found || value < minVal {
			minVal = value
			found = true
		}
	}

	return minVal, found
}

func composeSmallestDpValueFlat(file *scanner.File, args uint32) (float64, bool) {
	if args == 0 {
		return 0, false
	}

	var minVal float64
	found := false
	for i := 0; i < file.FlatNamedChildCount(args); i++ {
		arg := file.FlatNamedChild(args, i)
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		value, ok := composeDpLiteralValueFlat(file, flatValueArgumentExpression(file, arg))
		if !ok {
			continue
		}
		if !found || value < minVal {
			minVal = value
			found = true
		}
	}

	return minVal, found
}

func composeDpLiteralValueFlat(file *scanner.File, expr uint32) (float64, bool) {
	if file == nil || expr == 0 {
		return 0, false
	}
	expr = flatUnwrapParenExpr(file, expr)
	if file.FlatType(expr) != "navigation_expression" || flatNavigationExpressionLastIdentifier(file, expr) != "dp" {
		return 0, false
	}
	var literal uint32
	file.FlatWalkAllNodes(expr, func(n uint32) {
		if literal != 0 {
			return
		}
		switch file.FlatType(n) {
		case "integer_literal", "real_literal":
			literal = n
		}
	})
	if literal == 0 {
		return 0, false
	}
	text := strings.TrimRight(file.FlatNodeText(literal), "fFdD")
	value, err := strconv.ParseFloat(strings.ReplaceAll(text, "_", ""), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// ---------------------------------------------------------------------------
// ComposeDecorativeImageContentDescription
// ---------------------------------------------------------------------------

type ComposeDecorativeImageContentDescriptionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposeDecorativeImageContentDescriptionRule) Confidence() float64 {
	return api.ConfidenceMedium
}

var composeImageCallNames = map[string]bool{
	"Image":      true,
	"AsyncImage": true,
}

// ---------------------------------------------------------------------------
// ComposeIconButtonMissingContentDescription
// ---------------------------------------------------------------------------

type ComposeIconButtonMissingContentDescriptionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposeIconButtonMissingContentDescriptionRule) Confidence() float64 {
	return api.ConfidenceMedium
}

var composeContentDescriptionCalls = map[string]bool{
	"Icon":       true,
	"IconButton": true,
	"Image":      true,
	"AsyncImage": true,
}

type composeContentDescriptionImportFacts struct {
	// aliases maps the simple-name of an imported symbol to its FQN.
	aliases map[string]string
	// wildcards is the set of wildcard import bases (without the
	// trailing `.*`) — kept here in pre-stripped form to match the
	// original API expected by callers.
	wildcards   map[string]bool
	hasRelevant bool
}

var composeContentDescriptionCanonicalFQNs = map[string][]string{
	"Icon": {
		"androidx.compose.material.Icon",
		"androidx.compose.material3.Icon",
	},
	"IconButton": {
		"androidx.compose.material.IconButton",
		"androidx.compose.material3.IconButton",
	},
	"Image": {
		"androidx.compose.foundation.Image",
	},
	"AsyncImage": {
		"coil.compose.AsyncImage",
	},
}

// composeCallSetsContentDescription reports whether a call passes contentDescription by name or,
// for Icon/Image/AsyncImage, as its second positional argument.
func composeCallSetsContentDescription(file *scanner.File, call uint32) bool {
	_, args := flatCallExpressionParts(file, call)
	if args == 0 {
		return false
	}
	if flatNamedValueArgument(file, args, "contentDescription") != 0 {
		return true
	}
	switch flatCallNameAny(file, call) {
	case "Icon", "Image", "AsyncImage":
		return flatPositionalValueArgument(file, args, 1) != 0
	}
	return false
}

// composeSameFileHelperTakesContentDescription reports whether the file declares a function
// called name with a contentDescription-style parameter that it can forward to an Icon.
func composeSameFileHelperTakesContentDescription(file *scanner.File, name string) bool {
	found := false
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if found || extractIdentifierFlat(file, fn) != name {
			return
		}
		for _, param := range flatFunctionParameterNames(file, fn) {
			if strings.HasSuffix(strings.ToLower(param), "contentdescription") {
				found = true
				return
			}
		}
	})
	return found
}

// composeSubtreeSetsContentDescription reports whether any call under root sets a content
// description directly or through a same-file helper that takes one.
func composeSubtreeSetsContentDescription(file *scanner.File, root uint32) bool {
	found := composeSubtreeAssignsSemanticsDescription(file, root) || composeSubtreeInvokesSlotParameter(file, root)
	if found {
		return true
	}
	file.FlatWalkNodes(root, "call_expression", func(call uint32) {
		if found {
			return
		}
		if composeCallSetsContentDescription(file, call) {
			found = true
			return
		}
		if name := flatCallNameAny(file, call); name != "" && name != "IconButton" &&
			composeSameFileHelperTakesContentDescription(file, name) {
			found = true
		}
	})
	return found
}

// composeSubtreeAssignsSemanticsDescription reports `contentDescription = ...` inside a semantics block.
func composeSubtreeAssignsSemanticsDescription(file *scanner.File, root uint32) bool {
	found := false
	file.FlatWalkNodes(root, "assignment", func(assign uint32) {
		if found || file.FlatNamedChildCount(assign) == 0 {
			return
		}
		if file.FlatNodeTextEquals(file.FlatNamedChild(assign, 0), "contentDescription") &&
			hasAncestorCallNamedFlat(file, assign, "semantics") {
			found = true
		}
	})
	return found
}

// composeSubtreeInvokesSlotParameter reports a call to a function-typed parameter of the enclosing
// function (`icon()`), whose caller owns the description.
func composeSubtreeInvokesSlotParameter(file *scanner.File, root uint32) bool {
	fn, ok := flatEnclosingFunction(file, root)
	if !ok {
		return false
	}
	types := composeParamTypeTexts(file, fn)
	found := false
	file.FlatWalkNodes(root, "call_expression", func(call uint32) {
		if found || flatReceiverNameFromCall(file, call) != "" {
			return
		}
		if typ, ok := types[flatCallNameAny(file, call)]; ok && strings.Contains(typ, "->") {
			found = true
		}
	})
	return found
}

func composeContentDescriptionCallConfirmed(file *scanner.File, idx uint32, name string) bool {
	if file == nil || idx == 0 || name == "" {
		return false
	}
	navText := flatCallNavigationTextAny(file, idx)
	if navText == "" {
		return false
	}
	if strings.Contains(navText, ".") {
		for _, fqn := range composeContentDescriptionCanonicalFQNs[name] {
			if navText == fqn {
				return true
			}
		}
		return false
	}
	facts := composeContentDescriptionImports(file)
	if imported := facts.aliases[name]; imported != "" {
		for _, fqn := range composeContentDescriptionCanonicalFQNs[name] {
			if imported == fqn {
				return true
			}
		}
	}
	for _, fqn := range composeContentDescriptionCanonicalFQNs[name] {
		pkg := fqn[:strings.LastIndex(fqn, ".")]
		if facts.wildcards[pkg] {
			return true
		}
	}
	return false
}

func composeContentDescriptionFileMayUse(file *scanner.File) bool {
	return composeContentDescriptionImports(file).hasRelevant
}

func flatCallNavigationTextAny(file *scanner.File, idx uint32) string {
	if navExpr, _ := flatCallExpressionParts(file, idx); navExpr != 0 {
		return strings.TrimSpace(file.FlatNodeText(navExpr))
	}
	if name := flatCallExpressionName(file, idx); name != "" {
		return name
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "call_expression" {
			return flatCallNavigationTextAny(file, child)
		}
	}
	return ""
}

func composeHasConfirmedIconButtonAncestor(file *scanner.File, idx uint32) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		if file.FlatType(parent) != "call_expression" {
			continue
		}
		if flatCallNameAny(file, parent) == "IconButton" && composeContentDescriptionCallConfirmed(file, parent, "IconButton") {
			return true
		}
	}
	return false
}

// composeContentDescriptionImports is computed once per file (#944): the
// rule asks for it from every call it visits.
func composeContentDescriptionImports(file *scanner.File) composeContentDescriptionImportFacts {
	return filefacts.FileFact(fileFactsCache(), file, slotComposeContentDescImports, func() composeContentDescriptionImportFacts {
		return computeComposeContentDescriptionImports(file)
	})
}

func computeComposeContentDescriptionImports(file *scanner.File) composeContentDescriptionImportFacts {
	imports := fileFactsCache().Imports(file)
	facts := composeContentDescriptionImportFacts{
		aliases:   imports.Aliases,
		wildcards: make(map[string]bool, len(imports.Wildcards)),
	}
	for w := range imports.Wildcards {
		pkg := strings.TrimSuffix(w, ".*")
		facts.wildcards[pkg] = true
		if composeContentDescriptionRelevantPackage(pkg) {
			facts.hasRelevant = true
		}
	}
	for _, fqn := range imports.Aliases {
		if composeContentDescriptionRelevantFQN(fqn) {
			facts.hasRelevant = true
			break
		}
	}
	if !facts.hasRelevant {
		content := file.Content
		for _, fqns := range composeContentDescriptionCanonicalFQNs {
			for _, fqn := range fqns {
				if bytes.Contains(content, []byte(fqn)) {
					facts.hasRelevant = true
					break
				}
			}
			if facts.hasRelevant {
				break
			}
		}
	}
	return facts
}

func composeContentDescriptionRelevantFQN(imp string) bool {
	for _, fqns := range composeContentDescriptionCanonicalFQNs {
		for _, fqn := range fqns {
			if imp == fqn {
				return true
			}
		}
	}
	return false
}

func composeContentDescriptionRelevantPackage(pkg string) bool {
	for _, fqns := range composeContentDescriptionCanonicalFQNs {
		for _, fqn := range fqns {
			if pkg == fqn[:strings.LastIndex(fqn, ".")] {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// ComposeRawTextLiteral
// ---------------------------------------------------------------------------

type ComposeRawTextLiteralRule struct {
	FlatDispatchBase
	BaseRule
	CustomPreviewWildcard bool
	CustomPreviewPrefixes []string
	// NonProductionPathMarkers are extra path substrings that mark sources as not user-facing.
	NonProductionPathMarkers []string
}

func (r *ComposeRawTextLiteralRule) Confidence() float64 { return api.ConfidenceMedium }

func composeRawTextLiteralNonProductionFile(path string, extraMarkers []string) bool {
	if scanner.IsTestFile(path) {
		return true
	}
	lower := strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
	// Debug, playground and sandbox sources do not ship user-facing strings.
	for _, marker := range []string{"playground", "sandbox", "/debug"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	for _, marker := range extraMarkers {
		if marker != "" && strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}
	return strings.Contains(lower, "/samples/") ||
		strings.Contains(lower, "/sample/") ||
		strings.Contains(lower, "/demos/") ||
		strings.Contains(lower, "/demo/") ||
		strings.Contains(lower, "benchmark") ||
		strings.Contains(lower, "/ui-tooling/") ||
		strings.Contains(lower, "/test-utils/") ||
		strings.Contains(lower, "-test-utils/")
}

// ---------------------------------------------------------------------------
// ComposeSemanticsMissingRole
// ---------------------------------------------------------------------------

type ComposeSemanticsMissingRoleRule struct {
	FlatDispatchBase
	BaseRule
	CustomPreviewWildcard bool
	CustomPreviewPrefixes []string
}

func (r *ComposeSemanticsMissingRoleRule) Confidence() float64 { return api.ConfidenceMedium }

var composeInteractionModifiers = map[string]bool{
	"clickable":  true,
	"toggleable": true,
	"selectable": true,
}

func flatNamedBooleanArgumentIsFalse(file *scanner.File, args uint32, name string) bool {
	arg := flatNamedValueArgument(file, args, name)
	if arg == 0 {
		return false
	}
	expr := flatUnwrapParenExpr(file, flatValueArgumentExpression(file, arg))
	return expr != 0 && file.FlatType(expr) == "boolean_literal" && strings.TrimSpace(file.FlatNodeText(expr)) == "false"
}

// composeIndicationIsNullFlat reports whether the call passes `indication = null`, the shape of an invisible scrim click.
func composeIndicationIsNullFlat(file *scanner.File, args uint32) bool {
	arg := flatNamedValueArgument(file, args, "indication")
	if arg == 0 {
		return false
	}
	// The `null` token is unnamed, so read it from the argument text.
	_, value, ok := strings.Cut(file.FlatNodeText(arg), "=")
	return ok && strings.TrimSpace(value) == "null"
}

// ---------------------------------------------------------------------------
// ComposeTextFieldMissingLabel
// ---------------------------------------------------------------------------

type ComposeTextFieldMissingLabelRule struct {
	FlatDispatchBase
	BaseRule
	// NonProductionPathMarkers are extra path substrings that mark sources as not user-facing.
	NonProductionPathMarkers []string
}

func (r *ComposeTextFieldMissingLabelRule) Confidence() float64 { return api.ConfidenceMedium }

var composeTextFieldCalls = map[string]bool{
	"TextField":         true,
	"OutlinedTextField": true,
}

// composeTextFieldCallResolvesToMaterial reports whether an unqualified call resolves through imports to the
// Material TextField family, inside a @Composable and not shadowed by a local declaration.
func composeTextFieldCallResolvesToMaterial(file *scanner.File, call uint32, name string) bool {
	if navExpr, _ := flatCallExpressionParts(file, call); navExpr != 0 {
		return false
	}
	if scanner.IsTestFile(file.Path) || isAndroidTestSupportArtifactSource(file.Path) {
		return false
	}
	inComposable := false
	for p, ok := file.FlatParent(call); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "function_declaration" && flatHasAnnotationNamed(file, p, "Composable") {
			inComposable = true
			break
		}
	}
	if !inComposable {
		return false
	}
	// A same-file class or function with this name shadows the import.
	declared := false
	file.FlatWalkAllNodes(0, func(n uint32) {
		switch file.FlatType(n) {
		case "class_declaration", "function_declaration":
			if extractIdentifierFlat(file, n) == name {
				declared = true
			}
		}
	})
	if declared {
		return false
	}
	imports := fileFactsCache().Imports(file)
	if fqn, ok := imports.Aliases[name]; ok {
		return fqn == "androidx.compose.material3."+name || fqn == "androidx.compose.material."+name
	}
	_, m3 := imports.Wildcards["androidx.compose.material3.*"]
	_, m2 := imports.Wildcards["androidx.compose.material.*"]
	return m3 || m2
}

// composeIsTestSourceOrFakeAt reports whether idx sits in a test source set or a Fake/Test class.
func composeIsTestSourceOrFakeAt(file *scanner.File, idx uint32) bool {
	if scanner.IsTestFile(file.Path) || isAndroidTestSupportArtifactSource(file.Path) {
		return true
	}
	name := flatEnclosingClassName(file, idx)
	return strings.HasPrefix(name, "Fake") || strings.Contains(name, "Test")
}

func hasSiblingTextCall(file *scanner.File, parent uint32, self uint32) bool {
	for child := file.FlatFirstChild(parent); child != 0; child = file.FlatNextSib(child) {
		if child == self {
			continue
		}
		if file.FlatType(child) == "call_expression" {
			n := flatCallName(file, child)
			if n == "Text" {
				return true
			}
		}
		for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
			if file.FlatType(gc) == "call_expression" && flatCallName(file, gc) == "Text" {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// ToastForAccessibilityAnnouncement
// ---------------------------------------------------------------------------

type ToastForAccessibilityAnnouncementRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ToastForAccessibilityAnnouncementRule) Confidence() float64 { return api.ConfidenceMedium }

var a11yFunctionPatterns = []string{
	"accessibility", "announce", "a11y",
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func findOutermostModifierChainCall(file *scanner.File, idx uint32) uint32 {
	current := composeAscendModifierChain(file, idx)
	// A modifier passed to `.then(...)` belongs to the chain of that `then` call.
	for {
		owner := composeThenCallOwning(file, current)
		if owner == 0 {
			return current
		}
		current = composeAscendModifierChain(file, owner)
	}
}

// composeAscendModifierChain climbs from a call to the outermost call or navigation of the dotted chain it sits in.
func composeAscendModifierChain(file *scanner.File, idx uint32) uint32 {
	current := idx
	for {
		parent, ok := file.FlatParent(current)
		if !ok {
			return current
		}
		pt := file.FlatType(parent)
		if pt == "navigation_expression" || pt == "call_expression" {
			current = parent
			continue
		}
		return current
	}
}

// composeThenCallOwning returns the `x.then(...)` call whose argument is node, looking through if, when and
// parentheses around the argument expression. It returns 0 when node is not a `then` argument.
func composeThenCallOwning(file *scanner.File, node uint32) uint32 {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "if_expression", "when_expression", "when_entry", "control_structure_body", "parenthesized_expression",
			"statements", "value_argument", "value_arguments", "call_suffix":
			continue
		case "call_expression":
			navExpr, _ := flatCallExpressionParts(file, p)
			if navExpr != 0 && flatNavigationExpressionLastIdentifier(file, navExpr) == "then" {
				return p
			}
			return 0
		default:
			return 0
		}
	}
	return 0
}

func flatCallName(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "simple_identifier":
			return file.FlatNodeText(child)
		case "navigation_expression":
			return flatNavigationExpressionLastIdentifier(file, child)
		}
	}
	return ""
}

func flatFunctionName(file *scanner.File, fn uint32) string {
	if file == nil || fn == 0 {
		return ""
	}
	for child := file.FlatFirstChild(fn); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "simple_identifier" {
			return file.FlatNodeText(child)
		}
	}
	return ""
}

// composeInsideInspectionModeBranch reports a node in the branch of an if that runs only when
// LocalInspectionMode.current (or a local bound to it) is true, which is preview-only UI.
func composeInsideInspectionModeBranch(file *scanner.File, idx uint32) bool {
	child := idx
	for {
		cur, ok := file.FlatParent(child)
		if !ok {
			return false
		}
		switch file.FlatType(cur) {
		case "function_declaration":
			return false
		case "if_expression":
			cond, thenBody, elseBody := composeIfParts(file, cur)
			if cond != 0 {
				text := strings.TrimSpace(file.FlatNodeText(cond))
				negated := strings.HasPrefix(text, "!")
				text = strings.TrimSpace(strings.TrimPrefix(text, "!"))
				if composeIsInspectionModeRead(file, cur, text) &&
					(child == thenBody && !negated || child == elseBody && negated) {
					return true
				}
			}
		}
		child = cur
	}
}

// composeIfParts returns the condition and the then/else bodies of an if_expression.
func composeIfParts(file *scanner.File, ifExpr uint32) (cond, thenBody, elseBody uint32) {
	seenElse := false
	for c := file.FlatFirstChild(ifExpr); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "else":
			seenElse = true
		case "control_structure_body":
			if seenElse {
				elseBody = c
			} else if thenBody == 0 {
				thenBody = c
			}
		case "(", ")", "if":
		default:
			if cond == 0 && file.FlatIsNamed(c) && thenBody == 0 {
				cond = c
			}
		}
	}
	return cond, thenBody, elseBody
}

// composeIsInspectionModeRead reports `LocalInspectionMode.current` or a local val bound to it.
func composeIsInspectionModeRead(file *scanner.File, from uint32, text string) bool {
	if text == "LocalInspectionMode.current" {
		return true
	}
	fn, ok := flatEnclosingFunction(file, from)
	if !ok || text == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(fn, "property_declaration", func(prop uint32) {
		if !found && extractIdentifierFlat(file, prop) == text && strings.Contains(file.FlatNodeText(prop), "LocalInspectionMode.current") {
			found = true
		}
	})
	return found
}

// composeDebugToolingPath reports debug, playground and sandbox sources, which are not user-facing.
// extraMarkers are configured path substrings that count the same way.
func composeDebugToolingPath(path string, extraMarkers []string) bool {
	slash := "/" + strings.ToLower(strings.TrimPrefix(filepath.ToSlash(path), "/"))
	for _, marker := range []string{"/debug", "/playground/", "/sandbox/"} {
		if strings.Contains(slash, marker) {
			return true
		}
	}
	for _, marker := range extraMarkers {
		if marker != "" && strings.Contains(slash, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

var composeSmallFixedSizeRe = regexp.MustCompile(`\b(?:size|width)\(\s*(\d+(?:\.\d+)?)\.dp\s*\)`)

// composeTextFieldIsDescribedGridCell reports a text field that is one cell of a repeated indexed layout
// (such as a verification code) whose cell carries a semantics contentDescription. The description
// speaks for the cell, and a per-cell label would repeat on every cell.
func composeTextFieldIsDescribedGridCell(file *scanner.File, call, args uint32) bool {
	modifier := flatNamedValueArgument(file, args, "modifier")
	described := modifier != 0 && strings.Contains(file.FlatNodeText(modifier), "contentDescription")
	repeated := false
walk:
	for p, ok := file.FlatParent(call); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "function_declaration", "class_declaration", "source_file":
			break walk
		case "lambda_literal":
			if composeIsRepeatedCellLambda(file, p) {
				repeated = true
			}
		case "call_expression":
			if _, enclosingArgs := flatCallExpressionParts(file, p); enclosingArgs != 0 &&
				strings.Contains(file.FlatNodeText(enclosingArgs), "contentDescription") {
				described = true
			}
		}
	}
	if !described {
		return false
	}
	if repeated {
		return true
	}
	single := flatNamedValueArgument(file, args, "singleLine")
	if single == 0 || modifier == 0 || !strings.HasSuffix(strings.TrimSpace(file.FlatNodeText(single)), "true") {
		return false
	}
	m := composeSmallFixedSizeRe.FindStringSubmatch(file.FlatNodeText(modifier))
	if m == nil {
		return false
	}
	dp, err := strconv.ParseFloat(m[1], 64)
	return err == nil && dp <= 64
}

var (
	composeSingleCharInputRe = regexp.MustCompile(`\b(?:take|takeLast)\(\s*1\s*\)|\.length\s*(?:<=|==|<)\s*[12]\b|\.length\s*>\s*1\b|\bmaxLength\s*=\s*1\b`)
	composeCodeBoxNameRe     = regexp.MustCompile(`(?i)otp|pin|digit|passcode|verification|codebox|codecell`)
)

// composeTextFieldIsSingleCharBox reports a field that holds one character of a code (OTP or PIN box): its
// handlers cap the input at one character, or its enclosing composable is named for a code digit.
func composeTextFieldIsSingleCharBox(file *scanner.File, call uint32) bool {
	if composeSingleCharInputRe.MatchString(file.FlatNodeText(call)) {
		return true
	}
	if fn, ok := flatEnclosingFunction(file, call); ok {
		return composeCodeBoxNameRe.MatchString(flatFunctionName(file, fn))
	}
	return false
}

// composeIsRepeatedCellLambda reports a lambda run once per index: repeat, forEachIndexed, mapIndexed,
// or map/forEach over a range such as `(0 until n)`.
func composeIsRepeatedCellLambda(file *scanner.File, lambda uint32) bool {
	call, ok := file.FlatParent(lambda)
	for ok && file.FlatType(call) != "call_expression" {
		call, ok = file.FlatParent(call)
	}
	if !ok {
		return false
	}
	switch flatCallNameAny(file, call) {
	case "repeat", "forEachIndexed", "mapIndexed", "itemsIndexed":
		return true
	case "map", "forEach":
		start, end := file.FlatStartByte(call), file.FlatStartByte(lambda)
		if end <= start || int(end) > len(file.Content) {
			return false
		}
		head := string(file.Content[start:end])
		return strings.Contains(head, " until ") || strings.Contains(head, "..")
	}
	return false
}

// composePassesPositionalRole reports an unlabeled `Role.X` argument, as in clickable(true, null, Role.Button) { }.
func composePassesPositionalRole(file *scanner.File, args uint32) bool {
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" || flatHasValueArgumentLabel(file, arg) {
			continue
		}
		expr := flatValueArgumentExpression(file, arg)
		if expr == 0 || file.FlatType(expr) != "navigation_expression" {
			continue
		}
		segments := flatNavigationChainIdentifiers(file, expr)
		if len(segments) >= 2 && segments[len(segments)-2] == "Role" {
			return true
		}
	}
	return false
}

// composeTextCallMayBeCompose reports false when the file imports or declares a non-AndroidX Text, such as a design-system wrapper.
func composeTextCallMayBeCompose(file *scanner.File) bool {
	if fqn, ok := fileFactsCache().Imports(file).Aliases["Text"]; ok && !strings.HasPrefix(fqn, "androidx.") {
		return false
	}
	declared := false
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if extractIdentifierFlat(file, fn) == "Text" {
			declared = true
		}
	})
	return !declared
}

// composeRawTextLiteralText returns the literal text of a string, excluding interpolated expressions.
func composeRawTextLiteralText(file *scanner.File, expr uint32) string {
	interpolated := false
	var b strings.Builder
	for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
		switch t := file.FlatType(child); {
		case t == "string_content" || t == "string_fragment":
			b.WriteString(file.FlatNodeText(child))
		case strings.Contains(t, "interpolat"):
			interpolated = true
		}
	}
	if !interpolated {
		return stringLiteralContent(file, expr)
	}
	return b.String()
}
