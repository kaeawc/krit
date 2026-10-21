package rules

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func rangeOracleCallTarget(ctx *api.Context, call uint32) string {
	if ctx.Resolver == nil || ctx.File == nil {
		return ""
	}
	cr, ok := ctx.Resolver.(*oracle.CompositeResolver)
	if !ok {
		return ""
	}
	lookup := cr.Oracle()
	if lookup == nil {
		return ""
	}
	return oracleLookupCallTargetFlat(lookup, ctx.File, call)
}

func rangeFrameworkSpecsForTarget(target, callName string) []rangeArgSpec {
	target = strings.TrimSpace(target)
	switch {
	case rangeTargetMatches(target, callName, "android.graphics.Color.argb"):
		return []rangeArgSpec{
			rangeFrameworkSpec(0, "alpha", "Color.argb", "argument", 0, 255),
			rangeFrameworkSpec(1, "red", "Color.argb", "argument", 0, 255),
			rangeFrameworkSpec(2, "green", "Color.argb", "argument", 0, 255),
			rangeFrameworkSpec(3, "blue", "Color.argb", "argument", 0, 255),
		}
	case rangeTargetMatches(target, callName, "android.graphics.Color.rgb"):
		return []rangeArgSpec{
			rangeFrameworkSpec(0, "red", "Color.rgb", "argument", 0, 255),
			rangeFrameworkSpec(1, "green", "Color.rgb", "argument", 0, 255),
			rangeFrameworkSpec(2, "blue", "Color.rgb", "argument", 0, 255),
		}
	case callName == "setAlpha" && rangeTargetAny(target,
		"android.graphics.Paint.setAlpha",
		"android.graphics.drawable.Drawable.setAlpha",
		"android.view.View.setAlpha",
		"android.widget.ImageView.setAlpha"):
		return []rangeArgSpec{rangeFrameworkSpec(0, "alpha", "setAlpha", "value", 0, 255)}
	case callName == "setProgress" && rangeTargetAny(target,
		"android.widget.ProgressBar.setProgress",
		"android.widget.AbsSeekBar.setProgress"):
		return []rangeArgSpec{rangeFrameworkSpec(0, "progress", "setProgress", "value", 0, 100)}
	case callName == "setRotation" && rangeTargetAny(target,
		"android.view.View.setRotation"):
		return []rangeArgSpec{rangeFrameworkSpec(0, "rotation", "setRotation", "value", -360, 360)}
	default:
		return nil
	}
}

func rangeFrameworkSpec(index int, name, callLabel, argLabel string, from, to float64) rangeArgSpec {
	return rangeArgSpec{
		index:     index,
		name:      name,
		callLabel: callLabel,
		argLabel:  argLabel,
		bounds: rangeBounds{
			from:          from,
			to:            to,
			fromInclusive: true,
			toInclusive:   true,
		},
	}
}

func rangeTargetMatches(target, callName, want string) bool {
	return callName == simpleQualifiedName(want) && rangeTargetAny(target, want)
}

func rangeTargetAny(target string, wants ...string) bool {
	for _, want := range wants {
		if target == want || strings.HasSuffix(target, "."+want) || strings.Contains(target, want+"(") {
			return true
		}
	}
	return false
}

func rangeArgumentForSpec(file *scanner.File, args uint32, spec rangeArgSpec) uint32 {
	if spec.name != "" {
		if arg := flatNamedValueArgument(file, args, spec.name); arg != 0 {
			return arg
		}
	}
	return flatPositionalValueArgument(file, args, spec.index)
}

func rangeValueArgumentCount(file *scanner.File, args uint32) int {
	count := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) == "value_argument" {
			count++
		}
	}
	return count
}

func rangeMaxSpecIndex(specs []rangeArgSpec) int {
	maxVal := -1
	for _, spec := range specs {
		if spec.index > maxVal {
			maxVal = spec.index
		}
	}
	return maxVal
}

func rangeValueArgumentExpression(file *scanner.File, arg uint32) uint32 {
	if file == nil || arg == 0 {
		return 0
	}
	if !flatHasValueArgumentLabel(file, arg) {
		return flatValueArgumentExpression(file, arg)
	}
	seenEquals := false
	for child := file.FlatFirstChild(arg); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "=" {
			seenEquals = true
			continue
		}
		if seenEquals && file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

func rangeEvaluateNumericExpr(file *scanner.File, idx uint32, constants map[string][]rangeConstantSpec, seen map[string]bool) (rangeNumber, uint32, bool) {
	if file == nil || idx == 0 {
		return rangeNumber{}, 0, false
	}
	idx = flatUnwrapParenExpr(file, idx)
	switch file.FlatType(idx) {
	case "integer_literal":
		if value, ok := rangeParseIntegerLiteral(file.FlatNodeText(idx)); ok {
			return rangeNumber{value: float64(value), node: idx}, idx, true
		}
	case "real_literal":
		if value, ok := rangeParseFloatLiteral(file.FlatNodeText(idx)); ok {
			return rangeNumber{value: value, node: idx}, idx, true
		}
	case "simple_identifier":
		name := file.FlatNodeString(idx, nil)
		if constants == nil {
			return rangeNumber{}, 0, false
		}
		if seen == nil {
			seen = map[string]bool{}
		}
		if seen[name] {
			return rangeNumber{}, 0, false
		}
		seen[name] = true
		if value, ok := rangeResolveConstant(file, name, idx, constants); ok {
			return value, idx, true
		}
		return rangeNumber{}, 0, false
	case "unary_expression", "prefix_expression":
		if !rangeHasMinusPrefix(file, idx) {
			return rangeNumber{}, 0, false
		}
		operand := rangeFirstNamedChildAfterPrefix(file, idx)
		value, _, ok := rangeEvaluateNumericExpr(file, operand, constants, seen)
		if !ok {
			return rangeNumber{}, 0, false
		}
		value.value = -value.value
		value.node = idx
		return value, idx, true
	}
	return rangeNumber{}, 0, false
}

func rangeResolveConstant(file *scanner.File, name string, use uint32, constants map[string][]rangeConstantSpec) (rangeNumber, bool) {
	candidates := constants[name]
	if len(candidates) == 0 {
		return rangeNumber{}, false
	}
	useFn, _ := flatEnclosingFunction(file, use)
	useOwner := androidEnclosingOwner(file, use)
	t := file.FlatTree
	useStart := t.StartBytes[use]
	bestScore := 0
	var best rangeConstantSpec
	for _, candidate := range candidates {
		if candidate.decl == 0 || t.StartBytes[candidate.decl] >= useStart {
			continue
		}
		score := 0
		switch {
		case candidate.function != 0:
			if candidate.function == useFn {
				score = 4
			}
		case candidate.owner != 0:
			if candidate.owner == useOwner {
				score = 3
			}
		default:
			score = 1
		}
		if score == 0 {
			continue
		}
		if score > bestScore || (score == bestScore && t.StartBytes[candidate.decl] > t.StartBytes[best.decl]) {
			best = candidate
			bestScore = score
		}
	}
	if bestScore == 0 {
		return rangeNumber{}, false
	}
	return best.value, true
}

func rangeHasMinusPrefix(file *scanner.File, idx uint32) bool {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "-" {
			return true
		}
		if file.FlatIsNamed(child) {
			return false
		}
	}
	return false
}

func rangeFirstNamedChildAfterPrefix(file *scanner.File, idx uint32) uint32 {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			return child
		}
	}
	return 0
}

func rangeParseIntegerLiteral(text string) (int64, bool) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(text), "_", "")
	cleaned = strings.TrimSuffix(strings.TrimSuffix(cleaned, "L"), "l")
	if cleaned == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(cleaned, 0, 64)
	return value, err == nil
}

func rangeParseFloatLiteral(text string) (float64, bool) {
	cleaned := strings.ReplaceAll(strings.TrimSpace(text), "_", "")
	cleaned = strings.TrimRight(cleaned, "fFdD")
	if cleaned == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(cleaned, 64)
	return value, err == nil
}

func rangeBoolLiteral(file *scanner.File, idx uint32) (bool, bool) {
	idx = flatUnwrapParenExpr(file, idx)
	if file == nil || idx == 0 || file.FlatType(idx) != "boolean_literal" {
		return false, false
	}
	switch file.FlatNodeString(idx, nil) {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
}

func (b rangeBounds) outside(value float64) bool {
	if b.fromInclusive {
		if value < b.from {
			return true
		}
	} else if value <= b.from {
		return true
	}
	if b.toInclusive {
		return value > b.to
	}
	return value >= b.to
}

func rangeFindingMessage(spec rangeArgSpec, valueText string) string {
	label := spec.argLabel
	if label == "" {
		label = "argument"
	}
	return fmt.Sprintf("%s() %s %s is outside the valid range %s.", spec.callLabel, label, valueText, spec.bounds.String())
}

func (b rangeBounds) String() string {
	left, right := "[", "]"
	if !b.fromInclusive {
		left = "("
	}
	if !b.toInclusive {
		right = ")"
	}
	return fmt.Sprintf("%s%s, %s%s", left, rangeBoundString(b.from), rangeBoundString(b.to), right)
}

func rangeBoundString(value float64) string {
	switch {
	case math.IsInf(value, -1):
		return "-inf"
	case math.IsInf(value, 1):
		return "inf"
	case math.Trunc(value) == value:
		return strconv.FormatInt(int64(value), 10)
	default:
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
}

func rangeNumberText(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	return strings.TrimSpace(file.FlatNodeText(idx))
}
