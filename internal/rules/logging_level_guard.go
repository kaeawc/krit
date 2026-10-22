package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// LogLevelGuardMissingRule detects debug/trace logger calls whose message
// template interpolates a call expression, causing eager work when the level is
// disabled unless the call is guarded.
type LogLevelGuardMissingRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Observability rule. Detection pattern-matches logging/metrics API call
// shapes without confirming the receiver type. Classified per roadmap/17.
func (r *LogLevelGuardMissingRule) Confidence() float64 { return api.ConfidenceMedium }

func logLevelGuardMessageNodeFlat(file *scanner.File, call uint32) uint32 {
	if file == nil || call == 0 {
		return 0
	}

	_, args := flatCallExpressionParts(file, call)
	if arg := logLevelGuardMessageArgumentFlat(file, args); arg != 0 {
		return arg
	}

	return logLevelGuardTrailingLambdaMessageFlat(file, call)
}

func logLevelGuardMessageArgumentFlat(file *scanner.File, args uint32) uint32 {
	if file == nil || args == 0 {
		return 0
	}

	if arg := flatNamedValueArgument(file, args, "message"); arg != 0 && flatContainsStringInterpolation(file, arg) {
		return arg
	}

	messageArg := flatPositionalValueArgument(file, args, 0)
	if messageArg == 0 {
		return 0
	}
	if flatContainsStringInterpolation(file, messageArg) {
		return messageArg
	}

	return flatPositionalValueArgument(file, args, 1)
}

func logLevelGuardTrailingLambdaMessageFlat(file *scanner.File, call uint32) uint32 {
	if file == nil || call == 0 {
		return 0
	}

	suffix, _ := file.FlatFindChild(call, "call_suffix")
	if suffix == 0 {
		return 0
	}

	lambda, _ := file.FlatFindChild(suffix, "annotated_lambda")
	if lambda != 0 {
		if lit, ok := file.FlatFindChild(lambda, "lambda_literal"); ok {
			lambda = lit
		}
	}
	if lambda == 0 {
		lambda, _ = file.FlatFindChild(suffix, "lambda_literal")
	}
	if lambda == 0 {
		return 0
	}

	statements, _ := file.FlatFindChild(lambda, "statements")
	if statements == 0 || file.FlatNamedChildCount(statements) == 0 {
		return 0
	}

	lastExpr := file.FlatNamedChild(statements, file.FlatNamedChildCount(statements)-1)
	if lastExpr == 0 {
		return 0
	}

	if template := flatFirstStringTemplateNode(file, lastExpr); template != 0 {
		return template
	}
	return lastExpr
}

func flatFirstStringTemplateNode(file *scanner.File, idx uint32) uint32 {
	if file == nil || idx == 0 {
		return 0
	}
	var match uint32
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		if match != 0 {
			return
		}
		switch file.FlatType(candidate) {
		case "line_string_literal", "multi_line_string_literal", "string_literal":
			match = candidate
		}
	})
	return match
}

func compactLoggerLevel(callee string) string {
	switch callee {
	case "debug", "d":
		return "debug"
	case "trace", "v":
		return "trace"
	default:
		return ""
	}
}

func genericLogReceiverName(receiver string) bool {
	return receiver == "Timber" || strings.HasSuffix(receiver, "Log")
}

func containsInterpolatedCallFlat(file *scanner.File, node uint32) bool {
	if file == nil || node == 0 {
		return false
	}

	found := false
	file.FlatWalkAllNodes(node, func(candidate uint32) {
		if found {
			return
		}
		if file.FlatType(candidate) == "call_expression" && flatHasStringInterpolationAncestor(file, candidate, node) {
			found = true
		}
	})
	return found
}

func flatHasStringInterpolationAncestor(file *scanner.File, node, stop uint32) bool {
	for current, ok := file.FlatParent(node); ok && current != stop; current, ok = file.FlatParent(current) {
		switch file.FlatType(current) {
		case "interpolated_expression", "line_string_expression", "multi_line_string_expression":
			return true
		}
	}
	return false
}

func isInsideMatchingLogLevelGuardFlat(file *scanner.File, idx uint32, receiver, level string) bool {
	guardProperty := logLevelGuardProperty(level)

	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "if_expression":
			condition, thenBody, elseBody := ifConditionThenElseBodiesFlat(file, parent)
			if condition == 0 {
				continue
			}

			text := normalizeConditionText(file.FlatNodeText(condition))
			if thenBody != 0 && flatNodeWithin(file, thenBody, idx) && conditionTextRequiresGuard(text, receiver, guardProperty) {
				return true
			}
			if elseBody != 0 && flatNodeWithin(file, elseBody, idx) && conditionTextPreventsGuard(text, receiver, guardProperty) {
				return true
			}
		case "when_entry":
			if whenEntryRequiresGuardFlat(file, parent, idx, receiver, guardProperty) {
				return true
			}
		}
	}

	return false
}

func isAfterMatchingLogLevelEarlyExitFlatObs(file *scanner.File, idx uint32, receiver, level string) bool {
	if file == nil || idx == 0 {
		return false
	}

	guardProperty := logLevelGuardProperty(level)

	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "function_declaration", "lambda_literal", "anonymous_function", "source_file":
			return false
		case "statements":
			_, index := flatContainingChild(file, parent, idx)
			if index < 0 {
				continue
			}
			for i := index - 1; i >= 0; i-- {
				sibling := file.FlatChild(parent, i)
				if sibling == 0 {
					continue
				}
				switch file.FlatType(sibling) {
				case "if_expression":
					if ifExpressionHasEarlyExitForDisabledLevelFlatObs(file, sibling, receiver, guardProperty) {
						return true
					}
				case "when_expression":
					if whenExpressionHasEarlyExitForDisabledLevelFlatObs(file, sibling, receiver, guardProperty) {
						return true
					}
				}
			}
		}
	}

	return false
}

func flatContainingChild(file *scanner.File, container, node uint32) (uint32, int) {
	if file == nil || container == 0 || node == 0 {
		return 0, -1
	}
	for i := 0; i < file.FlatChildCount(container); i++ {
		child := file.FlatChild(container, i)
		if child == 0 {
			continue
		}
		if flatNodeWithin(file, child, node) {
			return child, i
		}
	}
	return 0, -1
}

func ifExpressionHasEarlyExitForDisabledLevelFlatObs(file *scanner.File, node uint32, receiver, guardProperty string) bool {
	if file == nil || node == 0 || file.FlatType(node) != "if_expression" {
		return false
	}

	condition, body, _ := ifConditionThenElseBodiesFlatObs(file, node)
	if condition == 0 || body == 0 || !isEarlyExitFlat(file, body) {
		return false
	}

	text := normalizeConditionText(file.FlatNodeText(condition))
	return conditionTextPreventsGuard(text, receiver, guardProperty)
}

func whenEntryRequiresGuardFlat(file *scanner.File, entry, node uint32, receiver, guardProperty string) bool {
	if file == nil || entry == 0 || file.FlatType(entry) != "when_entry" {
		return false
	}

	body := whenEntryBodyFlatObs(file, entry)
	if body == 0 || !flatNodeWithin(file, body, node) {
		return false
	}

	whenExpr, ok := file.FlatParent(entry)
	if !ok || file.FlatType(whenExpr) != "when_expression" {
		return false
	}

	texts := whenEntryConditionTextsFlatObs(file, entry)
	if len(texts) == 0 {
		return false
	}

	if whenExpressionHasMatchingGuardSubjectFlatObs(file, whenExpr, receiver, guardProperty) {
		if whenElseEntryIsGuardedByPriorDisabledBranchFlatObs(file, whenExpr, entry, receiver, guardProperty) {
			return true
		}
		for _, text := range texts {
			if normalizeConditionText(text) != "true" {
				return false
			}
		}
		return true
	}

	if whenExpressionHasSubjectFlatObs(file, whenExpr) {
		return false
	}

	if whenElseEntryIsGuardedByPriorDisabledBranchFlatObs(file, whenExpr, entry, receiver, guardProperty) {
		return true
	}

	for _, text := range texts {
		if !conditionTextRequiresGuard(text, receiver, guardProperty) {
			return false
		}
	}

	return true
}

func whenElseEntryIsGuardedByPriorDisabledBranchFlatObs(file *scanner.File, whenExpr, entry uint32, receiver, guardProperty string) bool {
	if !whenEntryHasElseConditionFlatObs(file, entry) {
		return false
	}

	matchingSubject := whenExpressionHasMatchingGuardSubjectFlatObs(file, whenExpr, receiver, guardProperty)
	if whenExpressionHasSubjectFlatObs(file, whenExpr) && !matchingSubject {
		return false
	}

	for i := 0; i < file.FlatChildCount(whenExpr); i++ {
		sibling := file.FlatChild(whenExpr, i)
		if sibling == 0 || file.FlatType(sibling) != "when_entry" {
			continue
		}
		if sibling == entry {
			return false
		}

		for _, text := range whenEntryConditionTextsFlatObs(file, sibling) {
			if matchingSubject {
				if normalizeConditionText(text) == "false" {
					return true
				}
				continue
			}
			if conditionTextPreventsGuard(text, receiver, guardProperty) {
				return true
			}
		}
	}

	return false
}

func whenEntryHasElseConditionFlatObs(file *scanner.File, entry uint32) bool {
	for _, text := range whenEntryConditionTextsFlatObs(file, entry) {
		if normalizeConditionText(text) == "else" {
			return true
		}
	}
	return false
}

func whenExpressionHasEarlyExitForDisabledLevelFlatObs(file *scanner.File, node uint32, receiver, guardProperty string) bool {
	if file == nil || node == 0 || file.FlatType(node) != "when_expression" {
		return false
	}

	for i := 0; i < file.FlatChildCount(node); i++ {
		entry := file.FlatChild(node, i)
		if entry == 0 || file.FlatType(entry) != "when_entry" {
			continue
		}

		body := whenEntryBodyFlatObs(file, entry)
		if body == 0 || !isEarlyExitFlat(file, body) {
			continue
		}

		for _, text := range whenEntryConditionTextsFlatObs(file, entry) {
			if whenExpressionHasMatchingGuardSubjectFlatObs(file, node, receiver, guardProperty) {
				if normalizeConditionText(text) == "false" {
					return true
				}
				continue
			}
			if whenExpressionHasSubjectFlatObs(file, node) {
				continue
			}
			if conditionTextPreventsGuard(text, receiver, guardProperty) {
				return true
			}
		}
	}

	return false
}

func whenExpressionHasMatchingGuardSubjectFlatObs(file *scanner.File, node uint32, receiver, guardProperty string) bool {
	subject := whenExpressionSubjectTextFlatObs(file, node)
	if subject == "" {
		return false
	}
	return conditionTextMatchesGuard(subject, receiver, guardProperty)
}

func whenExpressionHasSubjectFlatObs(file *scanner.File, node uint32) bool {
	if file == nil || node == 0 || file.FlatType(node) != "when_expression" {
		return false
	}
	for i := 0; i < file.FlatChildCount(node); i++ {
		child := file.FlatChild(node, i)
		if child != 0 && file.FlatType(child) == "when_subject" {
			return true
		}
	}
	return false
}

func whenExpressionSubjectTextFlatObs(file *scanner.File, node uint32) string {
	if file == nil || node == 0 || file.FlatType(node) != "when_expression" {
		return ""
	}
	for i := 0; i < file.FlatChildCount(node); i++ {
		child := file.FlatChild(node, i)
		if child == 0 || file.FlatType(child) != "when_subject" {
			continue
		}
		return normalizeConditionText(file.FlatNodeText(child))
	}
	return ""
}

func whenEntryBodyFlatObs(file *scanner.File, node uint32) uint32 {
	if file == nil || node == 0 || file.FlatType(node) != "when_entry" {
		return 0
	}

	if body, ok := file.FlatFindChild(node, "control_structure_body"); ok {
		return body
	}

	for i := file.FlatNamedChildCount(node) - 1; i >= 0; i-- {
		child := file.FlatNamedChild(node, i)
		if child == 0 {
			continue
		}
		switch file.FlatType(child) {
		case "when_condition", "else":
			continue
		default:
			return child
		}
	}
	return 0
}

func whenEntryConditionTextsFlatObs(file *scanner.File, node uint32) []string {
	if file == nil || node == 0 || file.FlatType(node) != "when_entry" {
		return nil
	}

	var texts []string
	for i := 0; i < file.FlatChildCount(node); i++ {
		child := file.FlatChild(node, i)
		if child == 0 {
			continue
		}
		switch file.FlatType(child) {
		case "when_condition":
			texts = append(texts, normalizeConditionText(file.FlatNodeText(child)))
		case "else":
			texts = append(texts, "else")
		}
	}
	return texts
}

func ifConditionThenElseBodiesFlatObs(file *scanner.File, node uint32) (condition, thenBody, elseBody uint32) {
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

func isEarlyExitFlat(file *scanner.File, node uint32) bool {
	if file == nil || node == 0 {
		return false
	}
	switch file.FlatType(node) {
	case "jump_expression":
		first := file.FlatFirstChild(node)
		if first == 0 {
			return false
		}
		switch file.FlatType(first) {
		case "return", "throw", "break", "continue":
			return true
		}
		return false
	case "control_structure_body":
		stmts, _ := file.FlatFindChild(node, "statements")
		if stmts != 0 {
			return isEarlyExitFlat(file, stmts)
		}
		for i := file.FlatChildCount(node) - 1; i >= 0; i-- {
			child := file.FlatChild(node, i)
			if child == 0 {
				continue
			}
			switch file.FlatType(child) {
			case "line_comment", "multiline_comment", "{", "}":
				continue
			default:
				return isEarlyExitFlat(file, child)
			}
		}
	case "statements":
		for i := file.FlatChildCount(node) - 1; i >= 0; i-- {
			child := file.FlatChild(node, i)
			if child == 0 {
				continue
			}
			switch file.FlatType(child) {
			case "line_comment", "multiline_comment", "{", "}":
				continue
			default:
				return isEarlyExitFlat(file, child)
			}
		}
	case "if_expression", "when_expression":
		return false
	}
	return false
}

func flatNodeWithin(file *scanner.File, container, node uint32) bool {
	if file == nil || container == 0 || node == 0 {
		return false
	}
	if container == node {
		return true
	}
	for current, ok := file.FlatParent(node); ok; current, ok = file.FlatParent(current) {
		if current == container {
			return true
		}
	}
	return false
}
