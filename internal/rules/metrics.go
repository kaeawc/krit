package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// MetricTimerOutsideBlockRule detects timer.record blocks that don't wrap any
// call work, making the timing measurement effectively meaningless.
type MetricTimerOutsideBlockRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *MetricTimerOutsideBlockRule) Confidence() float64 { return 0.75 }

func (r *MetricTimerOutsideBlockRule) shouldFlag(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return false
	}
	if flatCallExpressionName(file, idx) != "record" {
		return false
	}
	lambda := flatCallTrailingLambda(file, idx)
	if lambda == 0 {
		return false
	}
	hasCall := false
	file.FlatWalkNodes(lambda, "call_expression", func(call uint32) {
		if call != idx {
			hasCall = true
		}
	})
	return !hasCall
}

// MetricTagHighCardinalityRule detects metric constructor tag keys that would
// create one time series per user/session/request.
type MetricTagHighCardinalityRule struct {
	FlatDispatchBase
	BaseRule

	Keys []string
}

func (r *MetricTagHighCardinalityRule) Confidence() float64 { return 0.75 }

func (r *MetricTagHighCardinalityRule) shouldFlag(file *scanner.File, idx uint32) (string, bool) {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return "", false
	}
	if !metricConstructorNames[flatCallExpressionName(file, idx)] {
		return "", false
	}
	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return "", false
	}
	keys := highCardinalityKeySet(r.Keys)
	for pos := 1; ; pos += 2 {
		arg := flatPositionalValueArgument(file, args, pos)
		if arg == 0 {
			break
		}
		if key := firstHighCardinalityString(file, flatValueArgumentExpression(file, arg), keys); key != "" {
			return key, true
		}
	}
	return "", false
}

var metricConstructorNames = map[string]bool{
	"counter":             true,
	"gauge":               true,
	"timer":               true,
	"summary":             true,
	"distributionSummary": true,
	"longTaskTimer":       true,
}

// MetricNameMissingUnitRule detects metric names without a unit suffix.
type MetricNameMissingUnitRule struct {
	FlatDispatchBase
	BaseRule

	Suffixes []string
}

func (r *MetricNameMissingUnitRule) Confidence() float64 { return 0.75 }

func (r *MetricNameMissingUnitRule) shouldFlag(file *scanner.File, idx uint32) (string, bool) {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return "", false
	}
	if !metricConstructorNames[flatCallExpressionName(file, idx)] {
		return "", false
	}
	_, args := flatCallExpressionParts(file, idx)
	first := flatPositionalValueArgument(file, args, 0)
	nameNode := flatValueArgumentExpression(file, first)
	if nameNode == 0 || file.FlatType(nameNode) != "string_literal" || flatContainsStringInterpolation(file, nameNode) {
		return "", false
	}
	name := stringLiteralContent(file, nameNode)
	if name == "" || metricNameHasUnitSuffix(name, r.Suffixes) {
		return "", false
	}
	return name, true
}

// MetricCounterNotMonotonicRule detects negative counter increments.
type MetricCounterNotMonotonicRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *MetricCounterNotMonotonicRule) Confidence() float64 { return 0.75 }

func (r *MetricCounterNotMonotonicRule) shouldFlag(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return false
	}
	if flatCallExpressionName(file, idx) != "increment" {
		return false
	}
	_, args := flatCallExpressionParts(file, idx)
	first := flatValueArgumentExpression(file, flatPositionalValueArgument(file, args, 0))
	if first == 0 {
		return false
	}
	text := strings.TrimSpace(file.FlatNodeText(first))
	return strings.HasPrefix(text, "-") && len(text) > 1 && (text[1] >= '0' && text[1] <= '9')
}
