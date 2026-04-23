package rules

import (
	"strings"

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
func (r *LogLevelGuardMissingRule) Confidence() float64 { return 0.75 }

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
	"org.slf4j.",
	"ch.qos.logback.",
	"org.apache.logging.log4j.",
	"mu.",
	"io.github.oshai.kotlinlogging.",
}

// buildLoggerImportsFromAST walks import_header AST nodes and returns whether
// any known logger package is imported, plus a map of alias/simple-name to FQN
// for all known logger imports (used to resolve aliased logger receivers).
func buildLoggerImportsFromAST(file *scanner.File) (bool, map[string]string) {
	if file == nil {
		return false, nil
	}
	found := false
	aliases := make(map[string]string)
	file.FlatWalkNodes(0, "import_header", func(node uint32) {
		text := strings.TrimSpace(file.FlatNodeText(node))
		text = strings.TrimPrefix(text, "import ")
		text = strings.TrimSuffix(text, ";")
		text = strings.TrimSpace(text)

		fqn := text
		alias := ""
		if idx := strings.Index(text, " as "); idx >= 0 {
			fqn = strings.TrimSpace(text[:idx])
			alias = strings.TrimSpace(text[idx+4:])
		}

		for _, prefix := range knownLoggerPackagePrefixes {
			if strings.HasPrefix(fqn, prefix) {
				found = true
				key := alias
				if key == "" {
					parts := strings.Split(fqn, ".")
					if len(parts) > 0 {
						key = parts[len(parts)-1]
					}
				}
				if key != "" {
					aliases[key] = fqn
				}
				break
			}
		}
	})
	return found, aliases
}

func isLikelyLogReceiver(receiver string, aliases map[string]string) bool {
	if receiver == "" {
		return false
	}
	_, ok := aliases[receiver]
	return ok
}

func fileImportsKnownLoggerAPI(file *scanner.File) bool {
	found, _ := buildLoggerImportsFromAST(file)
	return found
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

func isKnownLoggerTypeText(text string) bool {
	text = compactConditionText(strings.TrimSuffix(text, "?"))
	switch text {
	case "org.slf4j.Logger",
		"ch.qos.logback.classic.Logger",
		"org.apache.logging.log4j.Logger",
		"mu.KLogger",
		"io.github.oshai.kotlinlogging.KLogger":
		return true
	default:
		return false
	}
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

func conditionTextRequiresGuard(text, receiver, guardProperty string) bool {
	text = normalizeConditionText(text)
	if text == "" {
		return false
	}
	if conditionTextMatchesGuard(text, receiver, guardProperty) {
		return true
	}

	disjunctions := splitTopLevelLogicalOr(text)
	if len(disjunctions) > 1 {
		for _, clause := range disjunctions {
			if !conditionTextRequiresGuard(clause, receiver, guardProperty) {
				return false
			}
		}
		return true
	}

	clauses := splitTopLevelLogicalAnd(text)
	if len(clauses) > 1 {
		for _, clause := range clauses {
			if conditionTextRequiresGuard(clause, receiver, guardProperty) {
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

func conditionTextPreventsGuard(text, receiver, guardProperty string) bool {
	text = normalizeConditionText(text)
	if text == "" {
		return false
	}
	if conditionTextMatchesNegatedGuard(text, receiver, guardProperty) {
		return true
	}

	conjunctions := splitTopLevelLogicalAnd(text)
	if len(conjunctions) > 1 {
		for _, clause := range conjunctions {
			if !conditionTextPreventsGuard(clause, receiver, guardProperty) {
				return false
			}
		}
		return true
	}

	clauses := splitTopLevelLogicalOr(text)
	if len(clauses) > 1 {
		for _, clause := range clauses {
			if conditionTextPreventsGuard(clause, receiver, guardProperty) {
				return true
			}
		}
	}

	return false
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

func conditionTextMatchesGuard(text, receiver, guardProperty string) bool {
	text = normalizeConditionText(text)
	for _, candidate := range logLevelGuardCandidates(receiver, guardProperty) {
		if conditionTextMatchesBooleanGuard(text, candidate) {
			return true
		}
	}
	return false
}

func conditionTextMatchesNegatedGuard(text, receiver, guardProperty string) bool {
	text = normalizeConditionText(text)
	if !strings.HasPrefix(text, "!") {
		for _, candidate := range logLevelGuardCandidates(receiver, guardProperty) {
			if conditionTextMatchesBooleanNegatedGuard(text, candidate) {
				return true
			}
		}
		return false
	}

	inner := normalizeConditionText(strings.TrimSpace(strings.TrimPrefix(text, "!")))
	return conditionTextMatchesGuard(inner, receiver, guardProperty)
}

func logLevelGuardCandidates(receiver, guardProperty string) []string {
	if receiver == "" || guardProperty == "" {
		return nil
	}

	return []string{
		receiver + "." + guardProperty,
		receiver + "." + guardProperty + "()",
		receiver + "?." + guardProperty,
		receiver + "?." + guardProperty + "()",
	}
}

func conditionTextMatchesBooleanGuard(text, candidate string) bool {
	text = compactConditionText(text)
	for _, form := range []string{
		candidate,
		candidate + "==true",
		candidate + "!=false",
		"true==" + candidate,
		"false!=" + candidate,
	} {
		if text == form || strings.HasSuffix(text, "."+form) {
			return true
		}
	}
	return false
}

func conditionTextMatchesBooleanNegatedGuard(text, candidate string) bool {
	text = compactConditionText(text)
	for _, form := range []string{
		candidate + "==false",
		candidate + "!=true",
		"false==" + candidate,
		"true!=" + candidate,
	} {
		if text == form || strings.HasSuffix(text, "."+form) {
			return true
		}
	}
	return false
}

func splitTopLevelLogicalAnd(text string) []string {
	var clauses []string
	depth := 0
	start := 0

	for i := 0; i < len(text)-1; i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case '&':
			if depth == 0 && text[i+1] == '&' {
				clauses = append(clauses, strings.TrimSpace(text[start:i]))
				start = i + 2
				i++
			}
		}
	}

	if len(clauses) == 0 {
		return []string{text}
	}

	clauses = append(clauses, strings.TrimSpace(text[start:]))
	return clauses
}

func splitTopLevelLogicalOr(text string) []string {
	var clauses []string
	depth := 0
	start := 0

	for i := 0; i < len(text)-1; i++ {
		switch text[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case '|':
			if depth == 0 && text[i+1] == '|' {
				clauses = append(clauses, strings.TrimSpace(text[start:i]))
				start = i + 2
				i++
			}
		}
	}

	if len(clauses) == 0 {
		return []string{text}
	}

	clauses = append(clauses, strings.TrimSpace(text[start:]))
	return clauses
}

func logLevelGuardProperty(level string) string {
	if level == "trace" {
		return "isTraceEnabled"
	}
	return "isDebugEnabled"
}

func normalizeConditionText(text string) string {
	text = strings.TrimSpace(text)
	for len(text) >= 2 && text[0] == '(' && text[len(text)-1] == ')' {
		text = strings.TrimSpace(text[1 : len(text)-1])
	}
	return text
}

func compactConditionText(text string) string {
	return strings.Join(strings.Fields(normalizeConditionText(text)), "")
}

// LogWithoutCorrelationIdRule detects logger calls inside coroutine builders
// whose context does not include MDCContext().
type LogWithoutCorrelationIdRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Observability rule. Detection pattern-matches logging/metrics API call
// shapes without confirming the receiver type. Classified per roadmap/17.
func (r *LogWithoutCorrelationIdRule) Confidence() float64 { return 0.75 }

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
func (r *LoggerWithoutLoggerFieldRule) Confidence() float64 { return 0.75 }
