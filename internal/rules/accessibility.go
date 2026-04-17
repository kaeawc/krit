package rules

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
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
func (r *AnimatorDurationIgnoresScaleRule) Confidence() float64 { return 0.75 }

func (r *AnimatorDurationIgnoresScaleRule) NodeTypes() []string {
	return []string{"call_expression", "assignment"}
}

func (r *AnimatorDurationIgnoresScaleRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	if animatorDurationScaleReferenced(file) {
		return nil
	}

	switch file.FlatType(idx) {
	case "call_expression":
		return r.checkSetDurationCallFlat(idx, file)
	case "assignment":
		return r.checkDurationAssignmentFlat(idx, file)
	default:
		return nil
	}
}

func animatorDurationScaleReferenced(file *scanner.File) bool {
	return strings.Contains(string(file.Content), "ANIMATOR_DURATION_SCALE")
}

func (r *AnimatorDurationIgnoresScaleRule) checkSetDurationCallFlat(idx uint32, file *scanner.File) []scanner.Finding {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "setDuration" {
		return nil
	}
	if !animatorReceiverLooksLikeAnimatorFlat(file, navExpr) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Animator duration ignores the system animation scale. Read Settings.Global.ANIMATOR_DURATION_SCALE and scale the duration before starting the animation.",
	)}
}

func (r *AnimatorDurationIgnoresScaleRule) checkDurationAssignmentFlat(idx uint32, file *scanner.File) []scanner.Finding {
	if file.FlatNamedChildCount(idx) == 0 {
		return nil
	}
	lhs := file.FlatNamedChild(idx, 0)

	target := strings.TrimSpace(file.FlatNodeText(lhs))
	if idx := strings.LastIndex(target, "."); idx >= 0 {
		target = target[idx+1:]
	}
	if target != "duration" {
		return nil
	}
	if !assignmentInsideAnimatorContextFlat(file, idx) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Animator duration ignores the system animation scale. Read Settings.Global.ANIMATOR_DURATION_SCALE and scale the duration before starting the animation.",
	)}
}

func animatorReceiverLooksLikeAnimatorFlat(file *scanner.File, navExpr uint32) bool {
	if navExpr == 0 || file.FlatNamedChildCount(navExpr) == 0 {
		return false
	}

	receiver := file.FlatNamedChild(navExpr, 0)
	return kotlinTextLooksLikeAnimator(file.FlatNodeText(receiver))
}

func assignmentInsideAnimatorContextFlat(file *scanner.File, idx uint32) bool {
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		text := file.FlatNodeText(parent)
		if kotlinTextLooksLikeAnimator(text) {
			return true
		}
		switch file.FlatType(parent) {
		case "function_declaration", "class_declaration", "source_file":
			return false
		}
	}
	return false
}

func kotlinTextLooksLikeAnimator(text string) bool {
	return strings.Contains(text, "ValueAnimator.") || strings.Contains(text, "ObjectAnimator.")
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
func (r *ComposeClickableWithoutMinTouchTargetRule) Confidence() float64 { return 0.75 }

func (r *ComposeClickableWithoutMinTouchTargetRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *ComposeClickableWithoutMinTouchTargetRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 || flatNavigationExpressionLastIdentifier(file, navExpr) != "clickable" {
		return nil
	}

	chain, rootedAtModifier := composeModifierCallChainFlat(file, composeModifierChainReceiverFlat(file, navExpr))
	if !rootedAtModifier || composeModifierChainContainsCall(chain, "minimumInteractiveComponentSize") {
		return nil
	}

	minDp, hasExplicitSize := composeModifierChainSmallestDpFlat(file, chain)
	if !hasExplicitSize || minDp >= 48 {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"clickable Compose modifier has a touch target below 48.dp; use at least 48.dp or add minimumInteractiveComponentSize().",
	)}
}

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
	composeDpLiteralRe = regexp.MustCompile(`(-?\d+(?:\.\d+)?)\s*\.dp\b`)
)

func composeModifierCallChainFlat(file *scanner.File, idx uint32) ([]composeModifierCall, bool) {
	if idx == 0 {
		return nil, false
	}

	switch file.FlatType(idx) {
	case "call_expression":
		navExpr, args := flatCallExpressionParts(file, idx)
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
		return nil, flatNavigationExpressionLastIdentifier(file, idx) == "Modifier"
	case "parenthesized_expression":
		if file.FlatNamedChildCount(idx) == 1 {
			return composeModifierCallChainFlat(file, file.FlatNamedChild(idx, 0))
		}
		return nil, false
	case "simple_identifier":
		return nil, file.FlatNodeTextEquals(idx, "Modifier")
	default:
		return nil, false
	}
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
	var min float64
	found := false

	for _, call := range chain {
		if !composeMinTouchTargetSizeCalls[call.name] {
			continue
		}
		value, ok := composeSmallestDpValueFlat(file, call.args)
		if !ok {
			continue
		}
		if !found || value < min {
			min = value
			found = true
		}
	}

	return min, found
}

func composeSmallestDpValueFlat(file *scanner.File, args uint32) (float64, bool) {
	if args == 0 {
		return 0, false
	}

	var min float64
	found := false
	for i := 0; i < file.FlatNamedChildCount(args); i++ {
		arg := file.FlatNamedChild(args, i)
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		value, ok := composeParseDpLiteral(file.FlatNodeText(arg))
		if !ok {
			continue
		}
		if !found || value < min {
			min = value
			found = true
		}
	}

	return min, found
}

func composeParseDpLiteral(text string) (float64, bool) {
	match := composeDpLiteralRe.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0, false
	}

	value, err := strconv.ParseFloat(match[1], 64)
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

func (r *ComposeDecorativeImageContentDescriptionRule) Confidence() float64 { return 0.75 }

func (r *ComposeDecorativeImageContentDescriptionRule) NodeTypes() []string {
	return []string{"call_expression"}
}

var composeImageCallNames = map[string]bool{
	"Image":      true,
	"AsyncImage": true,
}

func (r *ComposeDecorativeImageContentDescriptionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallName(file, idx)
	if !composeImageCallNames[name] {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}

	cdArg := flatNamedValueArgument(file, args, "contentDescription")
	if cdArg == 0 {
		return nil
	}
	argText := strings.TrimSpace(file.FlatNodeText(cdArg))
	if !strings.Contains(argText, "null") {
		return nil
	}

	callText := file.FlatNodeText(idx)
	if strings.Contains(callText, "clearAndSetSemantics") ||
		strings.Contains(callText, "invisibleToUser") {
		return nil
	}

	modArg := flatNamedValueArgument(file, args, "modifier")
	if modArg != 0 {
		modText := file.FlatNodeText(modArg)
		if strings.Contains(modText, "clearAndSetSemantics") ||
			strings.Contains(modText, "invisibleToUser") {
			return nil
		}
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Decorative image with `contentDescription = null` should use `Modifier.clearAndSetSemantics {}` or `semantics { invisibleToUser() }` to hide from TalkBack.",
	)}
}

// ---------------------------------------------------------------------------
// ComposeIconButtonMissingContentDescription
// ---------------------------------------------------------------------------

type ComposeIconButtonMissingContentDescriptionRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposeIconButtonMissingContentDescriptionRule) Confidence() float64 { return 0.75 }

func (r *ComposeIconButtonMissingContentDescriptionRule) NodeTypes() []string {
	return []string{"call_expression"}
}

var composeContentDescriptionCalls = map[string]bool{
	"Icon":       true,
	"IconButton": true,
	"Image":      true,
	"AsyncImage": true,
}

func (r *ComposeIconButtonMissingContentDescriptionRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallName(file, idx)
	if !composeContentDescriptionCalls[name] {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)

	if name == "IconButton" {
		if args != 0 && flatNamedValueArgument(file, args, "contentDescription") != 0 {
			return nil
		}
		return r.checkIconButtonContent(idx, file)
	}

	if args == 0 {
		return nil
	}

	cdArg := flatNamedValueArgument(file, args, "contentDescription")
	if cdArg != 0 {
		argText := strings.TrimSpace(file.FlatNodeText(cdArg))
		if !strings.Contains(argText, "= null") {
			return nil
		}
		modArg := flatNamedValueArgument(file, args, "modifier")
		if modArg != 0 {
			modText := file.FlatNodeText(modArg)
			if strings.Contains(modText, "invisibleToUser") || strings.Contains(modText, "clearAndSetSemantics") {
				return nil
			}
		}
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		name+" is missing `contentDescription`. Set a description for accessibility or mark as decorative.",
	)}
}

func (r *ComposeIconButtonMissingContentDescriptionRule) checkIconButtonContent(idx uint32, file *scanner.File) []scanner.Finding {
	text := file.FlatNodeText(idx)
	if strings.Contains(text, "contentDescription") {
		return nil
	}
	if parent, ok := file.FlatParent(idx); ok && file.FlatType(parent) == "call_expression" {
		parentText := file.FlatNodeText(parent)
		if strings.Contains(parentText, "contentDescription") {
			return nil
		}
	}
	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"IconButton's Icon is missing `contentDescription`. Set a description for accessibility.",
	)}
}

// ---------------------------------------------------------------------------
// ComposeRawTextLiteral
// ---------------------------------------------------------------------------

type ComposeRawTextLiteralRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposeRawTextLiteralRule) Confidence() float64 { return 0.75 }

func (r *ComposeRawTextLiteralRule) NodeTypes() []string {
	return []string{"call_expression"}
}

func (r *ComposeRawTextLiteralRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallName(file, idx)
	if name != "Text" {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}

	firstArg := flatPositionalValueArgument(file, args, 0)
	if firstArg == 0 {
		return nil
	}
	argText := strings.TrimSpace(file.FlatNodeText(firstArg))
	if !strings.HasPrefix(argText, "\"") {
		return nil
	}

	fn, ok := flatEnclosingFunction(file, idx)
	if !ok {
		return nil
	}
	if flatHasAnnotationNamed(file, fn, "Preview") {
		return nil
	}
	fnName := flatFunctionName(file, fn)
	if strings.Contains(fnName, "Preview") || strings.Contains(fnName, "Sample") {
		return nil
	}
	if strings.HasSuffix(file.Path, "Preview.kt") || strings.HasSuffix(file.Path, "Sample.kt") {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Text() uses a hardcoded string literal. Use `stringResource()` for internationalization and accessibility.",
	)}
}

// ---------------------------------------------------------------------------
// ComposeSemanticsMissingRole
// ---------------------------------------------------------------------------

type ComposeSemanticsMissingRoleRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposeSemanticsMissingRoleRule) Confidence() float64 { return 0.75 }

func (r *ComposeSemanticsMissingRoleRule) NodeTypes() []string {
	return []string{"call_expression"}
}

var composeInteractionModifiers = map[string]bool{
	"clickable":  true,
	"toggleable": true,
	"selectable": true,
}

func (r *ComposeSemanticsMissingRoleRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	navExpr, args := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return nil
	}
	name := flatNavigationExpressionLastIdentifier(file, navExpr)
	if !composeInteractionModifiers[name] {
		return nil
	}

	_, rootedAtModifier := composeModifierCallChainFlat(file, composeModifierChainReceiverFlat(file, navExpr))
	if !rootedAtModifier {
		return nil
	}

	if args != 0 && flatNamedValueArgument(file, args, "role") != 0 {
		return nil
	}

	outerCall := findOutermostModifierChainCall(file, idx)
	if outerCall != 0 {
		fullText := file.FlatNodeText(outerCall)
		if strings.Contains(fullText, "semantics") && strings.Contains(fullText, "role") {
			return nil
		}
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Modifier."+name+" without an explicit `role`. Add `role = Role.X` or a `Modifier.semantics { role = ... }` for screen readers.",
	)}
}

// ---------------------------------------------------------------------------
// ComposeTextFieldMissingLabel
// ---------------------------------------------------------------------------

type ComposeTextFieldMissingLabelRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposeTextFieldMissingLabelRule) Confidence() float64 { return 0.75 }

func (r *ComposeTextFieldMissingLabelRule) NodeTypes() []string {
	return []string{"call_expression"}
}

var composeTextFieldCalls = map[string]bool{
	"TextField":         true,
	"OutlinedTextField": true,
}

func (r *ComposeTextFieldMissingLabelRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	name := flatCallName(file, idx)
	if !composeTextFieldCalls[name] {
		return nil
	}

	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return nil
	}

	if flatNamedValueArgument(file, args, "label") != 0 {
		return nil
	}
	if flatNamedValueArgument(file, args, "placeholder") != 0 {
		return nil
	}

	parent, ok := file.FlatParent(idx)
	if ok && hasSiblingTextCall(file, parent, idx) {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		name+" is missing a `label` parameter. Add a label for accessibility.",
	)}
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

func (r *ToastForAccessibilityAnnouncementRule) Confidence() float64 { return 0.75 }

func (r *ToastForAccessibilityAnnouncementRule) NodeTypes() []string {
	return []string{"call_expression"}
}

var a11yFunctionPatterns = []string{
	"accessibility", "announce", "a11y",
}

func (r *ToastForAccessibilityAnnouncementRule) CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding {
	navExpr, _ := flatCallExpressionParts(file, idx)
	if navExpr == 0 {
		return nil
	}

	callName := flatNavigationExpressionLastIdentifier(file, navExpr)
	if callName != "makeText" {
		return nil
	}

	receiver := file.FlatNamedChild(navExpr, 0)
	if receiver == 0 || !file.FlatNodeTextEquals(receiver, "Toast") {
		return nil
	}

	fn, ok := flatEnclosingFunction(file, idx)
	if !ok {
		return nil
	}
	fnName := strings.ToLower(flatFunctionName(file, fn))
	isA11yContext := false
	for _, pattern := range a11yFunctionPatterns {
		if strings.Contains(fnName, pattern) {
			isA11yContext = true
			break
		}
	}
	if !isA11yContext {
		return nil
	}

	return []scanner.Finding{r.Finding(
		file,
		file.FlatRow(idx)+1,
		file.FlatCol(idx)+1,
		"Toast used in an accessibility context. Use `View.announceForAccessibility()` or `AccessibilityManager` instead.",
	)}
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
