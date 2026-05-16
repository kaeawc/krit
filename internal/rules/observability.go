package rules

import (
	"regexp"

	"github.com/kaeawc/krit/internal/scanner"
)

// ifConditionThenElseBodiesFlat returns the (condition, then-body, else-body)
// triple of an `if_expression`. Shared between logging guard analysis and the
// i18n plurals rule.
func ifConditionThenElseBodiesFlat(file *scanner.File, node uint32) (condition, thenBody, elseBody uint32) {
	if file == nil || node == 0 || file.FlatType(node) != "if_expression" {
		return 0, 0, 0
	}

	sawElse := false
	for i := 0; i < file.FlatChildCount(node); i++ {
		child := file.FlatChild(node, i)
		if child == 0 {
			continue
		}
		switch file.FlatType(child) {
		case "if", "(", ")", "{", "}":
			continue
		case "else":
			sawElse = true
			continue
		default:
			if condition == 0 {
				condition = child
				continue
			}
			if !sawElse && thenBody == 0 {
				thenBody = child
				continue
			}
			if sawElse && elseBody == 0 {
				elseBody = child
				return condition, thenBody, elseBody
			}
		}
	}

	return condition, thenBody, elseBody
}

// coroutineBuilderPartsFlat extracts the builder name, value-arguments node,
// and trailing lambda from a coroutine builder call. Shared by logging
// (LogWithoutCorrelationID, MdcAcrossCoroutineBoundary) and tracing
// (WithContextWithoutTracingContext) rules.
func coroutineBuilderPartsFlat(file *scanner.File, idx uint32) (name string, args uint32, lambda uint32) {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return "", 0, 0
	}

	first := file.FlatChild(idx, 0)
	if first == 0 {
		return "", 0, 0
	}

	switch file.FlatType(first) {
	case "call_expression":
		name = flatCallExpressionName(file, first)
		_, args = flatCallExpressionParts(file, first)
	case "simple_identifier", "navigation_expression":
		name = flatCallExpressionName(file, idx)
		_, args = flatCallExpressionParts(file, idx)
	default:
		return "", 0, 0
	}

	callSuffix, _ := file.FlatFindChild(idx, "call_suffix")
	if callSuffix == 0 {
		return name, args, 0
	}
	annotatedLambda, _ := file.FlatFindChild(callSuffix, "annotated_lambda")
	if annotatedLambda != 0 {
		lambda, _ = file.FlatFindChild(annotatedLambda, "lambda_literal")
		if lambda != 0 {
			return name, args, lambda
		}
	}
	lambda, _ = file.FlatFindChild(callSuffix, "lambda_literal")
	if lambda != 0 {
		return name, args, lambda
	}

	return name, args, 0
}

// coroutineContextHasMDCFlat reports whether the value-arguments node of a
// coroutine builder contains a call to `MDCContext()`.
func coroutineContextHasMDCFlat(file *scanner.File, args uint32) bool {
	if args == 0 {
		return false
	}

	found := false
	file.FlatWalkAllNodes(args, func(node uint32) {
		if found || file.FlatType(node) != "call_expression" {
			return
		}
		if flatCallExpressionName(file, node) == "MDCContext" {
			found = true
		}
	})
	return found
}

// firstHighCardinalityString walks a value-expression and returns the first
// non-interpolated string literal whose content is in `keys`. Shared by
// tracing (SpanAttributeWithHighCardinalityRule) and metrics
// (MetricTagHighCardinalityRule).
func firstHighCardinalityString(file *scanner.File, root uint32, keys map[string]bool) string {
	if file == nil || root == 0 || len(keys) == 0 {
		return ""
	}
	if file.FlatType(root) == "string_literal" && !flatContainsStringInterpolation(file, root) {
		key := stringLiteralContent(file, root)
		if keys[key] {
			return key
		}
	}
	found := ""
	file.FlatWalkNodes(root, "string_literal", func(lit uint32) {
		if found != "" || flatContainsStringInterpolation(file, lit) {
			return
		}
		key := stringLiteralContent(file, lit)
		if keys[key] {
			found = key
		}
	})
	return found
}

// throwableLikeIdentifierRe matches common Kotlin/Java identifiers that
// idiomatically refer to a Throwable. Used by logging rules and shared
// observability helpers.
var throwableLikeIdentifierRe = regexp.MustCompile(`\b(e|ex|exc|error|t|throwable|cause|exception)\b`)

// interpolationIdentifierRe captures the bare identifier in a Kotlin string
// template interpolation (e.g. `$foo` or `${foo}`). Used by logging rules and
// shared observability helpers.
var interpolationIdentifierRe = regexp.MustCompile(`\$\{?\s*([A-Za-z_][A-Za-z0-9_]*)`)
