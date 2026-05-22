package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
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

var knownLoggerPackagePrefixes = []string{
	"java.util.logging.",
	"org.slf4j.",
	"ch.qos.logback.",
	"org.apache.logging.log4j.",
	"mu.",
	"io.github.oshai.kotlinlogging.",
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

// buildLoggerImportsFromAST returns whether any known logger package is
// imported by the file, plus a map of alias/simple-name to FQN for all
// known logger imports (used to resolve aliased logger receivers).
// Backed by the shared filefacts.Cache so the underlying import set is
// computed once per file per run.
func buildLoggerImportsFromAST(file *scanner.File) (bool, map[string]string) {
	if file == nil {
		return false, nil
	}
	imports := fileFactsCache().Imports(file)
	aliases := make(map[string]string)
	found := false
	for name, fqn := range imports.Aliases {
		for _, prefix := range knownLoggerPackagePrefixes {
			if strings.HasPrefix(fqn, prefix) {
				found = true
				aliases[name] = fqn
				break
			}
		}
	}
	for w := range imports.Wildcards {
		for _, prefix := range knownLoggerPackagePrefixes {
			if strings.HasPrefix(w, prefix) {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	return found, aliases
}

func receiverHasKnownLoggerTypeFlat(file *scanner.File, idx uint32, receiver string) bool {
	if file == nil || idx == 0 || receiver == "" {
		return false
	}

	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "function_declaration":
			if receiverHasKnownLoggerTypeInParametersFlat(file, parent, receiver) {
				return true
			}
		case "statements", "class_body", "source_file":
			if receiverHasKnownLoggerTypeInPropertiesFlat(file, parent, receiver) {
				return true
			}
		case "class_declaration":
			if receiverHasKnownLoggerTypeInClassParametersFlat(file, parent, receiver) {
				return true
			}
		}
	}

	return false
}

func receiverHasKnownLoggerTypeInParametersFlat(file *scanner.File, function uint32, receiver string) bool {
	params, _ := file.FlatFindChild(function, "function_value_parameters")
	if params == 0 {
		return false
	}

	for param := file.FlatFirstChild(params); param != 0; param = file.FlatNextSib(param) {
		if !file.FlatIsNamed(param) || file.FlatType(param) != "parameter" {
			continue
		}
		if extractIdentifierFlat(file, param) != receiver {
			continue
		}
		return isKnownLoggerTypeText(explicitTypeTextFlat(file, param))
	}

	return false
}

func receiverHasKnownLoggerTypeInPropertiesFlat(file *scanner.File, container uint32, receiver string) bool {
	for child := file.FlatFirstChild(container); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) || file.FlatType(child) != "property_declaration" {
			continue
		}
		if extractIdentifierFlat(file, child) != receiver {
			continue
		}
		return isKnownLoggerTypeText(explicitTypeTextFlat(file, child))
	}

	return false
}

func receiverHasKnownLoggerTypeInClassParametersFlat(file *scanner.File, classDecl uint32, receiver string) bool {
	ctor, _ := file.FlatFindChild(classDecl, "primary_constructor")
	if ctor == 0 {
		return false
	}

	for i := 0; i < file.FlatNamedChildCount(ctor); i++ {
		param := file.FlatNamedChild(ctor, i)
		if param == 0 || file.FlatType(param) != "class_parameter" {
			continue
		}
		if extractIdentifierFlat(file, param) != receiver || !classParameterDefinesPropertyFlat(file, param) {
			continue
		}
		return isKnownLoggerTypeText(explicitTypeTextFlat(file, param))
	}

	return false
}

func classParameterDefinesPropertyFlat(file *scanner.File, param uint32) bool {
	for c := file.FlatFirstChild(param); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "binding_pattern_kind" {
			return true
		}
	}
	return false
}

func explicitTypeTextFlat(file *scanner.File, node uint32) string {
	if file == nil || node == 0 {
		return ""
	}

	if text := directExplicitTypeTextFlat(file, node); text != "" {
		return text
	}

	if child, ok := file.FlatFindChild(node, "variable_declaration"); ok {
		return directExplicitTypeTextFlat(file, child)
	}

	return ""
}

func directExplicitTypeTextFlat(file *scanner.File, node uint32) string {
	colonSeen := false
	for i := 0; i < file.FlatChildCount(node); i++ {
		child := file.FlatChild(node, i)
		if child == 0 {
			continue
		}
		if file.FlatType(child) == ":" {
			colonSeen = true
			continue
		}
		if !colonSeen {
			continue
		}

		switch file.FlatType(child) {
		case "user_type", "nullable_type", "type_identifier", "function_type", "parenthesized_type":
			return file.FlatNodeText(child)
		}
	}

	return ""
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

// LogWithoutCorrelationIDRule detects logger calls inside coroutine builders
// whose context does not include MDCContext().
type LogWithoutCorrelationIDRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Observability rule. Detection pattern-matches logging/metrics API call
// shapes without confirming the receiver type. Classified per roadmap/17.
func (r *LogWithoutCorrelationIDRule) Confidence() float64 { return api.ConfidenceMedium }

// NullableStructuredFieldRule detects addKeyValue fields that pass nullable
// safe-call expressions without an explicit fallback value.
type NullableStructuredFieldRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *NullableStructuredFieldRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NullableStructuredFieldRule) shouldFlag(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 || file.FlatType(idx) != "call_expression" {
		return false
	}
	if flatCallExpressionName(file, idx) != "addKeyValue" {
		return false
	}
	_, args := flatCallExpressionParts(file, idx)
	if args == 0 {
		return false
	}
	valueArg := flatPositionalValueArgument(file, args, 1)
	if valueArg == 0 {
		valueArg = flatNamedValueArgument(file, args, "value")
	}
	expr := flatValueArgumentExpression(file, valueArg)
	if expr == 0 {
		return false
	}
	return nullableSafeCallWithoutElvis(file, expr)
}

// nullableSafeCallWithoutElvis reports whether the expression rooted at idx
// contains an actual `?.` safe-navigation token outside any string literal
// and is not the receiver of an `?:` Elvis expression that supplies a
// fallback. Detection runs over the flat AST rather than raw text so that
// string-literal contents such as `"use ?. operator"` cannot trigger a
// false positive.
func nullableSafeCallWithoutElvis(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	// An Elvis expression at the top supplies the fallback; any safe call
	// inside is paired with `?:` so the rule should not fire.
	if file.FlatType(idx) == "elvis_expression" {
		return false
	}
	return flatExpressionHasSafeCallOutsideStrings(file, idx)
}

// flatExpressionHasSafeCallOutsideStrings walks the flat AST rooted at idx
// and returns true when it finds a `?.` token node that is not nested inside
// a string literal. It also returns false if any enclosing/descendant Elvis
// expression provides a fallback for the safe call.
func flatExpressionHasSafeCallOutsideStrings(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	hasSafe := false
	hasElvis := false
	var walk func(uint32)
	walk = func(node uint32) {
		if node == 0 || hasSafe && hasElvis {
			return
		}
		switch file.FlatType(node) {
		case "string_literal", "line_string_literal", "multi_line_string_literal",
			"raw_string_literal", "character_literal",
			"line_comment", "multiline_comment":
			return
		case "?.":
			hasSafe = true
		case "elvis_expression":
			hasElvis = true
		}
		for i := 0; i < file.FlatChildCount(node); i++ {
			walk(file.FlatChild(node, i))
		}
	}
	walk(idx)
	return hasSafe && !hasElvis
}

func firstCorrelationSensitiveLogCallFlat(file *scanner.File, idx uint32) uint32 {
	var match uint32
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		if match != 0 || file.FlatType(candidate) != "call_expression" {
			return
		}

		receiver := flatReceiverNameFromCall(file, candidate)
		if receiver != "logger" && receiver != "log" && receiver != "Timber" {
			return
		}

		switch flatCallExpressionName(file, candidate) {
		case "trace", "debug", "info", "warn", "warning", "error":
			match = candidate
		}
	})
	return match
}

// LoggerWithoutLoggerFieldRule detects LoggerFactory.getLogger(...) calls
// inside function bodies where a per-call logger instance would be created.
type LoggerWithoutLoggerFieldRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Observability rule. Detection pattern-matches logging/metrics API call
// shapes without confirming the receiver type. Classified per roadmap/17.
func (r *LoggerWithoutLoggerFieldRule) Confidence() float64 { return api.ConfidenceMedium }

// LoggerInterpolatedMessageRule detects SLF4J/Logback/log4j-style logger calls
// whose message argument is a Kotlin string template with interpolations.
// Parameterized logging (`logger.info("user {} logged in", id)`) is preferred
// because the template caches and the call skips argument evaluation when the
// level is disabled. Timber is excluded — its API is designed around Kotlin
// string interpolation. The lazy lambda form
// (`logger.info { "user $id logged in" }`) is also excluded because it defers
// evaluation until the level is enabled.
type LoggerInterpolatedMessageRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Observability rule. Detection pattern-matches logging/metrics API call
// shapes without confirming the receiver type. Classified per roadmap/17.
func (r *LoggerInterpolatedMessageRule) Confidence() float64 { return api.ConfidenceMedium }

// loggerLevelMethods are the SLF4J-style log level method names that take a
// message template as the first argument.
var loggerLevelMethods = map[string]bool{
	"trace":   true,
	"debug":   true,
	"info":    true,
	"warn":    true,
	"warning": true,
	"error":   true,
}

// loggerConventionalReceivers are receiver identifiers that idiomatically
// refer to a logger instance even without an explicit type declaration.
var loggerConventionalReceivers = map[string]bool{
	"logger": true,
	"log":    true,
	"LOG":    true,
	"LOGGER": true,
}

// receiverIsKnownLoggerFlat reports whether the receiver of `call` is
// recognised as a logger instance. It checks the conventional names, declared
// types, and module-level imports — but never matches Timber, which is the
// rule's documented carve-out.
func receiverIsKnownLoggerFlat(file *scanner.File, call uint32, receiver string) bool {
	if receiver == "" || receiver == "Timber" {
		return false
	}
	if loggerConventionalReceivers[receiver] {
		return true
	}
	if receiverHasKnownLoggerTypeFlat(file, call, receiver) {
		return true
	}
	knownLoggerImport, aliases := buildLoggerImportsFromAST(file)
	if isLikelyLogReceiver(receiver, aliases) {
		return true
	}
	return knownLoggerImport
}

func receiverIsKnownParameterizedLoggerFlat(file *scanner.File, call uint32, receiver string) bool {
	if receiver == "" || receiver == "Timber" {
		return false
	}
	knownLoggerImport, aliases := buildLoggerImportsFromAST(file)
	if known, typed := receiverExplicitLoggerTypeStatusFlat(file, call, receiver, aliases); typed {
		return known
	}
	if receiverHasKnownLoggerTypeFlat(file, call, receiver) {
		return true
	}
	if isLikelyLogReceiver(receiver, aliases) {
		return true
	}
	if receiver == "logger" && receiverIsGradleTaskLoggerFlat(file, call) {
		return true
	}
	return loggerConventionalReceivers[receiver] && knownLoggerImport
}

func receiverExplicitLoggerTypeStatusFlat(file *scanner.File, idx uint32, receiver string, aliases map[string]string) (bool, bool) {
	if file == nil || idx == 0 || receiver == "" {
		return false, false
	}

	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "function_declaration":
			if known, typed := receiverExplicitLoggerTypeInParametersFlat(file, parent, receiver, aliases); typed {
				return known, true
			}
		case "statements", "class_body", "source_file":
			if known, typed := receiverExplicitLoggerTypeInPropertiesFlat(file, parent, receiver, aliases); typed {
				return known, true
			}
		case "class_declaration":
			if known, typed := receiverExplicitLoggerTypeInClassParametersFlat(file, parent, receiver, aliases); typed {
				return known, true
			}
		}
	}

	return false, false
}

func receiverExplicitLoggerTypeInParametersFlat(file *scanner.File, function uint32, receiver string, aliases map[string]string) (bool, bool) {
	params, _ := file.FlatFindChild(function, "function_value_parameters")
	if params == 0 {
		return false, false
	}

	for param := file.FlatFirstChild(params); param != 0; param = file.FlatNextSib(param) {
		if !file.FlatIsNamed(param) || file.FlatType(param) != "parameter" {
			continue
		}
		if extractIdentifierFlat(file, param) != receiver {
			continue
		}
		return loggerTypeTextStatusFlat(explicitTypeTextFlat(file, param), aliases)
	}

	return false, false
}

func receiverExplicitLoggerTypeInPropertiesFlat(file *scanner.File, container uint32, receiver string, aliases map[string]string) (bool, bool) {
	for child := file.FlatFirstChild(container); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) || file.FlatType(child) != "property_declaration" {
			continue
		}
		if extractIdentifierFlat(file, child) != receiver {
			continue
		}
		return loggerTypeTextStatusFlat(explicitTypeTextFlat(file, child), aliases)
	}

	return false, false
}

func receiverExplicitLoggerTypeInClassParametersFlat(file *scanner.File, classDecl uint32, receiver string, aliases map[string]string) (bool, bool) {
	ctor, _ := file.FlatFindChild(classDecl, "primary_constructor")
	if ctor == 0 {
		return false, false
	}

	for i := 0; i < file.FlatNamedChildCount(ctor); i++ {
		param := file.FlatNamedChild(ctor, i)
		if param == 0 || file.FlatType(param) != "class_parameter" {
			continue
		}
		if extractIdentifierFlat(file, param) != receiver || !classParameterDefinesPropertyFlat(file, param) {
			continue
		}
		return loggerTypeTextStatusFlat(explicitTypeTextFlat(file, param), aliases)
	}

	return false, false
}

func loggerTypeTextStatusFlat(text string, aliases map[string]string) (bool, bool) {
	text = compactConditionText(strings.TrimSuffix(strings.TrimSpace(text), "?"))
	if text == "" {
		return false, false
	}
	if isKnownLoggerTypeText(text) {
		return true, true
	}
	if fqn, ok := aliases[text]; ok && isKnownLoggerTypeText(fqn) {
		return true, true
	}
	return false, true
}

func receiverIsGradleTaskLoggerFlat(file *scanner.File, call uint32) bool {
	classDecl, ok := flatEnclosingAncestor(file, call, "class_declaration")
	return ok && classHasSupertypeNamed(file, classDecl, "DefaultTask")
}

// loggerInterpolatedMessageArgFlat returns the message argument of `call` when
// it is a positional or named string-template argument that contains
// interpolations. It deliberately ignores the lazy lambda form
// (`logger.info { ... }`) because that form is the recommended substitute.
func loggerInterpolatedMessageArgFlat(file *scanner.File, call uint32) uint32 {
	_, args := flatCallExpressionParts(file, call)
	if args == 0 {
		return 0
	}

	if arg := flatNamedValueArgument(file, args, "message"); arg != 0 {
		if flatContainsStringInterpolation(file, arg) {
			return arg
		}
		return 0
	}

	first := flatPositionalValueArgument(file, args, 0)
	if first == 0 {
		return 0
	}
	if flatContainsStringInterpolation(file, first) {
		return first
	}
	return 0
}

// UnstructuredErrorLogRule detects logger.error/logger.warn calls that embed a
// Throwable-looking value in the message instead of passing it as a structured
// throwable argument.
type UnstructuredErrorLogRule struct {
	FlatDispatchBase
	BaseRule

	Methods []string
}

func (r *UnstructuredErrorLogRule) Confidence() float64 { return api.ConfidenceMediumLow }

func (r *UnstructuredErrorLogRule) methodEnabled(method string) bool {
	if len(r.Methods) == 0 {
		return method == "error" || method == "warn" || method == "warning"
	}
	for _, configured := range r.Methods {
		if strings.TrimSpace(configured) == method {
			return true
		}
	}
	return false
}

func unstructuredErrorLogMessageArgFlat(file *scanner.File, call uint32) uint32 {
	_, args := flatCallExpressionParts(file, call)
	if args == 0 || flatValueArgumentCount(args, file) != 1 {
		return 0
	}
	arg := flatPositionalValueArgument(file, args, 0)
	if arg == 0 {
		arg = flatNamedValueArgument(file, args, "message")
	}
	if arg == 0 {
		return 0
	}
	expr := flatValueArgumentExpression(file, arg)
	if expr == 0 {
		return 0
	}
	if flatContainsStringInterpolation(file, expr) && throwableLikeInterpolation(file.FlatNodeText(expr)) {
		return arg
	}
	if concatHasThrowableLikeOperand(file, expr) {
		return arg
	}
	if stringFormatHasThrowableLikeArg(file, expr) {
		return arg
	}
	return 0
}

func flatValueArgumentCount(args uint32, file *scanner.File) int {
	if file == nil || args == 0 {
		return 0
	}
	count := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) == "value_argument" {
			count++
		}
	}
	return count
}

func concatHasThrowableLikeOperand(file *scanner.File, expr uint32) bool {
	expr = flatUnwrapParenExpr(file, expr)
	if file == nil || expr == 0 || file.FlatType(expr) != "additive_expression" {
		return false
	}
	var operands []uint32
	if !collectStringConcatOperands(file, expr, &operands) {
		return false
	}
	for _, op := range operands {
		op = flatUnwrapParenExpr(file, op)
		switch file.FlatType(op) {
		case "string_literal", "line_string_literal", "multi_line_string_literal":
			continue
		default:
			if throwableLikeIdentifierExpr(file, op) {
				return true
			}
		}
	}
	return false
}

func stringFormatHasThrowableLikeArg(file *scanner.File, expr uint32) bool {
	expr = flatUnwrapParenExpr(file, expr)
	if file == nil || expr == 0 || file.FlatType(expr) != "call_expression" || flatCallExpressionName(file, expr) != "format" {
		return false
	}
	_, args := flatCallExpressionParts(file, expr)
	if args == 0 {
		return false
	}
	index := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		expr := flatValueArgumentExpression(file, arg)
		if index > 0 && throwableLikeIdentifierExpr(file, expr) {
			return true
		}
		index++
	}
	return false
}

func throwableLikeIdentifierExpr(file *scanner.File, expr uint32) bool {
	expr = flatUnwrapParenExpr(file, expr)
	if file == nil || expr == 0 {
		return false
	}
	switch file.FlatType(expr) {
	case "simple_identifier":
		return throwableLikeIdentifierRe.MatchString(file.FlatNodeText(expr))
	default:
		return false
	}
}

// TraceIDLoggedAsPlainMessageRule detects trace/span/request/correlation IDs
// embedded directly in log message text instead of being carried in MDC or a
// structured logging field.
type TraceIDLoggedAsPlainMessageRule struct {
	FlatDispatchBase
	BaseRule

	Identifiers []string
}

func (r *TraceIDLoggedAsPlainMessageRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *TraceIDLoggedAsPlainMessageRule) identifierSet() map[string]bool {
	defaults := []string{"traceId", "trace_id", "spanId", "span_id", "requestId", "request_id", "correlationId", "correlation_id"}
	values := r.Identifiers
	if len(values) == 0 {
		values = defaults
	}
	out := make(map[string]bool, len(values))
	for _, value := range values {
		if normalized := normalizeObservabilityIdentifier(value); normalized != "" {
			out[normalized] = true
		}
	}
	return out
}

func traceIDPlainMessageArgFlat(file *scanner.File, call uint32, identifiers map[string]bool) uint32 {
	_, args := flatCallExpressionParts(file, call)
	if args == 0 {
		return 0
	}
	arg := flatNamedValueArgument(file, args, "message")
	if arg == 0 {
		arg = flatPositionalValueArgument(file, args, 0)
	}
	if arg == 0 {
		return 0
	}
	expr := flatValueArgumentExpression(file, arg)
	if expr == 0 {
		return 0
	}
	if flatContainsStringInterpolation(file, expr) && interpolationHasObservedIdentifier(file.FlatNodeText(expr), identifiers) {
		return arg
	}
	if concatHasObservedIdentifier(file, expr, identifiers) {
		return arg
	}
	if stringFormatHasObservedIdentifier(file, expr, identifiers) {
		return arg
	}
	return 0
}

func concatHasObservedIdentifier(file *scanner.File, expr uint32, identifiers map[string]bool) bool {
	expr = flatUnwrapParenExpr(file, expr)
	if file == nil || expr == 0 || file.FlatType(expr) != "additive_expression" {
		return false
	}
	var operands []uint32
	if !collectStringConcatOperands(file, expr, &operands) {
		return false
	}
	for _, op := range operands {
		if observedIdentifierExpr(file, op, identifiers) {
			return true
		}
	}
	return false
}

func stringFormatHasObservedIdentifier(file *scanner.File, expr uint32, identifiers map[string]bool) bool {
	expr = flatUnwrapParenExpr(file, expr)
	if file == nil || expr == 0 || file.FlatType(expr) != "call_expression" || flatCallExpressionName(file, expr) != "format" {
		return false
	}
	_, args := flatCallExpressionParts(file, expr)
	index := 0
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		if index > 0 && observedIdentifierExpr(file, flatValueArgumentExpression(file, arg), identifiers) {
			return true
		}
		index++
	}
	return false
}

func observedIdentifierExpr(file *scanner.File, expr uint32, identifiers map[string]bool) bool {
	expr = flatUnwrapParenExpr(file, expr)
	if file == nil || expr == 0 || file.FlatType(expr) != "simple_identifier" {
		return false
	}
	return identifiers[normalizeObservabilityIdentifier(file.FlatNodeText(expr))]
}

// StructuredLogKeyMixedCaseRule detects files that mostly use one structured
// logging key convention but contain minority keys using the other convention.
type StructuredLogKeyMixedCaseRule struct {
	FlatDispatchBase
	BaseRule

	MinKeys          int
	ThresholdPercent int
}

func (r *StructuredLogKeyMixedCaseRule) Confidence() float64 { return api.ConfidenceMedium }

// structuredLogKeyDecision carries the per-file majority/minority verdict.
// Empty strings mean "no finding for this file".
type structuredLogKeyDecision struct {
	minority string
	majority string
}

// structuredLogKeyAtCall returns the static structured-log key written at
// `call` when it is an `addKeyValue(...)` or `MDC.put(...)` invocation whose
// first positional argument is a non-interpolated string literal. The
// receiver check rejects `someMap.put("k", v)` look-alikes.
func structuredLogKeyAtCall(file *scanner.File, call uint32) (string, bool) {
	switch flatCallExpressionName(file, call) {
	case "addKeyValue":
	case "put":
		if flatReceiverNameFromCall(file, call) != "MDC" {
			return "", false
		}
	default:
		return "", false
	}
	return mdcStaticKeyFlat(file, call)
}

func (r *StructuredLogKeyMixedCaseRule) decisionFor(ctx *api.Context) structuredLogKeyDecision {
	file := ctx.File
	minKeys := r.MinKeys
	if minKeys == 0 {
		minKeys = 3
	}
	threshold := r.ThresholdPercent
	if threshold == 0 {
		threshold = 70
	}
	return filefacts.FileFact(ctx.Facts, file, "structuredLogKeyDecision", func() structuredLogKeyDecision {
		snake, camel := 0, 0
		tree := file.FlatTree
		callTypeID, ok := scanner.LookupFlatNodeType("call_expression")
		if !ok || tree == nil {
			return structuredLogKeyDecision{}
		}
		for _, flatIdx := range tree.NodesOfType(callTypeID) {
			key, ok := structuredLogKeyAtCall(file, flatIdx)
			if !ok {
				continue
			}
			switch structuredLogKeyConvention(key) {
			case "snake_case":
				snake++
			case "camelCase":
				camel++
			}
		}
		total := snake + camel
		if total < minKeys || snake == 0 || camel == 0 {
			return structuredLogKeyDecision{}
		}
		switch {
		case snake*100 >= total*threshold:
			return structuredLogKeyDecision{minority: "camelCase", majority: "snake_case"}
		case camel*100 >= total*threshold:
			return structuredLogKeyDecision{minority: "snake_case", majority: "camelCase"}
		}
		return structuredLogKeyDecision{}
	})
}

func (r *StructuredLogKeyMixedCaseRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if file == nil || file.FlatTree == nil {
		return
	}
	key, ok := structuredLogKeyAtCall(file, idx)
	if !ok {
		return
	}
	convention := structuredLogKeyConvention(key)
	if convention == "" {
		return
	}
	decision := r.decisionFor(ctx)
	if decision.minority == "" || convention != decision.minority {
		return
	}
	ctx.EmitAt(file.FlatRow(idx)+1, 1,
		"Structured log key "+key+" uses "+decision.minority+" in a file that mostly uses "+decision.majority+". Keep structured log keys consistent within a file.")
}

// LoggerStringConcatRule detects SLF4J/Logback/log4j-style logger calls whose
// message argument is built with `+` string concatenation. Like string
// interpolation, this evaluates the concatenation eagerly even when the level
// is disabled. The fix is the same parameterized form
// (`logger.info("value={}", value)`).
type LoggerStringConcatRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Detection
// pattern-matches logger call shapes without confirming the receiver type.
func (r *LoggerStringConcatRule) Confidence() float64 { return api.ConfidenceMedium }

// loggerStringConcatMessageArgFlat returns the message argument of `call` when
// it is a positional or named `+` concatenation that includes a string literal
// operand. Pure numeric or non-string concatenation is ignored.
func loggerStringConcatMessageArgFlat(file *scanner.File, call uint32) uint32 {
	_, args := flatCallExpressionParts(file, call)
	if args == 0 {
		return 0
	}

	if arg := flatNamedValueArgument(file, args, "message"); arg != 0 {
		if argIsLoggerStringConcat(file, arg) {
			return arg
		}
		return 0
	}

	first := flatPositionalValueArgument(file, args, 0)
	if first == 0 {
		return 0
	}
	if argIsLoggerStringConcat(file, first) {
		return first
	}
	return 0
}

func argIsLoggerStringConcat(file *scanner.File, arg uint32) bool {
	expr := flatValueArgumentExpression(file, arg)
	if expr == 0 {
		return false
	}
	expr = flatUnwrapParenExpr(file, expr)
	if file.FlatType(expr) != "additive_expression" {
		return false
	}
	var operands []uint32
	if !collectStringConcatOperands(file, expr, &operands) {
		return false
	}
	if len(operands) < 2 {
		return false
	}
	for _, op := range operands {
		op = flatUnwrapParenExpr(file, op)
		switch file.FlatType(op) {
		case "string_literal", "line_string_literal", "multi_line_string_literal":
			return true
		}
	}
	return false
}

// MdcPutNoRemoveRule detects MDC.put("key", value) calls inside a function
// body where the same function does not contain a matching MDC.remove("key")
// or MDC.clear(). MDC values otherwise leak across requests when the thread
// is reused. The MDCCloseable form (MDC.putCloseable("key", value).use { ... })
// is naturally excluded because its call name is putCloseable, not put.
type MdcPutNoRemoveRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Observability rule. Detection pattern-matches logging/metrics API call
// shapes without confirming the receiver type. Classified per roadmap/17.
func (r *MdcPutNoRemoveRule) Confidence() float64 { return api.ConfidenceMedium }

// mdcStaticKeyFlat returns the literal value of `call`'s first positional
// argument when that argument is a non-interpolated string literal. Used by
// MDC.put / MDC.remove key tracking and by StructuredLogKeyMixedCaseRule's
// key-convention scan. The boolean is false when the argument is dynamic.
func mdcStaticKeyFlat(file *scanner.File, call uint32) (string, bool) {
	_, args := flatCallExpressionParts(file, call)
	if args == 0 {
		return "", false
	}
	keyArg := flatPositionalValueArgument(file, args, 0)
	if keyArg == 0 {
		return "", false
	}
	expr := flatValueArgumentExpression(file, keyArg)
	if expr == 0 || file.FlatType(expr) != "string_literal" {
		return "", false
	}
	if flatContainsStringInterpolation(file, expr) {
		return "", false
	}
	return stringLiteralContent(file, expr), true
}

// mdcRemoveOrClearMatchesFlat reports whether `fn`'s body contains an
// MDC.remove(key) for the given key, or an MDC.clear() call, within
// the same synchronous scope as the matching MDC.put. Calls that live
// inside a nested lambda, anonymous function, or local class/object —
// e.g. `launch { MDC.clear() }` — execute on a different thread or
// at a different time and do not actually clean up the put we paired
// against, so flatWalkLocalScope stops at those boundaries. When the
// key is not a static literal, any in-scope MDC.remove(...) call
// counts as a match.
func mdcRemoveOrClearMatchesFlat(file *scanner.File, fn uint32, key string, keyKnown bool) bool {
	if file == nil || fn == 0 {
		return false
	}
	matched := false
	flatWalkLocalScope(file, fn, func(candidate uint32) {
		if matched {
			return
		}
		if file.FlatType(candidate) != "call_expression" {
			return
		}
		if flatReceiverNameFromCall(file, candidate) != "MDC" {
			return
		}
		switch flatCallExpressionName(file, candidate) {
		case "clear":
			matched = true
			return
		case "remove":
			if !keyKnown {
				matched = true
				return
			}
			rmKey, ok := mdcStaticKeyFlat(file, candidate)
			if !ok || rmKey == key {
				matched = true
			}
		}
	})
	return matched
}

// MdcAcrossCoroutineBoundaryRule detects an `MDC.put(...)` followed by a
// coroutine builder (`launch`/`async`/`withContext`) in the same statement
// scope whose context argument does not include `MDCContext()`. MDC values do
// not propagate to a coroutine running on a different dispatcher unless the
// caller installs `MDCContext` in the coroutine context.
type MdcAcrossCoroutineBoundaryRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Observability rule. Detection pattern-matches logging/metrics API call
// shapes without confirming the receiver type. Classified per roadmap/17.
func (r *MdcAcrossCoroutineBoundaryRule) Confidence() float64 { return api.ConfidenceMedium }

// firstUnpropagatedCoroutineBuilderAfterFlat walks subsequent siblings of a
// statement-level call_expression and returns the first sibling call that
// looks like a coroutine builder without `MDCContext()` in its context arg.
// Walks only direct siblings — it does not descend into nested scopes.
func firstUnpropagatedCoroutineBuilderAfterFlat(file *scanner.File, idx uint32) (uint32, string) {
	if file == nil || idx == 0 {
		return 0, ""
	}
	for sib := file.FlatNextSib(idx); sib != 0; sib = file.FlatNextSib(sib) {
		if file.FlatType(sib) != "call_expression" {
			continue
		}
		name, args, lambda := coroutineBuilderPartsFlat(file, sib)
		if name != "launch" && name != "async" && name != "withContext" {
			continue
		}
		if lambda == 0 {
			continue
		}
		if coroutineContextHasMDCFlat(file, args) {
			continue
		}
		return sib, name
	}
	return 0, ""
}
