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
func animatorDurationScaleReferenced(file *scanner.File) bool {
	return strings.Contains(string(file.Content), "ANIMATOR_DURATION_SCALE")
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

func (r *ComposeIconButtonMissingContentDescriptionRule) Confidence() float64 { return 0.75 }
var composeContentDescriptionCalls = map[string]bool{
	"Icon":       true,
	"IconButton": true,
	"Image":      true,
	"AsyncImage": true,
}
// ---------------------------------------------------------------------------
// ComposeRawTextLiteral
// ---------------------------------------------------------------------------

type ComposeRawTextLiteralRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposeRawTextLiteralRule) Confidence() float64 { return 0.75 }
// ---------------------------------------------------------------------------
// ComposeSemanticsMissingRole
// ---------------------------------------------------------------------------

type ComposeSemanticsMissingRoleRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposeSemanticsMissingRoleRule) Confidence() float64 { return 0.75 }
var composeInteractionModifiers = map[string]bool{
	"clickable":  true,
	"toggleable": true,
	"selectable": true,
}
// ---------------------------------------------------------------------------
// ComposeTextFieldMissingLabel
// ---------------------------------------------------------------------------

type ComposeTextFieldMissingLabelRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposeTextFieldMissingLabelRule) Confidence() float64 { return 0.75 }
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

func (r *ToastForAccessibilityAnnouncementRule) Confidence() float64 { return 0.75 }
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
