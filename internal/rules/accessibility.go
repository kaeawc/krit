package rules

import (
	"bytes"
	"strconv"
	"strings"

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

func composeContentDescriptionImports(file *scanner.File) composeContentDescriptionImportFacts {
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
}

func (r *ComposeRawTextLiteralRule) Confidence() float64 { return api.ConfidenceMedium }

func composeRawTextLiteralNonProductionFile(path string) bool {
	if scanner.IsTestFile(path) {
		return true
	}
	lower := strings.ToLower(path)
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

// ---------------------------------------------------------------------------
// ComposeTextFieldMissingLabel
// ---------------------------------------------------------------------------

type ComposeTextFieldMissingLabelRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposeTextFieldMissingLabelRule) Confidence() float64 { return api.ConfidenceMedium }

var composeTextFieldCalls = map[string]bool{
	"TextField":         true,
	"OutlinedTextField": true,
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
