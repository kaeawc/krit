package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func flatIsUsedAsExpression(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	parent, ok := file.FlatParent(idx)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "source_file":
		return false
	case "statements":
		return flatIsUsedAsExpressionInStatements(file, idx, parent)
	case "property_declaration", "variable_declaration", "value_argument", "value_arguments",
		"return_expression", "jump_expression", "assignment", "augmented_assignment",
		"binary_expression", "comparison_expression", "equality_expression",
		"additive_expression", "multiplicative_expression", "conjunction_expression",
		"disjunction_expression", "elvis_expression", "range_expression",
		"infix_expression", "check_expression", "as_expression", "if_expression",
		"when_expression", "call_expression":
		return true
	case "parenthesized_expression", "navigation_expression":
		return flatIsUsedAsExpression(file, parent)
	case "lambda_literal":
		for i := 0; i < file.FlatNamedChildCount(parent); i++ {
			child := file.FlatNamedChild(parent, i)
			if file.FlatType(child) == "statements" {
				return flatLastNamedChild(file, child) == idx
			}
		}
		return false
	case "control_structure_body":
		return flatIsUsedAsExpressionInControlBody(file, parent)
	case "when_entry":
		if gp, ok := file.FlatParent(parent); ok && file.FlatType(gp) == "when_expression" {
			return flatIsUsedAsExpression(file, gp)
		}
		return false
	default:
		return true
	}
}

func flatIsUsedAsExpressionInStatements(file *scanner.File, idx, parent uint32) bool {
	for prev, ok := file.FlatPrevSibling(idx); ok; prev, ok = file.FlatPrevSibling(prev) {
		if !file.FlatTree.NodeIsNamed(prev) {
			continue
		}
		if file.FlatType(prev) == "jump_expression" {
			return true
		}
	}
	gp, ok := file.FlatParent(parent)
	if !ok {
		return false
	}
	gpType := file.FlatType(gp)
	if gpType == "control_structure_body" || gpType == "lambda_literal" {
		if flatLastNamedChild(file, parent) == idx {
			return flatIsUsedAsExpression(file, gp)
		}
	}
	if gpType == "catch_block" || gpType == "finally_block" {
		if flatLastNamedChild(file, parent) == idx {
			if ggp, ok := file.FlatParent(gp); ok && file.FlatType(ggp) == "try_expression" {
				return flatIsUsedAsExpression(file, ggp)
			}
		}
	}
	if gpType == "try_expression" && flatLastNamedChild(file, parent) == idx {
		return flatIsUsedAsExpression(file, gp)
	}
	return false
}

func flatIsUsedAsExpressionInControlBody(file *scanner.File, parent uint32) bool {
	gp, ok := file.FlatParent(parent)
	if !ok {
		return false
	}
	gpType := file.FlatType(gp)
	if gpType == "if_expression" || gpType == "when_expression" {
		return flatIsUsedAsExpression(file, gp)
	}
	if gpType == "when_entry" {
		if ggp, ok := file.FlatParent(gp); ok && file.FlatType(ggp) == "when_expression" {
			return flatIsUsedAsExpression(file, ggp)
		}
	}
	return false
}

// hasCheckReturnAnnotation checks if any annotation in the list matches the
// built-in @CheckReturnValue/@CheckResult patterns or the user-configured patterns.
func hasCheckReturnAnnotation(annotations []string, configPatterns []string) bool {
	for _, ann := range annotations {
		// Check against built-in annotation set
		if checkReturnValueAnnotations[ann] {
			return true
		}
		// Also check the simple name (last segment after '.')
		if idx := strings.LastIndex(ann, "."); idx >= 0 {
			if checkReturnValueAnnotations[ann[idx+1:]] {
				return true
			}
		}
		// Check user-configured patterns (supports wildcards like "*.CheckResult")
		for _, pat := range configPatterns {
			if matchAnnotationPattern(pat, ann) {
				return true
			}
		}
	}
	return false
}

// hasIgnoreReturnAnnotation checks if any annotation overrides @CheckReturnValue
// (e.g., @CanIgnoreReturnValue).
func hasIgnoreReturnAnnotation(annotations []string, configPatterns []string) bool {
	for _, ann := range annotations {
		if canIgnoreReturnValueAnnotations[ann] {
			return true
		}
		if idx := strings.LastIndex(ann, "."); idx >= 0 {
			if canIgnoreReturnValueAnnotations[ann[idx+1:]] {
				return true
			}
		}
		for _, pat := range configPatterns {
			if matchAnnotationPattern(pat, ann) {
				return true
			}
		}
	}
	return false
}

func ignoredReturnValueAnnotationEvidence(annotations []string, checkPatterns, ignorePatterns []string) (check bool, ignored bool) {
	return hasCheckReturnAnnotation(annotations, checkPatterns), hasIgnoreReturnAnnotation(annotations, ignorePatterns)
}

func ignoredReturnValueOracleAnnotations(lookup oracleAnnotationLookup, filePath string, line, col int) []string {
	if lookup == nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	add := func(values []string) {
		for _, ann := range values {
			if ann == "" || seen[ann] {
				continue
			}
			seen[ann] = true
			out = append(out, ann)
		}
	}
	add(lookup.LookupCallTargetAnnotations(filePath, line, col))
	return out
}

func ignoredReturnValueMergedOracleAnnotations(lookup oracle.Lookup, file *scanner.File, idx uint32, line, col int) []string {
	if lookup == nil || file == nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	add := func(values []string) {
		for _, ann := range values {
			if ann == "" || seen[ann] {
				continue
			}
			seen[ann] = true
			out = append(out, ann)
		}
	}
	add(oracleLookupCallTargetAnnotationsFlat(lookup, file, idx))
	add(ignoredReturnValueOracleAnnotations(lookup, file.Path, line, col))
	return out
}

type oracleAnnotationLookup interface {
	LookupCallTargetAnnotations(filePath string, line, col int) []string
}

// matchAnnotationPattern matches an annotation FQN against a pattern.
// Patterns: exact match, "*.Name" matches any FQN ending with ".Name", "*Name"
// matches a suffix.
func matchAnnotationPattern(pattern, annotation string) bool {
	if pattern == annotation {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".Name"
		return strings.HasSuffix(annotation, suffix)
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(annotation, pattern[1:])
	}
	return false
}

// matchesReturnValueType checks if a resolved FQN matches the configured
// returnValueTypes patterns (e.g., "kotlinx.coroutines.flow.*Flow").
func matchesReturnValueType(fqn string, patterns []string) bool {
	fqn = strings.TrimSpace(fqn)
	if fqn == "" {
		return false
	}
	if idx := strings.IndexByte(fqn, '<'); idx >= 0 {
		fqn = fqn[:idx]
	}
	if returnValueFQNs[fqn] {
		return true
	}
	for _, pat := range patterns {
		pat = strings.TrimSpace(pat)
		if pat == fqn {
			return true
		}
		// "pkg.*Suffix" — prefix + suffix glob
		if idx := strings.Index(pat, "*"); idx >= 0 {
			prefix := pat[:idx]
			suffix := pat[idx+1:]
			if strings.HasPrefix(fqn, prefix) && strings.HasSuffix(fqn, suffix) {
				return true
			}
		}
	}
	return false
}

func ignoredReturnValueTypeMatches(rt *typeinfer.ResolvedType, configured []string, restrictToConfig bool) bool {
	if !ignoredReturnValueTypeKnown(rt) ||
		ignoredReturnValueTypeIsUnitOrNothing(rt) {
		return false
	}
	if rt.Kind == typeinfer.TypeFunction ||
		strings.HasPrefix(rt.FQN, "kotlin.Function") ||
		strings.HasPrefix(rt.Name, "Function") {
		return !restrictToConfig || matchesReturnValueType("kotlin.Function", configured)
	}
	patterns := configured
	if !restrictToConfig {
		patterns = append(append([]string{}, defaultReturnValueTypePatterns...), configured...)
	}
	if matchesReturnValueType(rt.FQN, patterns) {
		return true
	}
	if rt.FQN == "" && rt.Name != "" {
		return matchesReturnValueType(rt.Name, patterns)
	}
	return false
}

func ignoredReturnValueTypeKnown(rt *typeinfer.ResolvedType) bool {
	if rt == nil || rt.Kind == typeinfer.TypeUnknown {
		return false
	}
	return rt.FQN != "" || rt.Name != "" || rt.Kind == typeinfer.TypeFunction ||
		rt.Kind == typeinfer.TypeUnit || rt.Kind == typeinfer.TypeNothing
}

func ignoredReturnValueTypeIsUnitOrNothing(rt *typeinfer.ResolvedType) bool {
	if rt == nil {
		return false
	}
	return rt.Kind == typeinfer.TypeUnit || rt.Kind == typeinfer.TypeNothing ||
		rt.FQN == "kotlin.Unit" || rt.FQN == "kotlin.Nothing" ||
		rt.Name == "Unit" || rt.Name == "Nothing"
}

func ignoredReturnValueTypeName(rt *typeinfer.ResolvedType) string {
	if rt == nil {
		return "a must-use type"
	}
	if rt.FQN != "" {
		return rt.FQN
	}
	if rt.Name != "" {
		return rt.Name
	}
	return "a must-use type"
}

var ignoredReturnValueCollectionSourceCalls = map[string]bool{
	"arrayListOf":   true,
	"asFlow":        true,
	"asSequence":    true,
	"emptyList":     true,
	"emptyMap":      true,
	"emptySet":      true,
	"flow":          true,
	"flowOf":        true,
	"hashMapOf":     true,
	"hashSetOf":     true,
	"linkedMapOf":   true,
	"linkedSetOf":   true,
	"listOf":        true,
	"mapOf":         true,
	"mutableListOf": true,
	"mutableMapOf":  true,
	"mutableSetOf":  true,
	"sequence":      true,
	"sequenceOf":    true,
	"setOf":         true,
	"stream":        true,
	"streamOf":      true,
	"toList":        true,
	"toMap":         true,
	"toSet":         true,
}

var ignoredReturnValueReceiverTypeHints = []string{
	"Array",
	"Collection",
	"Flow",
	"Iterable",
	"List",
	"Map",
	"Sequence",
	"Set",
	"Stream",
}

func ignoredReturnValueFunctionalFallbackHasReceiverEvidence(file *scanner.File, call uint32) bool {
	if file == nil || call == 0 || file.FlatType(call) != "call_expression" {
		return false
	}
	navExpr, _ := flatCallExpressionParts(file, call)
	if navExpr == 0 || file.FlatType(navExpr) != "navigation_expression" || file.FlatNamedChildCount(navExpr) == 0 {
		return false
	}
	return ignoredReturnValueReceiverLooksFunctionalSource(file, flatUnwrapParenExpr(file, file.FlatNamedChild(navExpr, 0)))
}

func ignoredReturnValueReceiverLooksFunctionalSource(file *scanner.File, receiver uint32) bool {
	if file == nil || receiver == 0 {
		return false
	}
	receiver = flatUnwrapParenExpr(file, receiver)
	switch file.FlatType(receiver) {
	case "simple_identifier":
		return ignoredReturnValueIdentifierHasFunctionalType(file, file.FlatNodeText(receiver))
	case "navigation_expression":
		return ignoredReturnValueNavigationLooksFunctionalSource(file, receiver)
	case "call_expression":
		return ignoredReturnValueCallLooksFunctionalSource(file, receiver)
	default:
		return false
	}
}

func ignoredReturnValueNavigationLooksFunctionalSource(file *scanner.File, nav uint32) bool {
	if file == nil || nav == 0 || file.FlatType(nav) != "navigation_expression" {
		return false
	}
	if ignoredReturnValueIdentifierHasFunctionalType(file, flatNavigationExpressionLastIdentifier(file, nav)) {
		return true
	}
	if file.FlatNamedChildCount(nav) == 0 {
		return false
	}
	return ignoredReturnValueReceiverLooksFunctionalSource(file, file.FlatNamedChild(nav, 0))
}

func ignoredReturnValueCallLooksFunctionalSource(file *scanner.File, call uint32) bool {
	name := flatCallExpressionName(file, call)
	if name == "" {
		return false
	}
	if ignoredReturnValueCollectionSourceCalls[name] ||
		ignoredReturnValueFunctionReturnsFunctionalType(file, name) {
		return true
	}
	if !functionalOps[name] {
		return false
	}
	return ignoredReturnValueFunctionalFallbackHasReceiverEvidence(file, call)
}

func ignoredReturnValueIdentifierHasFunctionalType(file *scanner.File, name string) bool {
	if file == nil || name == "" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if found {
			return
		}
		switch file.FlatType(idx) {
		case "function_value_parameter", "parameter", "variable_declaration", "property_declaration":
			declName, typeText := ignoredReturnValueDeclarationNameAndType(file, idx)
			found = declName == name && (ignoredReturnValueTypeTextLooksFunctional(typeText) ||
				ignoredReturnValueDeclarationInitializerLooksFunctional(file, idx))
		}
	})
	return found
}

func ignoredReturnValueFunctionReturnsFunctionalType(file *scanner.File, name string) bool {
	if file == nil || name == "" {
		return false
	}
	found := false
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if found || file.FlatType(idx) != "function_declaration" || flatFunctionName(file, idx) != name {
			return
		}
		found = ignoredReturnValueTypeTextLooksFunctional(ignoredReturnValueFunctionReturnTypeText(file, idx))
	})
	return found
}

func ignoredReturnValueDeclarationNameAndType(file *scanner.File, idx uint32) (string, string) {
	if file == nil || idx == 0 {
		return "", ""
	}
	return extractIdentifierFlat(file, idx), explicitTypeTextFlat(file, idx)
}

func ignoredReturnValueFunctionReturnTypeText(file *scanner.File, fn uint32) string {
	if file == nil || fn == 0 || file.FlatType(fn) != "function_declaration" {
		return ""
	}
	return explicitTypeTextFlat(file, fn)
}

func ignoredReturnValueDeclarationInitializerLooksFunctional(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		if found || candidate == idx || file.FlatType(candidate) != "call_expression" {
			return
		}
		found = ignoredReturnValueCallLooksFunctionalSource(file, candidate)
	})
	return found
}

func ignoredReturnValueTypeTextLooksFunctional(typeText string) bool {
	typeText = strings.TrimSpace(typeText)
	if typeText == "" {
		return false
	}
	for _, hint := range ignoredReturnValueReceiverTypeHints {
		if typeText == hint ||
			strings.HasPrefix(typeText, hint+"<") ||
			strings.HasPrefix(typeText, hint+"?") ||
			strings.HasSuffix(typeText, "."+hint) ||
			strings.Contains(typeText, "."+hint+"<") {
			return true
		}
	}
	return false
}

func stringListContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
